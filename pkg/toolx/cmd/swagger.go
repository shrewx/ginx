package cmd

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/go-courier/packagesx"
	"github.com/shrewx/ginx/pkg/openapi"
	"github.com/spf13/cobra"
	"golang.org/x/tools/go/packages"
)

const SwaggerImage = "swaggerapi/swagger-ui"
const NginxImage = "nginx:alpine"
const SwaggerNetworkName = "ginx-swagger-network"

var (
	path        string
	swaggerPort int32
	serverUrl   string
	tags        string
)

//go:embed certs/server.crt
var embeddedSwaggerCrt []byte

//go:embed certs/server.key
var embeddedSwaggerKey []byte

func Swagger() *cobra.Command {
	swagger := &cobra.Command{
		Use:   "swagger",
		Short: "run swagger ui with docker",
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
			// 检查 serverUrl 是否是 HTTPS
			parsedURL, err := url.Parse(serverUrl)
			if err != nil {
				panic(fmt.Errorf("invalid server URL: %w", err))
			}

			isHTTPS := parsedURL.Scheme == "https"
			if !isHTTPS {
				g.SetServer(serverUrl)
			} else {
				g.SetServer(fmt.Sprintf("https://127.0.0.1:%d/api", swaggerPort))
			}

			g.Output("/tmp")

			if isHTTPS {
				// 使用 nginx 反向代理支持 HTTPS
				if err := setupHTTPSProxy(swaggerPort, serverUrl); err != nil {
					panic(err)
				}
			} else {
				// 原有逻辑：直接启动 swagger-ui
				if err := startSwaggerDirect(swaggerPort); err != nil {
					panic(err)
				}
			}
		},
	}

	swagger.Flags().Int32VarP(&swaggerPort, "swagger-port", "p", 9200, "define swagger server export port")
	swagger.Flags().StringVarP(&serverUrl, "server-host", "s", "http://127.0.0.1:8888", "define local api server port")
	swagger.Flags().StringVarP(&tags, "tags", "t", "", "define build tags")
	return swagger
}

// startSwaggerDirect 直接启动 swagger-ui 容器（HTTP 模式）
func startSwaggerDirect(port int32) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}
	list, err := cli.ContainerList(context.Background(), container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("name", "ginx-swagger-openapi")),
	})
	if err != nil {
		return err
	}

	if len(list) > 0 {
		err = cli.ContainerRemove(context.Background(), list[0].ID, container.RemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		})

		if err != nil {
			return err
		}
	}
	exposedPorts, portBindings, _ := nat.ParsePortSpecs([]string{
		fmt.Sprintf("127.0.0.1:%d:8080", port),
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
		return err
	}
	err = cli.ContainerStart(context.Background(), resp.ID, container.StartOptions{})
	if err != nil {
		return err
	}

	log.Printf("docker start ginx-swagger-openapi container , visit http://127.0.0.1:%d", port)
	return nil
}

