package ssh

import (
	"trance-cli/internal/system"

	"github.com/spf13/cobra"
)

var executor = &Executor{}

var cmd = &cobra.Command{
	Use:   "ssh [wrapper-flags] [host] [-- ssh-options] [-- remote-command [arguments]]",
	Short: "Connect to an SSH host, with an interactive selector",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		executor.Run(cmd, args)
	},
	DisableFlagParsing: true,
}

func Register(parentCmd *cobra.Command) {
	cmd.Flags().BoolVarP(&executor.DryRun, "dry-run", "n", false, "Print the commands that would be executed, but do not execute them")
	cmd.Flags().BoolVarP(&executor.Proxy, "proxy", "p", false, "Create a SOCKS proxy on 0.0.0.0:1080")
	cmd.DisableFlagsInUseLine = true
	if system.IsCommandAvailable("ssh") {
		parentCmd.AddCommand(cmd)
	}
}
