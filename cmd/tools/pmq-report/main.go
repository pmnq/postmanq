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

func main() {
	var file, configURL string
	flag.StringVar(&file, "f", common.ExampleConfigYaml, "configuration yaml file")
	flag.StringVar(&configURL, "u", common.InvalidInputString, "remote configurations file url")
	flag.Parse()

	app := application.NewReport()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		app.SendEvents(common.NewApplicationEvent(common.FinishApplicationEventKind))
	}()

	if app.IsValidConfigFilename(file) {
		app.SetConfigMeta(file, configURL, "")
		app.Run()
	} else {
		fmt.Println("Usage: pmq-report -f")
		flag.VisitAll(common.PrintUsage)
		fmt.Println("Example:")
		fmt.Printf("  pmq-report -f %s\n", common.ExampleConfigYaml)
	}
}
