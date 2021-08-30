package limiter

import (
	"context"
	"sync/atomic"
	"time"
)

// Cleaner чистильщик, проверяет значения ограничений и обнуляет значения ограничений
type Cleaner struct {
	service *Service
	ticker  *time.Ticker
	ctx     context.Context
	done    context.CancelFunc
}

// создает нового чистильщика
func newCleaner(service *Service) *Cleaner {
	ctx, done := context.WithCancel(context.Background())
	return &Cleaner{service: service, ticker: time.NewTicker(time.Second), ctx: ctx, done: done}
}

// проверяет значения ограничений и обнуляет значения ограничений
func (c *Cleaner) run() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case now := <-c.ticker.C:
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
}

func (c *Cleaner) stop() {
	c.ticker.Stop()
	c.done()
}
