package gen

import (
	"github.com/go-courier/packagesx"
	"github.com/shrewx/ginx/pkg/i18nx"
	"github.com/spf13/cobra"
	"os"
)

func i18nCommand() *cobra.Command {
	var (
		prefix    string
		className string
	)
	cmd := &cobra.Command{
		Use:   "i18n",
		Short: "generate i18n file",
		Run: func(cmd *cobra.Command, args []string) {
			pwd, _ := os.Getwd()
			pkg, err := packagesx.Load(pwd)
			if err != nil {
				panic(err)
			}

			g := i18nx.NewI18nGenerator(pkg)
			g.Scan(className)
			g.Output(pwd, prefix)
		},
	}

	cmd.Flags().StringVarP(&prefix, "prefix", "p", "", "prefix of error code")
	cmd.Flags().StringVarP(&className, "class", "c", "", "class name")

	return cmd
}

func statusI18nYamlCommand() *cobra.Command {
	var (
		prefix    string
		className string
		outputDir string
	)
	cmd := &cobra.Command{
		Use:   "i18nYaml",
		Short: "generate i18n yaml file",
		Run: func(cmd *cobra.Command, args []string) {
			pwd, _ := os.Getwd()
			pkg, err := packagesx.Load(pwd)
			if err != nil {
				panic(err)
			}

			g := i18nx.NewI18nGenerator(pkg)
			g.Scan(className)

			g.OutputYAML(pwd, prefix, outputDir)
		},
	}

	cmd.Flags().StringVarP(&prefix, "prefix", "p", "", "prefix of error code")
	cmd.Flags().StringVarP(&outputDir, "outputDir", "o", "", "output directory for YAML files")
	cmd.Flags().StringVarP(&className, "class", "c", "", "class name")

	return cmd
}
