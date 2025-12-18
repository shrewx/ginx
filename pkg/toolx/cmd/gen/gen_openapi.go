package gen

import (
	"context"
	"github.com/go-courier/packagesx"
	"github.com/shrewx/ginx/pkg/openapi"
	"github.com/spf13/cobra"
	"golang.org/x/tools/go/packages"
	"os"
	"strings"
)

var (
	path string
	tags string
)

func openapiCommand() *cobra.Command {
	openapi := &cobra.Command{
		Use:   "openapi",
		Short: "generate openapi swagger",
		Run: func(cmd *cobra.Command, args []string) {
			if path == "" {
				path, _ = os.Getwd()
			}

			var (
				pkg *packagesx.Package
				err error
			)

			// 如果指定了构建标签，使用 packages.Load 并设置 BuildFlags
			if tags != "" {
				// 支持逗号或空格分隔的构建标签
				tagValue := strings.ReplaceAll(tags, ",", " ")
				tagValue = strings.TrimSpace(tagValue)
				config := &packages.Config{
					Mode:       packages.LoadAllSyntax | packages.NeedImports,
					BuildFlags: []string{"-tags", tagValue},
				}
				pkgs, err := packages.Load(config, path)
				if err != nil {
					panic(err)
				}

				pkg = packagesx.NewPackage(pkgs[0])
			} else {
				// 默认行为，使用 packagesx.Load
				pkg, err = packagesx.Load(path)
				if err != nil {
					panic(err)
				}
			}

			g := openapi.NewOpenAPIGenerator(pkg)
			g.Scan(context.Background())
			g.Output(path)
		},
	}

	openapi.Flags().StringVarP(&path, "path", "p", "", "define the path of server")
	openapi.Flags().StringVarP(&tags, "tags", "t", "", "build tags to include (e.g., 'clouddge' or 'clouddge,dev')")

	return openapi
}
