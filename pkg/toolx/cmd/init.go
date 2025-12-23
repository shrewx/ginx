package cmd

import (
	"fmt"
	"github.com/shrewx/ginx/pkg/initx"
	"github.com/shrewx/ginx/pkg/utils"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func Init() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [project-name]",
		Short: "initialize a new ginx project",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			projectName := args[0]
			if projectName == "" {
				fmt.Println("Error: project name is required")
				os.Exit(1)
			}

			pwd, err := os.Getwd()
			if err != nil {
				panic(err)
			}

			// 创建项目根目录
			projectRoot := filepath.Join(pwd, projectName)

			// 检查项目目录是否已存在
			if _, err := os.Stat(projectRoot); err == nil {
				panic(fmt.Errorf("project directory already exists: %s", projectRoot))
			}

			// 创建项目根目录
			if err := os.MkdirAll(projectRoot, 0755); err != nil {
				panic(fmt.Errorf("failed to create project directory %s: %v", projectRoot, err))
			}

			// 初始化 Go module
			fmt.Printf("🚀 Initializing Go module...\n")
			modInitCmd := exec.Command("go", "mod", "init", projectName)
			modInitCmd.Dir = projectRoot
			modInitCmd.Stdout = os.Stdout
			modInitCmd.Stderr = os.Stderr
			if err := modInitCmd.Run(); err != nil {
				panic(fmt.Errorf("failed to initialize Go module: %v", err))
			}

			// 创建目录结构（在项目根目录下）
			dirs := []string{
				"apis/user",
				fmt.Sprintf("cmd/%s", projectName),
				"constants/status_error",
				"global",
				"i18n",
				"pkg",
				"services",
				"repositories/controllers",
				"repositories/models",
			}

			for _, dir := range dirs {
				fullPath := filepath.Join(projectRoot, dir)
				if err := os.MkdirAll(fullPath, 0755); err != nil {
					panic(fmt.Errorf("failed to create directory %s: %v", dir, err))
				}
			}

			// 生成文件
			templateData := map[string]interface{}{
				"ProjectName": projectName,
			}

			// 生成 apis/root.go
			apisRootPath := filepath.Join(projectRoot, "apis", "root.go")
			if err := generateFile(apisRootPath, initx.ApisRootTemplate, templateData); err != nil {
				panic(fmt.Errorf("failed to generate apis/root.go: %v", err))
			}

			// 生成 global/config.go
			globalConfigPath := filepath.Join(projectRoot, "global", "config.go")
			if err := generateFile(globalConfigPath, initx.GlobalConfigTemplate, templateData); err != nil {
				panic(fmt.Errorf("failed to generate global/config.go: %v", err))
			}

			// 生成 cmd/{projectName}/main.go
			cmdMainPath := filepath.Join(projectRoot, "cmd", projectName, "main.go")
			if err := generateFile(cmdMainPath, initx.CmdMainTemplate, templateData); err != nil {
				panic(fmt.Errorf("failed to generate cmd/%s/main.go: %v", projectName, err))
			}

			// 生成 constants/status_error/error.go
			constantsErrorPath := filepath.Join(projectRoot, "constants", "status_error", "error.go")
			if err := generateFile(constantsErrorPath, initx.ConstantsErrorTemplate, templateData); err != nil {
				panic(fmt.Errorf("failed to generate constants/status_error/error.go: %v", err))
			}

			// 生成 cmd/{projectName}/local-config.yaml
			localConfigPath := filepath.Join(projectRoot, "cmd", projectName, "local-config.yaml")
			if err := generateFile(localConfigPath, initx.LocalConfigTemplate, templateData); err != nil {
				panic(fmt.Errorf("failed to generate cmd/%s/local-config.yaml: %v", projectName, err))
			}

			// 生成 repositories/models/user.go
			modelsUserPath := filepath.Join(projectRoot, "repositories", "models", "user.go")
			if err := generateFile(modelsUserPath, initx.RepositoriesModelsUserTemplate, templateData); err != nil {
				panic(fmt.Errorf("failed to generate repositories/models/user.go: %v", err))
			}

			// 生成 repositories/controllers/user_controller.go
			controllersUserPath := filepath.Join(projectRoot, "repositories", "controllers", "user_controller.go")
			if err := generateFile(controllersUserPath, initx.RepositoriesControllersUserControllerTemplate, templateData); err != nil {
				panic(fmt.Errorf("failed to generate repositories/controllers/user_controller.go: %v", err))
			}

			// 生成 services/user_service.go
			servicesUserPath := filepath.Join(projectRoot, "services", "user_service.go")
			if err := generateFile(servicesUserPath, initx.ServicesUserServiceTemplate, templateData); err != nil {
				panic(fmt.Errorf("failed to generate services/user_service.go: %v", err))
			}

			// 生成 apis/user/router.go
			apisUserRouterPath := filepath.Join(projectRoot, "apis", "user", "router.go")
			if err := generateFile(apisUserRouterPath, initx.ApisUserRouterTemplate, templateData); err != nil {
				panic(fmt.Errorf("failed to generate apis/user/router.go: %v", err))
			}

			// 生成 apis/user/create_user.go
			apisUserCreatePath := filepath.Join(projectRoot, "apis", "user", "create_user.go")
			if err := generateFile(apisUserCreatePath, initx.ApisUserCreateUserTemplate, templateData); err != nil {
				panic(fmt.Errorf("failed to generate apis/user/create_user.go: %v", err))
			}

			// 生成 apis/user/get_user.go
			apisUserGetPath := filepath.Join(projectRoot, "apis", "user", "get_user.go")
			if err := generateFile(apisUserGetPath, initx.ApisUserGetUserTemplate, templateData); err != nil {
				panic(fmt.Errorf("failed to generate apis/user/get_user.go: %v", err))
			}

			// 生成 apis/user/update_user.go
			apisUserUpdatePath := filepath.Join(projectRoot, "apis", "user", "update_user.go")
			if err := generateFile(apisUserUpdatePath, initx.ApisUserUpdateUserTemplate, templateData); err != nil {
				panic(fmt.Errorf("failed to generate apis/user/update_user.go: %v", err))
			}

			// 生成 apis/user/delete_user.go
			apisUserDeletePath := filepath.Join(projectRoot, "apis", "user", "delete_user.go")
			if err := generateFile(apisUserDeletePath, initx.ApisUserDeleteUserTemplate, templateData); err != nil {
				panic(fmt.Errorf("failed to generate apis/user/delete_user.go: %v", err))
			}

			// 生成 apis/user/list_users.go
			apisUserListPath := filepath.Join(projectRoot, "apis", "user", "list_users.go")
			if err := generateFile(apisUserListPath, initx.ApisUserListUsersTemplate, templateData); err != nil {
				panic(fmt.Errorf("failed to generate apis/user/list_users.go: %v", err))
			}

			// 执行 go mod tidy
			fmt.Printf("📦 Running go mod tidy...\n")
			modTidyCmd := exec.Command("go", "mod", "tidy")
			modTidyCmd.Dir = projectRoot
			modTidyCmd.Stdout = os.Stdout
			modTidyCmd.Stderr = os.Stderr
			if err := modTidyCmd.Run(); err != nil {
				panic(fmt.Errorf("failed to run go mod tidy: %v", err))
			}

			// 生成错误码
			fmt.Printf("🔧 Generating error codes...\n")
			statusErrorDir := filepath.Join(projectRoot, "constants", "status_error")
			genErrorCmd := exec.Command("toolx", "gen", "error", "-p", "error_codes", "-c", "StatusError")
			genErrorCmd.Dir = statusErrorDir
			genErrorCmd.Stdout = os.Stdout
			genErrorCmd.Stderr = os.Stderr
			if err := genErrorCmd.Run(); err != nil {
				fmt.Printf("Warning: failed to generate error codes: %v\n", err)
				fmt.Println("You can manually run: cd constants/status_error && toolx gen error -p error_codes -c StatusError")
			}

			// 打印项目结构
			fmt.Printf("\n✨ Successfully initialized project '%s'!\n\n", projectName)
			fmt.Printf("📁 Project structure:\n")
			fmt.Printf("📁 %s/\n", projectName)
			printTree(projectRoot, projectRoot, "", true)

			// 打印启动提示
			fmt.Printf("\n🎉 Project initialization completed!\n\n")
			fmt.Printf("📝 To start the server, run:\n")
			fmt.Printf("   cd %s/cmd/%s && go build && ./%s -f local-config.yaml\n", projectName, projectName, projectName)
			fmt.Printf("📝 To debug api, run:\n")
			fmt.Printf("   cd %s/cmd/%s && toolx swagger -s \"http://127.0.0.1:8321\"\n", projectName, projectName)
		},
	}

	return cmd
}

