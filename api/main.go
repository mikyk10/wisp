package main

import (
	"os"

	"github.com/mikyk10/wisp/app/di"
	"github.com/mikyk10/wisp/app/infra/config"
	"github.com/mikyk10/wisp/app/interface/cmd"

	"github.com/goark/gocli/rwi"
)

func main() {

	rwi := rwi.New(
		rwi.WithReader(os.Stdin),
		rwi.WithWriter(os.Stdout),
		rwi.WithErrorWriter(os.Stderr),
	)

	configLoader := config.NewDefaultConfigLoader()
	globalConfig, serviceConfig, err := configLoader.LoadConfig()
	if err != nil {
		rwi.OutputErrln(err) //nolint:errcheck
		os.Exit(1)
	}

	container := di.NewBuilder().
		WithConfig(globalConfig, serviceConfig).
		WithDatabase(globalConfig).
		WithRWI(rwi).
		Build()

	cmd.Execute(
		container,
		os.Args[1:],
	).Exit()

	//
}
