package logger

import (
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"

	"github.com/Halfi/postmanq/common"
)

type Config struct {
	// название уровня логирования, устанавливается в конфиге
	LevelName string `yaml:"logLevel"`

	// название вывода логов
	Output string `yaml:"logOutput"`

	file *os.File
}

type Service struct {
	Config

	Configs map[string]*Config `yaml:"postmans"`
}

func Inst() common.SendingService {
	return &Service{Config: Config{
		LevelName: "debug",
		Output:    "stdout",
	}}
}

// инициализирует сервис логирования
func (s *Service) OnInit(event *common.ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, s)
	if err != nil {
		All().ErrWithErr(err, "can't unmarshal logger config")
	}

	if len(s.Configs) == 0 {
		All().FailExit("logger config is empty")
		return
	}

	loggers = make(map[string]zerolog.Logger, len(s.Configs)+1)

	zerolog.SetGlobalLevel(getZerologLevel(s.LevelName))
	zerolog.TimestampFieldName = "t"
	zerolog.LevelFieldName = "l"
	zerolog.MessageFieldName = "m"
	zerolog.CallerFieldName = "c"

	// default CallerMarshalFunc adds full path
	// callerMarshalFunc adds only last 2 parts
	zerolog.CallerMarshalFunc = callerMarshalFunc

	s.file = getFileByOutput(s.Output)
	loggers[common.AllDomains] = zerolog.New(s.file).With().Timestamp().Caller().Logger()
	log.Logger = loggers[common.AllDomains]

	for domain, cfg := range s.Configs {
		cfg.file = getFileByOutput(cfg.Output)
		loggers[domain] = zerolog.New(cfg.file).Level(getZerologLevel(cfg.LevelName)).With().Timestamp().Caller().Logger()
	}

	All().Debug("logger initialisation success")
}

// ничего не делает, авторы логов уже пишут
func (s *Service) OnRun() {}

// Event send event
func (s *Service) Event(_ *common.SendEvent) bool {
	return true
}

// закрывает канал логирования
func (s *Service) OnFinish() {
	if s.file.Fd() > 2 {
		_ = s.file.Close()
	}

	for _, cfg := range s.Configs {
		if cfg.file.Fd() > 2 {
			_ = cfg.file.Close()
		}
	}
}

func callerMarshalFunc(file string, line int) string {
	parts := strings.Split(file, "/")
	if len(parts) > 1 {
		return strings.Join(parts[len(parts)-2:], "/") + ":" + strconv.Itoa(line)
	}
	return file + ":" + strconv.Itoa(line)
}

func getZerologLevel(level string) zerolog.Level {
	if level, _ := zerolog.ParseLevel(level); level != zerolog.NoLevel {
		return level
	}

	return zerolog.DebugLevel
}

func getFileByOutput(output string) *os.File {
	switch output {
	case "stdout":
		return os.Stdout
	default:
		f, err := os.OpenFile(output, os.O_RDWR|os.O_CREATE, 0600)
		if err != nil {
			f = os.Stdout
		}

		return f
	}
}