func generateFile(filePath string, template string, data map[string]interface{}) error {
	// 检查文件是否已存在
	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("file already exists: %s", filePath)
	}

	// 解析模板并生成内容
	buff, err := utils.ParseTemplate(filepath.Base(filePath), template, data)
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 写入文件
	return os.WriteFile(filePath, buff.Bytes(), 0644)
}

// printTree 递归打印目录树结构
func printTree(rootPath, basePath, prefix string, isLast bool) {
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return
	}

	// 过滤掉隐藏文件和目录，并排序
	var filteredEntries []os.DirEntry
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	// 排序：目录在前，文件在后，然后按名称排序
	sort.Slice(filteredEntries, func(i, j int) bool {
		if filteredEntries[i].IsDir() != filteredEntries[j].IsDir() {
			return filteredEntries[i].IsDir()
		}
		return filteredEntries[i].Name() < filteredEntries[j].Name()
	})

	for i, entry := range filteredEntries {
		isLastEntry := i == len(filteredEntries)-1
		currentPrefix := prefix
		if isLast {
			currentPrefix += "    "
		} else {
			currentPrefix += "│   "
		}

		// 显示文件名
		displayPath := entry.Name()

		if entry.IsDir() {
			connector := "├── "
			if isLastEntry {
				connector = "└── "
			}
			fmt.Printf("%s%s📁 %s/\n", prefix, connector, displayPath)
			subPath := filepath.Join(rootPath, entry.Name())
			printTree(subPath, basePath, currentPrefix, isLastEntry)
		} else {
			connector := "├── "
			if isLastEntry {
				connector = "└── "
			}
			// 根据文件扩展名选择不同的 emoji
			emoji := "📄"
			ext := filepath.Ext(entry.Name())
			switch ext {
			case ".go":
				emoji = "🔷"
			case ".yaml", ".yml":
				emoji = "⚙️"
			case ".json":
				emoji = "📋"
			case ".md":
				emoji = "📝"
			case ".mod", ".sum":
				emoji = "📦"
			}
			fmt.Printf("%s%s%s %s\n", prefix, connector, emoji, displayPath)
		}
	}
}
