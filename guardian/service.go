package guardian

import (
	"gopkg.in/yaml.v3"

	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
)

// Service сервис блокирующий отправку писем
type Service struct {
	// GuardiansCount количество горутин блокирующий отправку писем к почтовым сервисам
	GuardiansCount int `yaml:"workers"`

	Configs map[string]*Config `yaml:"postmans"`

	events       chan *common.SendEvent
	eventsClosed bool
}

// Inst создает новый сервис блокировок
func Inst() common.SendingService {
	return new(Service)
}

// OnInit инициализирует сервис блокировок
func (s *Service) OnInit(event *common.ApplicationEvent) {
	logger.All().Debug("init guardians...")

	err := yaml.Unmarshal(event.Data, s)
	if err != nil {
		logger.All().FailExitErr(err)
		return
	}

	s.events = make(chan *common.SendEvent)
	s.eventsClosed = false

	if s.GuardiansCount == 0 {
		s.GuardiansCount = common.DefaultWorkersCount
	}
}

// OnRun запускает горутины
func (s *Service) OnRun() {
	for i := 0; i < s.GuardiansCount; i++ {
		go newGuardian(i+1, s)
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

// OnFinish завершает работу сервиса соединений
func (s *Service) OnFinish() {
	if !s.eventsClosed {
		s.eventsClosed = true
		close(s.events)
	}
}

func (s Service) getExcludes(hostname string) []string {
	if conf, ok := s.Configs[hostname]; ok {
		return conf.Excludes
	}
	return common.EmptyStrSlice
}

type Config struct {
	// Excludes хосты, на которую блокируется отправка писем
	Excludes []string `yaml:"exclude"`
}
