package main

import (
	"flag"
	"fmt"

	"github.com/Halfi/postmanq/application"
	"github.com/Halfi/postmanq/common"
)

func main() {
	var file, configURL string
	flag.StringVar(&file, "f", common.ExampleConfigYaml, "configuration yaml file")
	flag.StringVar(&configURL, "u", common.InvalidInputString, "remote configurations file url")
	flag.Parse()

	app := application.NewReport()
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
