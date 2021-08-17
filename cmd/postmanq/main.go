package main

import (
	"flag"
	"fmt"
	"os"

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
	if app.IsValidConfigFilename(file) {
		app.SetConfigMeta(file, configURL, configUpdateDuration)
		app.Run()
	} else {
		fmt.Printf("Usage: postmanq -f %s\n", common.ExampleConfigYaml)
		flag.VisitAll(common.PrintUsage)
	}
}
