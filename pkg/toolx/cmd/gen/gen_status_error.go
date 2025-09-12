package gen

import (
	"os"

	"github.com/go-courier/packagesx"
	"github.com/shrewx/ginx/pkg/statuserror"
	"github.com/spf13/cobra"
)

func statusErrorCommand() *cobra.Command {
	var (
		prefix    string
		className string
	)
	cmd := &cobra.Command{
		Use:   "error",
		Short: "generate status error file",
		Run: func(cmd *cobra.Command, args []string) {
			pwd, _ := os.Getwd()
			pkg, err := packagesx.Load(pwd)
			if err != nil {
				panic(err)
			}

			g := statuserror.NewStatusErrorGenerator(pkg)
			g.Scan(className)
			// 生成Go代码
			g.Output(pwd, prefix)
		},
	}

	cmd.Flags().StringVarP(&prefix, "prefix", "p", "", "prefix of error code")
	cmd.Flags().StringVarP(&className, "class", "c", "", "class name")
	return cmd
}

func statusErrorYamlCommand() *cobra.Command {
	var (
		prefix    string
		className string
		outputDir string
	)
	cmd := &cobra.Command{
		Use:   "errorYaml",
		Short: "generate status error file",
		Run: func(cmd *cobra.Command, args []string) {
			pwd, _ := os.Getwd()
			pkg, err := packagesx.Load(pwd)
			if err != nil {
				panic(err)
			}

			g := statuserror.NewStatusErrorGenerator(pkg)
			g.Scan(className)
			// 生成Go代码
			g.OutputYAML(pwd, prefix, outputDir)
		},
	}

	cmd.Flags().StringVarP(&prefix, "prefix", "p", "", "prefix of error code")
	cmd.Flags().StringVarP(&outputDir, "outputDir", "o", "", "output directory for YAML files")
	cmd.Flags().StringVarP(&className, "class", "c", "", "class name")

	return cmd
}
