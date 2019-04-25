package main

import (
	"github.com/Halfi/postmanq/common"
	"github.com/Halfi/postmanq/logger"
	"github.com/Halfi/postmanq/recipient"
	"runtime"
)

func main() {
	common.DefaultWorkersCount = runtime.NumCPU()
	logger.Inst()

	conf := &recipient.Config{
		ListenerCount: 10,
		MxHostnames:   []string{"localhost"},
	}
	service := recipient.Inst()
	service.(*recipient.Service).Configs = map[string]*recipient.Config{
		"localhost": conf,
	}
	service.OnRun()

	ch := make(chan bool)
	<-ch
}
