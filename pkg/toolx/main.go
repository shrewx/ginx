package main

import (
	"fmt"
	"github.com/shrewx/ginx/pkg/toolx/cmd"
	"github.com/shrewx/ginx/pkg/toolx/cmd/gen"

	"github.com/spf13/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use: "toolx",
}

func init() {
	rootCmd.AddCommand(gen.CmdGen)
	rootCmd.AddCommand(cmd.Swagger())
	rootCmd.AddCommand(cmd.Init())
	rootCmd.AddCommand(cmd.InstallSkill())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
