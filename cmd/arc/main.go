package arc

import (
	"trance-cli/cmd/arc/unpack"

	"github.com/spf13/cobra"
)

var cmd = &cobra.Command{
	Use:   "arc",
	Short: "A archive script snippet collection",
	Long:  "",
}

func Register(parentCmd *cobra.Command) {
	unpack.Register(cmd)
	parentCmd.AddCommand(cmd)
}
