package application

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
)

const httpTimeout = 5 * time.Minute

type FireAction interface {
	Fire(common.Application, *common.ApplicationEvent, interface{})
}

type PreFireAction interface {
	FireAction
	PreFire(common.Application, *common.ApplicationEvent)
}

type PostFireAction interface {
	FireAction
	PostFire(common.Application, *common.ApplicationEvent)
}

var (
	actions = map[common.ApplicationEventKind]FireAction{
		common.InitApplicationEventKind:        InitFireAction((*Abstract).FireInit),
		common.RunApplicationEventKind:         RunFireAction((*Abstract).FireRun),
		common.FinishApplicationEventKind:      FinishFireAction((*Abstract).FireFinish),
		common.ReconfigureApplicationEventKind: ReconfigureFireAction((*Abstract).FireFinish),
	}
)

// Abstract базовое приложение
type Abstract struct {
	// config meta data
	configMeta ConfigMeta

	// сервисы приложения, отправляющие письма
	services []interface{}

	// канал событий приложения
	events       chan *common.ApplicationEvent
	eventsClosed bool

	// флаг, сигнализирующий окончание работы приложения
	done       chan bool
	doneClosed bool

	CommonTimeout common.Timeout `yaml:"timeouts"`
}

type ConfigMeta struct {
	// local config path
	configFilename string

	// remote config addr
	configRemoteAddr string

	// config update duration
	configUpdateDuration time.Duration

	configUpdateTimer *time.Ticker

	// config data
	configData []byte

	// warning error
	configWarning error

	// error config initialisation
	configError error

	client *http.Client

	rwm sync.RWMutex
}

// IsValidConfigFilename проверяет валидность пути к файлу с настройками
func (a *Abstract) IsValidConfigFilename(filename string) bool {
	return len(filename) > 0 && filename != common.ExampleConfigYaml
}

// запускает основной цикл приложения
func (a *Abstract) run(app common.Application, event *common.ApplicationEvent) {
	// создаем каналы для событий
	app.InitChannels(3)
	defer app.CloseEvents()

	app.OnEvent(func(ev *common.ApplicationEvent) {
		action := actions[ev.Kind]

		if preAction, ok := action.(PreFireAction); ok {
			preAction.PreFire(app, ev)
		}

		for _, service := range app.Services() {
			action.Fire(app, ev, service)
		}

		if postAction, ok := action.(PostFireAction); ok {
			postAction.PostFire(app, ev)
		}
	})

	app.InitConfig()

	app.SendEvents(event)
	<-app.Done()
}

func (a *Abstract) InitConfig() {
	a.collectConfigData()

	if a.configMeta.configUpdateTimer != nil {
		go func() {
			for range a.configMeta.configUpdateTimer.C {
				a.collectConfigData()
				a.SendEvents(common.NewApplicationEvent(common.ReconfigureApplicationEventKind))
			}
		}()
	}
}

func (a *Abstract) collectConfigData() {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout*2)
	defer cancel()

	a.configMeta.configData = nil

	if err := a.getRemoteConfigData(ctx); err != nil {
		a.configMeta.configWarning = err
	}

	if a.configMeta.configData == nil {
		if err := a.getLocalConfigData(ctx); err != nil {
			if a.configMeta.configWarning != nil {
				err = fmt.Errorf("remote err %s; loal error %w", a.configMeta.configWarning, err)
			}
			a.configMeta.configWarning = err
		}
	}

	if a.configMeta.configData == nil {
		a.configMeta.configError = fmt.Errorf("config file is empty")
	}
}

func (a *Abstract) getRemoteConfigData(ctx context.Context) error {
	if a.configMeta.client != nil && a.configMeta.configRemoteAddr != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", a.configMeta.configRemoteAddr, nil)
		if err != nil {
			return fmt.Errorf("create http request error %w", err)
		}

		resp, err := a.configMeta.client.Do(req)
		if err != nil {
			return fmt.Errorf("http client request error %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		cfg, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("can't read config body %w", err)
		}
		if cfg != nil && len(cfg) > 0 {
			a.configMeta.rwm.Lock()
			defer a.configMeta.rwm.Unlock()
			a.configMeta.configData = cfg
			return nil
		}
	}

	return nil
}

func (a *Abstract) getLocalConfigData(_ context.Context) error {
	cfg, err := ioutil.ReadFile(a.configMeta.configFilename)
	if err != nil {
		return fmt.Errorf("can't read file %s: %w", a.configMeta.configFilename, err)
	}

	if cfg != nil && len(cfg) > 0 {
		a.configMeta.rwm.Lock()
		defer a.configMeta.rwm.Unlock()
		a.configMeta.configData = cfg
		return nil
	}

	return nil
}

