package tmpl

import (
	"github.com/spf13/cobra"
)

var executor = &Executor{}

var cmd = &cobra.Command{
	Use:   "tmpl",
	Short: "Interactive template builder",
	Long:  "",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		executor.Run(cmd, args)
	},
}

func Register(parentCmd *cobra.Command) {
	cmd.Flags().StringVarP(&executor.TemplateRegistryDir, "template-registry", "d", "template-registry", "template registry directory")
	parentCmd.AddCommand(cmd)
}
