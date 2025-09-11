package cmd

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/go-courier/packagesx"
	"github.com/shrewx/ginx/pkg/openapi"
	"github.com/spf13/cobra"
	"log"
	"os"
)

const SwaggerImage = "swaggerapi/swagger-ui"

var (
	path        string
	swaggerPort int32
	serverUrl   string
)

func Swagger() *cobra.Command {
	swagger := &cobra.Command{
		Use:   "swagger",
		Short: "run swagger ui with docker",
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

			g.SetServer(serverUrl)
			g.Output("/tmp")

			cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err != nil {
				panic(err)
			}
			list, err := cli.ContainerList(context.Background(), container.ListOptions{
				All:     true,
				Filters: filters.NewArgs(filters.Arg("name", "ginx-swagger-openapi")),
			})
			if err != nil {
				panic(err)
			}

			if len(list) > 0 {
				err = cli.ContainerRemove(context.Background(), list[0].ID, container.RemoveOptions{
					RemoveVolumes: true,
					Force:         true,
				})

				if err != nil {
					panic(err)
				}
			}
			exposedPorts, portBindings, _ := nat.ParsePortSpecs([]string{
				fmt.Sprintf("127.0.0.1:%d:8080", swaggerPort),
			})
			resp, err := cli.ContainerCreate(context.Background(), &container.Config{
				Image:        SwaggerImage,
				ExposedPorts: exposedPorts,
				Env:          []string{"SWAGGER_JSON=/swagger/openapi.json"},
			}, &container.HostConfig{
				Binds:        []string{"/tmp/openapi.json:/swagger/openapi.json"},
				PortBindings: portBindings,
			}, nil, nil, "ginx-swagger-openapi")

			if err != nil {
				panic(err)
			}
			err = cli.ContainerStart(context.Background(), resp.ID, container.StartOptions{})
			if err != nil {
				panic(err)
			}

			log.Printf("docker start ginx-swagger-openapi container , visit http://127.0.0.1:%d", swaggerPort)
		},
	}

	swagger.Flags().Int32VarP(&swaggerPort, "swagger-port", "p", 9200, "define swagger server export port")
	swagger.Flags().StringVarP(&serverUrl, "server-host", "s", "http://127.0.0.1:8888", "define local api server port")

	return swagger
}
