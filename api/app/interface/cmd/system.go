package cmd

import (
	"log"
	"github.com/mikyk10/wisp/app/interface/cmd/util"
	"github.com/mikyk10/wisp/app/usecase"

	"github.com/goark/gocli/rwi"
	"github.com/spf13/cobra"
	"go.uber.org/dig"
)

func NewSystemPruneCommand(c *dig.Container) *cobra.Command {

	var ui *rwi.RWI
	var sysuc usecase.SystemUsecase
	if err := c.Invoke(func(uc usecase.SystemUsecase, r *rwi.RWI) {
		sysuc = uc
		ui = r
	}); err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Destroy all the data",
		RunE: func(cmd *cobra.Command, args []string) error {

			const msg = `WARNING! This will remove:
- all indexed image catalog
- all cache
			
Are you sure you want to continue?`

			// Ask the user whether to continue.
			yes, _ := cmd.Flags().GetBool("yes")
			if err := util.IgnoblePromptYn(ui, msg, yes); err != nil {
				ui.OutputErrln("aborted")
				return err
			}

			return sysuc.Prune()
		}}
	cmd.Flags().Bool("yes", false, "Always assume yes to all prompts")

	return cmd
}
