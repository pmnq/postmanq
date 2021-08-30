package guardian

import (
	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
)

// Guardian защитник, блокирует отправку на указанные почтовые сервисы
type Guardian struct {
	// идентификатор для логов
	id int
	s  *Service
}

// создает нового защитника
func newGuardian(id int, s *Service) {
	guardian := &Guardian{id: id, s: s}
	guardian.run()
}

// запускает прослушивание событий отправки писем
func (g *Guardian) run() {
	for event := range g.s.events {
		g.guard(event)
	}
}

// блокирует отправку на указанные почтовые сервисы
func (g *Guardian) guard(event *common.SendEvent) {
	logger.By(event.Message.HostnameFrom).Info("guardian#%d-%d check mail", g.id, event.Message.Id)

	excludes := g.s.getExcludes(event.Message.HostnameFrom)
	isExclude := false
	for _, exclude := range excludes {
		if exclude == event.Message.HostnameTo {
			isExclude = true
			break
		}
	}

	if isExclude {
		logger.By(event.Message.HostnameFrom).Debug("guardian#%d-%d detect postal worker - %s, revoke sending mail", g.id, event.Message.Id, event.Message.HostnameTo)
		event.Result <- common.RevokeSendEventResult
	} else {
		logger.By(event.Message.HostnameFrom).Debug("guardian#%d-%d continue sending mail", g.id, event.Message.Id)
		event.Iterator.Next().(common.SendingService).Event(event)
	}
}
