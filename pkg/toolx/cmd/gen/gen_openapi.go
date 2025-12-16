package gen

import (
	"context"
	"github.com/go-courier/packagesx"
	"github.com/shrewx/ginx/pkg/openapi"
	"github.com/spf13/cobra"
	"os"
)

var (
	path string
)

func openapiCommand() *cobra.Command {
	openapi := &cobra.Command{
		Use:   "openapi",
		Short: "generate openapi swagger",
		Run: func(cmd *cobra.Command, args []string) {
			if path == "" {
				path, _ = os.Getwd()
			}

			pkg, err := packagesx.Load(path)
			if err != nil {
				panic(err)
			}
			g := openapi.NewOpenAPIGenerator(pkg)
			g.Scan(context.Background())
			g.Output(path)
		},
	}

	openapi.Flags().StringVarP(&path, "path", "p", "", "define the path of server")

	return openapi
}
