package cmd

import (
	"fmt"
	"os"
	"trance-cli/cmd/img"
	"trance-cli/cmd/ssh"

	"github.com/spf13/cobra"
)

var cmd = &cobra.Command{
	Use:   "trance",
	Short: "A script snippet collection",
	Long:  "",
}

func Execute() {
	img.Register(cmd)
	ssh.Register(cmd)
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
