package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Halfi/postmanq/application"
	"github.com/Halfi/postmanq/common"
)

const envPrefix = "POSTMANQ_"

func main() {
	var file, configURL, configUpdateDuration string

	flag.StringVar(&file, "f", common.ExampleConfigYaml, "configuration yaml file")
	flag.StringVar(&configURL, "u", common.InvalidInputString, "remote configurations file url")
	flag.StringVar(&configUpdateDuration, "t", common.InvalidInputString, "config update duration")
	flag.Parse()

	if url := os.Getenv(fmt.Sprintf("%sCONFIG_URL", envPrefix)); url != "" {
		configURL = url
	}

	if updateDuration := os.Getenv(fmt.Sprintf("%sCONFIG_UPDATE_DURATION", envPrefix)); updateDuration != "" {
		configUpdateDuration = updateDuration
	}

	app := application.NewPost()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		app.SendEvents(common.NewApplicationEvent(common.FinishApplicationEventKind))
	}()

	if app.IsValidConfigFilename(file) {
		app.SetConfigMeta(file, configURL, configUpdateDuration)
		app.Run()
	} else {
		fmt.Printf("Usage: postmanq -f %s\n", common.ExampleConfigYaml)
		flag.VisitAll(common.PrintUsage)
	}
}
