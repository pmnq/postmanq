package mailer

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"

	"gopkg.in/yaml.v3"

	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
)

const defaultDkimSelector = "mail"

// сервис отправки писем
type Service struct {
	// количество отправителей
	MailersCount int `yaml:"workers"`

	Configs map[string]*Config `yaml:"postmans"`

	events       chan *common.SendEvent
	eventsClosed bool
}

// создает новый сервис отправки писем
func Inst() common.SendingService {
	return new(Service)
}

// инициализирует сервис отправки писем
func (s *Service) OnInit(event *common.ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, s)
	if err != nil {
		logger.All().ErrErr(err)
	}

	if len(s.Configs) == 0 {
		logger.All().FailExit("mailer config is empty")
		return
	}

	s.events = make(chan *common.SendEvent)
	s.eventsClosed = false

	for name, config := range s.Configs {
		s.init(config, name)
	}

	if s.MailersCount == 0 {
		s.MailersCount = common.DefaultWorkersCount
	}
}

func (s *Service) init(conf *Config, hostname string) {
	// закрытый ключ должен быть указан обязательно
	// поэтому даже не проверяем что указано в переменной
	privateKey, err := ioutil.ReadFile(conf.PrivateKeyFilename)
	if err != nil {
		logger.By(hostname).ErrWithErr(err, "mailer service can't read private key %s", conf.PrivateKeyFilename)
		return
	}

	logger.By(hostname).Debug("mailer service private key %s read success", conf.PrivateKeyFilename)
	der, _ := pem.Decode(privateKey)
	conf.privateKey, err = x509.ParsePKCS1PrivateKey(der.Bytes)
	if err != nil {
		logger.By(hostname).ErrWithErr(err, "mailer service can't decode or parse private key %s", conf.PrivateKeyFilename)
		return
	}
}

// запускает отправителей и прием сообщений из очереди
func (s *Service) OnRun() {
	logger.All().Debug("run mailers apps...")
	for i := 0; i < s.MailersCount; i++ {
		go newMailer(i+1, s)
	}
}

// Event send event
func (s *Service) Event(ev *common.SendEvent) bool {
	if s.eventsClosed {
		return false
	}

	s.events <- ev
	return true
}

// завершает работу сервиса отправки писем
func (s *Service) OnFinish() {
	if !s.eventsClosed {
		s.eventsClosed = true
		close(s.events)
	}
}

func (s *Service) getDkimSelector(hostname string) string {
	if conf, ok := s.Configs[hostname]; ok {
		return conf.DkimSelector
	} else {
		logger.By(hostname).Warn("mailer service can't find dkim selector by %s. Set default %s", hostname, defaultDkimSelector)
		return defaultDkimSelector
	}
}

func (s *Service) getPrivateKey(hostname string) *rsa.PrivateKey {
	if conf, ok := s.Configs[hostname]; ok {
		return conf.privateKey
	} else {
		logger.By(hostname).Err("mailer service can't find private key by %s", hostname)
		return nil
	}
}

type Config struct {
	// путь до закрытого ключа
	PrivateKeyFilename string `yaml:"privateKey"`

	// селектор
	DkimSelector string `yaml:"dkimSelector"`

	// содержимое приватного ключа
	privateKey *rsa.PrivateKey
}
