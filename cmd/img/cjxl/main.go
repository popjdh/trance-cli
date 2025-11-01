package cjxl

import (
	"trance-cli/internal/system"

	"github.com/spf13/cobra"
)

var executor = &Executor{}

var cmd = &cobra.Command{
	Use:   "cjxl <file1> [<file2> ...]",
	Short: "Lossless cjxl wrapper",
	Long:  "",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		executor.Run(cmd, args)
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"jpg", "jpeg", "png", "bmp", "tiff", "gif", "webp"}, cobra.ShellCompDirectiveFilterFileExt
	},
}

func Register(parentCmd *cobra.Command) {
	cmd.Flags().BoolVarP(&executor.Verbose, "verbose", "v", false, "verbosely list files processed")
	cmd.Flags().BoolVarP(&executor.Recursive, "recursion", "r", true, "recurse into directories")
	if system.IsCommandAvailable("cjxl") {
		parentCmd.AddCommand(cmd)
	}
}
