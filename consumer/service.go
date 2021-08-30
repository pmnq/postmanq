package consumer

import (
	"fmt"
	"net/url"
	"sync"

	"github.com/streadway/amqp"
	"gopkg.in/yaml.v3"

	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
)

type amqpConnector struct {
	connect *amqp.Connection
	rwm     sync.RWMutex
}

func (ac *amqpConnector) SetConnect(connect *amqp.Connection) {
	ac.rwm.Lock()
	defer ac.rwm.Unlock()
	ac.connect = connect
}

func (ac *amqpConnector) GetConnect() *amqp.Connection {
	ac.rwm.RLock()
	defer ac.rwm.RUnlock()
	return ac.connect
}

// сервис получения сообщений
type Service struct {
	// настройка получателей сообщений
	Configs []*Config `yaml:"consumers"`

	// подключения к очередям
	connections map[string]*amqpConnector

	// получатели сообщений из очереди
	consumers map[string][]*Consumer

	assistants map[string][]*Assistant

	finish bool
}

// создает новый сервис получения сообщений
func Inst() common.SendingService {
	return &Service{
		connections: make(map[string]*amqpConnector),
		consumers:   make(map[string][]*Consumer),
		assistants:  make(map[string][]*Assistant),
	}
}

// OnInit инициализирует сервис
func (s *Service) OnInit(event *common.ApplicationEvent) {
	logger.All().Debug("init consumer service")
	// получаем настройки
	err := yaml.Unmarshal(event.Data, s)
	if err != nil {
		logger.All().ErrWithErr(err, "consumer service can't unmarshal config")
	}

	if len(s.Configs) == 0 {
		logger.All().FailExit("consumer config is empty")
		return
	}

	consumersCount := 0
	assistantsCount := 0
	for _, config := range s.Configs {
		connect, err := amqp.Dial(config.URI)
		if err != nil {
			logger.All().ErrWithErr(err, "consumer service can't connect to %s", config.URI)
			continue
		}

		connector := new(amqpConnector)
		connector.SetConnect(connect)

		channel, err := connector.GetConnect().Channel()
		if err != nil {
			logger.All().ErrWithErr(err, "consumer service can't get channel to %s", config.URI)
			continue
		}

		consumers := make([]*Consumer, len(config.Bindings))
		for i, binding := range config.Bindings {
			binding.init()
			// объявляем очередь
			binding.declare(channel)

			binding.delayedBindings = make(map[common.DelayedBindingType]*Binding)
			// объявляем отложенные очереди
			for delayedBindingType, delayedBinding := range delayedBindings {
				delayedBinding.declareDelayed(binding, channel)
				binding.delayedBindings[delayedBindingType] = delayedBinding
			}

			binding.failureBindings = make(map[FailureBindingType]*Binding)
			for failureBindingType, tplName := range failureBindingTypeTplNames {
				failureBinding := new(Binding)
				failureBinding.Exchange = fmt.Sprintf(tplName, binding.Exchange)
				failureBinding.Queue = fmt.Sprintf(tplName, binding.Queue)
				failureBinding.Type = binding.Type
				failureBinding.declare(channel)
				binding.failureBindings[failureBindingType] = failureBinding
			}

			consumersCount++
			consumers[i] = NewConsumer(consumersCount, connector, binding)
		}

		assistants := make([]*Assistant, len(config.Assistants))
		for i, assistantBinding := range config.Assistants {
			assistantBinding.Binding.init()
			// объявляем очередь
			assistantBinding.Binding.declare(channel)

			destBindings := make(map[string]*Binding)
			for domain, exchange := range assistantBinding.Dest {
				for _, consumer := range consumers {
					if consumer.binding.Exchange == exchange {
						destBindings[domain] = consumer.binding
						break
					}
				}
			}

			assistantsCount++
			assistants[i] = &Assistant{
				id:           assistantsCount,
				connector:    connector,
				srcBinding:   assistantBinding,
				destBindings: destBindings,
			}
		}

		s.connections[config.URI] = connector
		s.consumers[config.URI] = consumers
		s.assistants[config.URI] = assistants
		// слушаем закрытие соединения
		s.reconnect(connector, config)
	}
}

