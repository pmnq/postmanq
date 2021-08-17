package connector

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"net"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
)

var (
	// сервис создания соединения
	service *Service

	// канал для приема событий отправки писем
	events       = make(chan *common.SendEvent)
	eventsClosed bool

	// почтовые сервисы будут хранится в карте по домену
	mailServers = make(map[string]*MailServer)

	cipherSuites = []uint16{
		tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	}
)

// Service сервис, управляющий соединениями к почтовым сервисам
// письма могут отсылаться в несколько потоков, почтовый сервис может разрешить несколько подключений с одного IP
// количество подключений может быть не равно количеству отсылающих потоков
// если доверить управление подключениями отправляющим потокам, тогда это затруднит общее управление подключениями
// поэтому создание подключений и предоставление имеющихся подключений отправляющим потокам вынесено в отдельный сервис
type Service struct {
	// количество горутин устанавливающих соединения к почтовым сервисам
	ConnectorsCount int `yaml:"workers"`

	Configs map[string]*Config `yaml:"postmans"`
}

// Inst создает новый сервис соединений
func Inst() *Service {
	if service == nil {
		service = new(Service)
	}
	return service
}

// OnInit инициализирует сервис соединений
func (s *Service) OnInit(event *common.ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, s)
	if err != nil {
		logger.All().ErrWithErr(err, "connection service can't unmarshal config")
	}

	if len(s.Configs) == 0 {
		logger.All().FailExit("connector config is empty")
		return
	}

	for name, config := range s.Configs {
		if config.MXHostname != "" {
			name = config.MXHostname
		}
		s.init(config, name)
	}

	if s.ConnectorsCount == 0 {
		s.ConnectorsCount = common.DefaultWorkersCount
	}
}

func (s *Service) init(conf *Config, hostname string) {
	conf.tlsConfig = getTLSConfig(conf.CertFilename, conf.PrivateKeyFilename, hostname)

	conf.addressesLen = len(conf.Addresses)
	if conf.addressesLen == 0 {
		logger.By(hostname).Warn("connection service - ips should be defined")
	}

	mxes, err := net.LookupMX(hostname)
	if err != nil {
		logger.By(hostname).Err("connection service - can't lookup mx for %s", hostname)
		return
	}

	conf.hostname = strings.TrimRight(mxes[0].Host, ".")
}

func getTLSConfig(certFilename, privateKeyFilename, hostname string) *tls.Config {
	if certFilename == "" {
		logger.By(hostname).Debug("connection service - certificate is not defined")
		return nil
	}

	// пытаемся прочитать сертификат
	pemBytes, err := ioutil.ReadFile(certFilename)
	if err != nil {
		logger.By(hostname).ErrWithErr(err, "connection service can't read certificate %s", certFilename)
		return nil
	}

	// получаем сертификат
	pemBlock, _ := pem.Decode(pemBytes)
	cert, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		logger.By(hostname).ErrWithErr(err, "connection parse certificate %s", certFilename)
		return nil
	}

	pool := x509.NewCertPool()
	pool.AddCert(cert)

	key, err := tls.LoadX509KeyPair(certFilename, privateKeyFilename)
	if err != nil {
		logger.By(hostname).ErrWithErr(err, "connection service can't load certificate %s, private key %s", certFilename, privateKeyFilename)
		return nil
	}

	return &tls.Config{
		ClientAuth:             tls.RequireAndVerifyClientCert,
		CipherSuites:           cipherSuites,
		MinVersion:             tls.VersionTLS12,
		SessionTicketsDisabled: true,
		RootCAs:                pool,
		ClientCAs:              pool,
		Certificates: []tls.Certificate{
			key,
		},
	}
}

// OnRun запускает горутины
func (s *Service) OnRun() {
	for i := 0; i < s.ConnectorsCount; i++ {
		id := i + 1
		go newPreparer(id)
		go newSeeker(id)
		go newConnector(id)
	}
}

// Event send event
func (s *Service) Event(ev *common.SendEvent) bool {
	if eventsClosed {
		return false
	}

	events <- ev
	return true
}

// OnFinish завершает работу сервиса соединений
func (s *Service) OnFinish() {
	if !eventsClosed {
		eventsClosed = true
		close(events)
	}
}

func (s *Service) Reconfigure(event *common.ApplicationEvent) {
	err := yaml.Unmarshal(event.Data, s)
	if err != nil {
		logger.All().ErrWithErr(err, "connection service can't unmarshal config")
		return
	}

	for name, config := range s.Configs {
		if config.MXHostname != "" {
			name = config.MXHostname
		}
		s.init(config, name)
	}

	if s.ConnectorsCount == 0 {
		s.ConnectorsCount = common.DefaultWorkersCount
	}
}

func (s Service) getTlsConfig(hostname string) *tls.Config {
	if conf, ok := s.Configs[hostname]; ok {
		return conf.tlsConfig
	} else {
		logger.By(hostname).Err("connection service can't make tls config by %s", hostname)
		return nil
	}
}

func (s Service) getAddresses(hostname string) []string {
	if conf, ok := s.Configs[hostname]; ok {
		return conf.Addresses
	} else {
		logger.By(hostname).Err("connection service can't find ips by %s", hostname)
		return common.EmptyStrSlice
	}
}

func (s Service) getAddress(hostname string, id int) string {
	if conf, ok := s.Configs[hostname]; ok && conf.addressesLen > 0 {
		return conf.Addresses[id%conf.addressesLen]
	} else {
		logger.By(hostname).Err("connection service can't find ip by %s", hostname)
		return common.EmptyStr
	}
}

func (s Service) getHostname(hostname string) string {
	if conf, ok := s.Configs[hostname]; ok {
		return conf.hostname
	} else {
		logger.By(hostname).Err("connection service can't find hostname by %s", hostname)
		return common.EmptyStr
	}
}

// ConnectionEvent событие создания соединения
type ConnectionEvent struct {
	*common.SendEvent

	// канал для получения почтового сервиса после поиска информации о его серверах
	servers chan *MailServer

	// почтовый сервис, которому будет отправлено письмо
	server *MailServer

	// идентификатор заготовщика запросившего поиск информации о почтовом сервисе
	connectorId int

	// адрес, с которого будет отправлено письмо
	address string
}

type Config struct {
	// PrivateKeyFilename путь до файла с закрытым ключом
	PrivateKeyFilename string `yaml:"privateKey"`

	// CertFilename путь до файла с сертификатом
	CertFilename string `yaml:"certificate"`

	// Addresses ip с которых будем рассылать письма
	Addresses []string `yaml:"ips"`

	// MXHostname hostname, на котором будет слушаться 25 порт
	MXHostname string `yaml:"mxHostname"`

	// количество ip
	addressesLen int

	tlsConfig *tls.Config

	hostname string
}
