package cmd_test

import (
	"bytes"
	"os"
	"testing"
	"wspf/app/di"
	"wspf/app/domain/model"
	"wspf/app/domain/model/config"

	"wspf/app/interface/cmd"

	"github.com/goark/gocli/rwi"
	"github.com/stretchr/testify/assert"
)

func TestVersionCommand(t *testing.T) {

	model.AppVersion = "0.0.0"
	model.CommitHash = "deadbeaf"
	model.BuildTime = "2023-01-01"

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
		[]string{"version"},
	)

	assert.Equal(t, "0.0.0(deadbeaf,2023-01-01)\n", sut.String())
}
