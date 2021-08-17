package limiter

import (
	"sync/atomic"
	"time"
)

// Cleaner чистильщик, проверяет значения ограничений и обнуляет значения ограничений
type Cleaner struct {
	service *Service
}

// создает нового чистильщика
func newCleaner(service *Service) {
	(&Cleaner{service: service}).clean()
}

// проверяет значения ограничений и обнуляет значения ограничений
func (c *Cleaner) clean() {
	for now := range c.service.ticker.C {
		// смотрим все ограничения
		for _, conf := range c.service.Configs {
			for _, limit := range conf.Limits {
				// проверяем дату последнего изменения ограничения
				if !limit.isValidDuration(now) {
					// если дата последнего изменения выходит за промежуток времени для проверки
					// обнулям текущее количество отправленных писем
					atomic.StoreInt32(&limit.currentValue, 0)
					limit.modifyDate = time.Now()
				}
			}
		}
	}
}
