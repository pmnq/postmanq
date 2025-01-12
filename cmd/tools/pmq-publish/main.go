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
	var file, srcQueue, destQueue, host, envelope, recipient, configURL string
	var code int
	flag.StringVar(&file, "f", common.ExampleConfigYaml, "configuration yaml file")
	flag.StringVar(&configURL, "u", common.InvalidInputString, "remote configurations file url")
	flag.StringVar(&srcQueue, "s", common.InvalidInputString, "source queue")
	flag.StringVar(&destQueue, "d", common.InvalidInputString, "destination queue")
	flag.StringVar(&host, "h", common.InvalidInputString, "amqp server hostname")
	flag.IntVar(&code, "c", common.InvalidInputInt, "error code")
	flag.StringVar(&envelope, "e", common.InvalidInputString, "necessary envelope")
	flag.StringVar(&recipient, "r", common.InvalidInputString, "necessary recipient")
	flag.Parse()

	app := application.NewPublish()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		app.SendEvents(common.NewApplicationEvent(common.FinishApplicationEventKind))
	}()

	if app.IsValidConfigFilename(file) &&
		srcQueue != common.InvalidInputString &&
		destQueue != common.InvalidInputString {
		app.SetConfigMeta(file, configURL, "")
		app.RunWithArgs(srcQueue, destQueue, host, code, envelope, recipient)
	} else {
		fmt.Println("Usage: pmq-publish -f -s -d [-h] [-c] [-e] [-r]")
		flag.VisitAll(common.PrintUsage)
		fmt.Println("Example:")
		fmt.Printf("  pmq-publish -f %s -s outbox.fail -d outbox\n", common.ExampleConfigYaml)
		fmt.Printf("  pmq-publish -f %s -s outbox.fail -d outbox -h example.com\n", common.ExampleConfigYaml)
		fmt.Printf("  pmq-publish -f %s -s outbox.fail -d outbox -c 554\n", common.ExampleConfigYaml)
	}
}