// setupHTTPSProxy 设置 HTTPS 反向代理（使用 nginx + swagger-ui）
func setupHTTPSProxy(port int32, apiServerUrl string) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return err
	}

	ctx := context.Background()

	// 停止并删除现有容器
	removeContainer(cli, ctx, "ginx-swagger-openapi")
	removeContainer(cli, ctx, "ginx-swagger-nginx")

	// 创建或获取网络
	networkID, err := ensureNetwork(cli, ctx)
	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	// 将嵌入的证书文件写入到 /tmp 目录
	certFile := "/tmp/swagger.crt"
	keyFile := "/tmp/swagger.key"

	if len(embeddedSwaggerCrt) == 0 || len(embeddedSwaggerKey) == 0 {
		return fmt.Errorf("embedded certificate files not found. Please ensure certs/swagger.crt and certs/swagger.key exist")
	}

	// 写入证书文件到 /tmp
	if err := os.WriteFile(certFile, embeddedSwaggerCrt, 0644); err != nil {
		return fmt.Errorf("failed to write certificate file: %w", err)
	}
	if err := os.WriteFile(keyFile, embeddedSwaggerKey, 0600); err != nil {
		return fmt.Errorf("failed to write certificate key file: %w", err)
	}

	// 创建临时目录用于 nginx 配置和证书
	swaggerHTTPSDir := filepath.Join(os.TempDir(), "swagger-https")
	nginxDir := filepath.Join(swaggerHTTPSDir, "nginx")
	certsTmpDir := filepath.Join(swaggerHTTPSDir, "certs")

	if err := os.MkdirAll(nginxDir, 0755); err != nil {
		return fmt.Errorf("failed to create nginx directory: %w", err)
	}
	if err := os.MkdirAll(certsTmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	// 将证书文件复制到临时目录
	certTmpPath := filepath.Join(certsTmpDir, "swagger.crt")
	keyTmpPath := filepath.Join(certsTmpDir, "swagger.key")
	if err := os.WriteFile(certTmpPath, embeddedSwaggerCrt, 0644); err != nil {
		return fmt.Errorf("failed to copy certificate file: %w", err)
	}
	if err := os.WriteFile(keyTmpPath, embeddedSwaggerKey, 0600); err != nil {
		return fmt.Errorf("failed to copy certificate key file: %w", err)
	}

	// 处理 API server URL，确保末尾有斜杠（用于去掉 /api/ 前缀）
	// 同时需要将 127.0.0.1 或 localhost 替换为 host.docker.internal，以便容器能访问宿主机
	parsedAPIUrl, err := url.Parse(apiServerUrl)
	if err != nil {
		return fmt.Errorf("invalid API server URL: %w", err)
	}

	// 如果 host 是 127.0.0.1 或 localhost，替换为 host.docker.internal
	host := parsedAPIUrl.Hostname()
	if host == "127.0.0.1" || host == "localhost" {
		port := parsedAPIUrl.Port()
		if port != "" {
			parsedAPIUrl.Host = fmt.Sprintf("host.docker.internal:%s", port)
		} else {
			// 根据 scheme 设置默认端口
			if parsedAPIUrl.Scheme == "https" {
				parsedAPIUrl.Host = "host.docker.internal:443"
			} else {
				parsedAPIUrl.Host = "host.docker.internal:80"
			}
		}
	}

	apiProxyUrl := parsedAPIUrl.String()
	if !strings.HasSuffix(apiProxyUrl, "/") {
		apiProxyUrl += "/"
	}

	// 创建 nginx 配置文件
	nginxConfig := fmt.Sprintf(`server {
    listen 443 ssl;
    server_name localhost;

    ssl_certificate     /etc/nginx/certs/swagger.crt;
    ssl_certificate_key /etc/nginx/certs/swagger.key;

    # 前端 Swagger UI
    location / {
        proxy_pass http://ginx-swagger-openapi:8080/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # API 反向代理
    location /api/ {
        proxy_pass %s;
        proxy_ssl_verify off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
`, apiProxyUrl)

	nginxConfigPath := filepath.Join(nginxDir, "default.conf")
	if err := os.WriteFile(nginxConfigPath, []byte(nginxConfig), 0644); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	// 创建 swagger 容器
	swaggerExposedPorts, _, _ := nat.ParsePortSpecs([]string{"8080"})
	swaggerResp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:        SwaggerImage,
		ExposedPorts: swaggerExposedPorts,
		Env:          []string{"SWAGGER_JSON=/swagger/openapi.json"},
	}, &container.HostConfig{
		Binds: []string{"/tmp/openapi.json:/swagger/openapi.json"},
	}, nil, nil, "ginx-swagger-openapi")
	if err != nil {
		return fmt.Errorf("failed to create swagger container: %w", err)
	}

	// 连接 swagger 容器到网络
	if err := cli.NetworkConnect(ctx, networkID, swaggerResp.ID, &network.EndpointSettings{}); err != nil {
		return fmt.Errorf("failed to connect swagger container to network: %w", err)
	}

	// 启动 swagger 容器
	if err := cli.ContainerStart(ctx, swaggerResp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start swagger container: %w", err)
	}

	// 创建 nginx 容器
	nginxExposedPorts, nginxPortBindings, _ := nat.ParsePortSpecs([]string{
		fmt.Sprintf("127.0.0.1:%d:443", port),
	})

	// 添加 extra_hosts 以便容器能访问宿主机（host.docker.internal）
	extraHosts := []string{"host.docker.internal:host-gateway"}

	nginxResp, err := cli.ContainerCreate(ctx, &container.Config{
		Image:        NginxImage,
		ExposedPorts: nginxExposedPorts,
	}, &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/etc/nginx/certs:ro", certsTmpDir),
			fmt.Sprintf("%s:/etc/nginx/conf.d:ro", nginxDir),
		},
		PortBindings: nginxPortBindings,
		ExtraHosts:   extraHosts,
	}, nil, nil, "ginx-swagger-nginx")
	if err != nil {
		return fmt.Errorf("failed to create nginx container: %w", err)
	}

	// 连接 nginx 容器到网络
	if err := cli.NetworkConnect(ctx, networkID, nginxResp.ID, &network.EndpointSettings{}); err != nil {
		return fmt.Errorf("failed to connect nginx container to network: %w", err)
	}

	// 启动 nginx 容器
	if err := cli.ContainerStart(ctx, nginxResp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start nginx container: %w", err)
	}

	log.Printf("HTTPS Swagger UI started successfully! Visit https://127.0.0.1:%d", port)
	log.Println("Note: You may need to accept the self-signed certificate in your browser.")
	return nil
}

// ensureNetwork 创建或获取网络
func ensureNetwork(cli *client.Client, ctx context.Context) (string, error) {
	// 检查网络是否已存在
	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(filters.Arg("name", SwaggerNetworkName)),
	})
	if err != nil {
		return "", err
	}

	if len(networks) > 0 {
		return networks[0].ID, nil
	}

	// 创建新网络
	resp, err := cli.NetworkCreate(ctx, SwaggerNetworkName, types.NetworkCreate{
		Driver: "bridge",
	})
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

// removeContainer 停止并删除容器
func removeContainer(cli *client.Client, ctx context.Context, containerName string) {
	list, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("name", containerName)),
	})
	if err != nil || len(list) == 0 {
		return
	}

	cli.ContainerRemove(ctx, list[0].ID, container.RemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
}
