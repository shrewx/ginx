package gen

import (
	"github.com/go-courier/packagesx"
	"github.com/shrewx/ginx/pkg/enum"

	"github.com/spf13/cobra"
	"os"
)

func enumCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "enum",
		Short: "generate enum file",
		Run: func(cmd *cobra.Command, args []string) {
			pwd, _ := os.Getwd()
			pkg, err := packagesx.Load(pwd)
			if err != nil {
				panic(err)
			}

			g := enum.NewEnumGenerator(pkg)
			g.Scan(args...)
			g.Output(pwd)
		},
	}
}
