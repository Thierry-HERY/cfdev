package cmd

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/cfdev/config"
	gdn "code.cloudfoundry.org/cfdev/garden"
	"code.cloudfoundry.org/cfdev/shell"
	"code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/garden/client/connection"
	"github.com/spf13/cobra"
)

func NewBosh(Exit chan struct{}, UI UI, Config config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use: "bosh",
		Run: func(cmd *cobra.Command, args []string) {
			UI.Say(`Usage: eval $(cf dev bosh env)`)
		},
	}
	HideHelpFlag(cmd)
	envCmd := &cobra.Command{
		Use: "env",
		RunE: func(cmd *cobra.Command, args []string) error {
			go func() {
				<-Exit
				os.Exit(128)
			}()

			gClient := client.New(connection.New("tcp", "localhost:8888"))
			config, err := gdn.FetchBOSHConfig(gClient)
			if err != nil {
				return fmt.Errorf("failed to fetch bosh configuration: %v\n", err)
			}

			env := shell.Environment{StateDir: Config.StateDir}
			shellScript, err := env.Prepare(config)
			if err != nil {
				return fmt.Errorf("failed to prepare bosh configuration: %v\n", err)
			}

			UI.Say(shellScript)
			return nil
		},
	}
	HideHelpFlag(envCmd)
	cmd.AddCommand(envCmd)
	return cmd
}
