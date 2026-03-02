package cmd

import (
	"log"
	"wspf/app/domain/model"

	"github.com/goark/gocli/rwi"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

func NewVersionCommand(c *dig.Container) *cobra.Command {

	var ui *rwi.RWI
	if err := c.Invoke(func(u *rwi.RWI) {
		ui = u
	}); err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}

	return &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Run: func(cmd *cobra.Command, args []string) {
			ui.OutputErrln(model.AppVersionString())
		}}
}
