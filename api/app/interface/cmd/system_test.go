package cmd_test

import (
	"bytes"
	"os"
	"testing"
	"github.com/mikyk10/wisp/app/di"
	"github.com/mikyk10/wisp/app/domain/model/config"

	"github.com/mikyk10/wisp/app/interface/cmd"

	"github.com/goark/gocli/rwi"
)

func TestSystemPruneCommand(t *testing.T) {
	sut := &bytes.Buffer{}
	rwif := rwi.New(
		rwi.WithReader(os.Stdin),
		rwi.WithWriter(sut),
		rwi.WithErrorWriter(sut),
	)

	globalConfig := &config.GlobalConfig{}
	serviceConfig := &config.ServiceConfig{
		Catalog:  map[string]*config.ImageProviderConfig{},
		Displays: map[string]*config.DisplayConfig{},
	}

	container := di.NewBuilder().WithConfig(globalConfig, serviceConfig).WithSQLiteMock().WithRWI(rwif).Build()

	cmd.Execute(
		container,
		[]string{"system", "prune", "--yes"},
	)
}
