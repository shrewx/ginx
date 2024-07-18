package gen

import (
	"github.com/shrewx/ginx/pkg/client"
	"github.com/spf13/cobra"
	"net/url"
	"os"
)

var (
	openUrl     string
	serviceName string
)

func clientCommand() *cobra.Command {
	client := &cobra.Command{
		Use:   "client",
		Short: "generate client",
		Run: func(cmd *cobra.Command, args []string) {
			if path == "" {
				path, _ = os.Getwd()
			}

			u, err := url.Parse(openUrl)
			if err != nil {
				panic(err)
			}
			g := client.NewClientGenerator(serviceName, u)
			g.Load()
			g.Output(path)
		},
	}

	client.Flags().StringVarP(&serviceName, "serviceName", "s", "", " service name")
	client.Flags().StringVarP(&openUrl, "url", "u", "", "the url to get openapi info")

	return client
}
