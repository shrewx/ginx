package gen

import (
	"github.com/go-courier/packagesx"
	"github.com/shrewx/ginx/pkg/i18nx"
	"github.com/spf13/cobra"
	"os"
)

func i18nCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "i18n",
		Short: "generate i18n file",
		Run: func(cmd *cobra.Command, args []string) {
			pwd, _ := os.Getwd()
			pkg, err := packagesx.Load(pwd)
			if err != nil {
				panic(err)
			}

			g := i18nx.NewI18nGenerator(pkg)
			g.Scan(args...)
			g.Output(pwd)
		},
	}
}