// объявляет слушателя закрытия соединения
func (s *Service) reconnect(connector *amqpConnector, config *Config) {
	closeErrors := connector.GetConnect().NotifyClose(make(chan *amqp.Error))
	go s.notifyCloseError(config, closeErrors)
}

// слушает закрытие соединения
func (s *Service) notifyCloseError(config *Config, closeErrors chan *amqp.Error) {
	for closeError := range closeErrors {
		if s.finish {
			return
		}

		logger.All().Warn("consumer service close connection %s with error - %v, restart...", config.URI, closeError)
		connect, err := amqp.Dial(config.URI)
		if err == nil {
			connector, ok := s.connections[config.URI]
			if !ok {
				s.connections[config.URI] = new(amqpConnector)
			}
			connector.SetConnect(connect)

			s.reconnect(connector, config)
			logger.All().Debug("consumer service reconnect to amqp server %s", config.URI)
		} else {
			logger.All().WarnWithErr(err, "consumer service can't reconnect to amqp server %s", config.URI)
		}
	}
}

// запускает сервис
func (s *Service) OnRun() {
	logger.All().Debug("run consumers...")
	for _, consumers := range s.consumers {
		s.runConsumers(consumers)
	}
	for _, assistants := range s.assistants {
		s.runAssistants(assistants)
	}
}

// запускает получателей
func (s *Service) runConsumers(consumers []*Consumer) {
	for _, consumer := range consumers {
		go consumer.run()
	}
}

func (s *Service) runAssistants(assistants []*Assistant) {
	for _, assistant := range assistants {
		go assistant.run()
	}
}

// останавливает получателей
func (s *Service) OnFinish() {
	logger.All().Debug("stop consumers...")
	s.finish = true
	for _, connector := range s.connections {
		if connector != nil && connector.GetConnect() != nil {
			err := connector.GetConnect().Close()
			if err != nil {
				logger.All().WarnErr(err)
			}
		}
	}

	s.connections = make(map[string]*amqpConnector)
	s.consumers = make(map[string][]*Consumer)
	s.assistants = make(map[string][]*Assistant)
}

// Event send event
func (s *Service) Event(_ *common.SendEvent) bool {
	return true
}

// запускает получение сообщений с ошибками и пересылает их другому сервису
func (s *Service) OnShowReport() {
	waiter := newWaiter()
	group := new(sync.WaitGroup)

	var delta int
	for _, apps := range s.consumers {
		for _, app := range apps {
			delta += app.binding.Handlers
		}
	}
	group.Add(delta)
	for _, apps := range s.consumers {
		go func(apps []*Consumer) {
			for _, app := range apps {
				for i := 0; i < app.binding.Handlers; i++ {
					go app.consumeFailureMessages(group)
				}
			}
		}(apps)
	}
	group.Wait()
	waiter.Stop()

	sendEvent := common.NewSendEvent(nil)
	sendEvent.Iterator.Next().(common.ReportService).Event(sendEvent)
}

// перекладывает сообщения из очереди в очередь
func (s *Service) OnPublish(event *common.ApplicationEvent) {
	group := new(sync.WaitGroup)
	delta := 0
	for uri, apps := range s.consumers {
		var necessaryPublish bool
		if len(event.GetStringArg("host")) > 0 {
			parsedUri, err := url.Parse(uri)
			if err == nil && parsedUri.Host == event.GetStringArg("host") {
				necessaryPublish = true
			} else {
				necessaryPublish = false
			}
		} else {
			necessaryPublish = true
		}
		if necessaryPublish {
			for _, app := range apps {
				delta += app.binding.Handlers
				for i := 0; i < app.binding.Handlers; i++ {
					go app.consumeAndPublishMessages(event, group)
				}
			}
		}
	}
	group.Add(delta)
	group.Wait()
	fmt.Println("done")
	common.App.SendEvents(common.NewApplicationEvent(common.FinishApplicationEventKind))
}

// получатель сообщений из очереди
type Config struct {
	URI        string              `yaml:"uri"`
	Assistants []*AssistantBinding `yaml:"assistants"`
	Bindings   []*Binding          `yaml:"bindings"`
}
