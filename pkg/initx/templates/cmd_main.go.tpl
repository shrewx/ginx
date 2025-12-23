package main

import (
	"{{ .ProjectName }}/apis"
	"{{ .ProjectName }}/global"
	"github.com/shrewx/ginx"
	"github.com/spf13/cobra"
)

//go:generate toolx gen openapi

func main() {
	ginx.Launch(func(cmd *cobra.Command, args []string) {
		// 加载配置
		load(cmd)
		// 运行服务
		ginx.RunServer(&global.Config.Server, apis.RootRouter)
	})
}

func load(cmd *cobra.Command) {
	// 解析配置
	ginx.Parse(global.Config)
	// 加载数据库
	global.DBLoad(global.Config.DB)
}

