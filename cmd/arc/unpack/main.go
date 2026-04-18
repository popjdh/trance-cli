package unpack

import (
	"trance-cli/internal/system"

	"github.com/spf13/cobra"
)

var executor = &Executor{}

var cmd = &cobra.Command{
	Use:   "unpack <file1|dir1> [<file2|dir2> ...]",
	Short: "Archive unpacking cli wrapper with password support and structured output",
	Long:  "",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		executor.Run(cmd, args)
	},
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"7z", "7z.001", "rar", "part1.rar", "zip", "zip.001", "tar", "tar.bz2", "tar.z", "tar.gz", "tar.lz4", "tar.lz", "tar.lzma", "tar.xz", "tar.zst"}, cobra.ShellCompDirectiveFilterFileExt
	},
}

func Register(parentCmd *cobra.Command) {
	cmd.Flags().BoolVarP(&executor.Verbose, "verbose", "v", false, "verbosely list files processed")
	cmd.Flags().BoolVarP(&executor.Recursive, "recursion", "r", false, "recurse into directories")
	cmd.Flags().StringSliceVarP(&executor.Passwords, "password", "p", nil, "password to try (allow multiple -p)")
	cmd.Flags().StringVarP(&executor.PasswordFile, "password-file", "P", "", "password list file (one password per line)")
	if system.IsCommandAvailable("7z") && system.IsCommandAvailable("unzip") {
		parentCmd.AddCommand(cmd)
	}
}