func (a *Abstract) SetConfigMeta(configFilename, configRemoteAddr, configUpdateDuration string) {
	a.configMeta = ConfigMeta{configFilename: configFilename}

	if configRemoteAddr != "" {
		a.configMeta.configRemoteAddr = configRemoteAddr
		a.configMeta.client = &http.Client{Transport: http.DefaultTransport, Timeout: httpTimeout}
	}

	if configUpdateDuration != "" {
		var err error
		a.configMeta.configUpdateDuration, err = time.ParseDuration(configUpdateDuration)
		if err == nil && a.configMeta.configUpdateDuration > 0 {
			a.configMeta.configUpdateTimer = time.NewTicker(a.configMeta.configUpdateDuration)
		}
	}
}

func (a *Abstract) GetConfigData() ([]byte, error, error) {
	a.configMeta.rwm.RLock()
	defer a.configMeta.rwm.RUnlock()

	return a.configMeta.configData, a.configMeta.configWarning, a.configMeta.configError
}

// SetEvents устанавливает канал событий приложения
func (a *Abstract) SetEvents(events chan *common.ApplicationEvent) {
	a.events = events
}

// InitChannels init channels
func (a *Abstract) InitChannels(cBufSize int) {
	a.events = make(chan *common.ApplicationEvent, cBufSize)
	a.done = make(chan bool)
}

func (a *Abstract) OnEvent(f func(ev *common.ApplicationEvent)) {
	go func() {
		for ev := range a.events {
			go f(ev)
		}
	}()
}

// CloseEvents close events channel
func (a *Abstract) CloseEvents() {
	if !a.eventsClosed {
		a.eventsClosed = true
		close(a.events)
	}
}

func (a *Abstract) SendEvents(ev *common.ApplicationEvent) bool {
	if a.eventsClosed {
		return false
	}

	a.events <- ev
	return true
}

// Done возвращает канал завершения приложения
func (a *Abstract) Done() <-chan bool {
	return a.done
}

func (a *Abstract) Close() {
	if !a.doneClosed {
		a.doneClosed = true
		close(a.done)
		a.configMeta.configUpdateTimer.Stop()
	}
}

// Services возвращает сервисы, используемые приложением
func (a *Abstract) Services() []interface{} {
	return a.services
}

// FireInit инициализирует сервисы
func (a *Abstract) FireInit(event *common.ApplicationEvent, abstractService interface{}) {
	service := abstractService.(common.Service)
	service.OnInit(event)
}

// Init инициализирует приложение
func (a *Abstract) Init(_ *common.ApplicationEvent, _ bool) {}

// Run запускает приложение
func (a *Abstract) Run() {}

// RunWithArgs запускает приложение с аргументами
func (a *Abstract) RunWithArgs(args ...interface{}) {}

// FireRun запускает сервисы приложения
func (a *Abstract) FireRun(event *common.ApplicationEvent, abstractService interface{}) {}

// FireFinish останавливает сервисы приложения
func (a *Abstract) FireFinish(event *common.ApplicationEvent, abstractService interface{}) {}

// Timeout возвращает таймауты приложения
func (a *Abstract) Timeout() common.Timeout {
	return a.CommonTimeout
}

type InitFireAction func(*Abstract, *common.ApplicationEvent, interface{})

func (i InitFireAction) Fire(app common.Application, event *common.ApplicationEvent, abstractService interface{}) {
	app.FireInit(event, abstractService)
}

func (i InitFireAction) PreFire(app common.Application, event *common.ApplicationEvent) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		app.SendEvents(common.NewApplicationEvent(common.FinishApplicationEventKind))
	}()

	bytes, warn, err := app.GetConfigData()
	if warn != nil {
		logger.All().WarnWithErr(err, "application configuration read warning")
	}

	if err != nil {
		logger.All().FailExitWithErr(err, "application can't read configuration file")
		return
	}

	event.Data = bytes
	app.Init(event, false)
}

func (i InitFireAction) PostFire(app common.Application, event *common.ApplicationEvent) {
	event.Kind = common.RunApplicationEventKind
	app.SendEvents(event)
}

type RunFireAction func(*Abstract, *common.ApplicationEvent, interface{})

func (r RunFireAction) Fire(app common.Application, event *common.ApplicationEvent, abstractService interface{}) {
	app.FireRun(event, abstractService)
}

type FinishFireAction func(*Abstract, *common.ApplicationEvent, interface{})

func (f FinishFireAction) Fire(app common.Application, event *common.ApplicationEvent, abstractService interface{}) {
	app.FireFinish(event, abstractService)
}

func (f FinishFireAction) PostFire(app common.Application, event *common.ApplicationEvent) {
	time.Sleep(2 * time.Second)
	app.Close()
}

type ReconfigureFireAction func(*Abstract, *common.ApplicationEvent, interface{})

func (r ReconfigureFireAction) Fire(app common.Application, event *common.ApplicationEvent, abstractService interface{}) {
	app.FireFinish(event, abstractService)
}

func (r ReconfigureFireAction) PostFire(app common.Application, event *common.ApplicationEvent) {
	time.Sleep(2 * time.Second)
	event.Kind = common.InitApplicationEventKind
	app.SendEvents(event)
}
