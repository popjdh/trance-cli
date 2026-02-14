package img

import (
	"trance-cli/cmd/img/cjxl"
	"trance-cli/cmd/img/noexif"

	"github.com/spf13/cobra"
)

var cmd = &cobra.Command{
	Use:   "img",
	Short: "A image script snippet collection",
	Long:  "",
}

func Register(parentCmd *cobra.Command) {
	cjxl.Register(cmd)
	noexif.Register(cmd)
	parentCmd.AddCommand(cmd)
}
