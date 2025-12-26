package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func InstallSkill() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "install ginx-skill using openskills",
		Long: `Install ginx-skill using openskills.

This command will:
1. Locate the ginx module directory (via go get github.com/shrewx/ginx)
2. Find the ginx-skill directory (ai/ginx-skill)
3. Run openskills install in the current working directory

The skill will be installed to .claude/skills/ginx-skill in your current project directory.

Users can get ginx-skill by running:
  go get github.com/shrewx/ginx

The command will automatically locate the ginx-skill directory from the installed module.`,
		Run: func(cmd *cobra.Command, args []string) {
			// 获取当前工作目录（用户执行命令的目录）
			cwd, err := os.Getwd()
			if err != nil {
				fmt.Printf("❌ Error: Failed to get current working directory: %v\n", err)
				os.Exit(1)
			}

			skillDir, err := findGinxSkillDir()
			if err != nil {
				fmt.Printf("❌ Error: Failed to locate ginx-skill directory: %v\n", err)
				fmt.Println("\n💡 Please ensure you have installed ginx module:")
				fmt.Println("   go get github.com/shrewx/ginx")
				os.Exit(1)
			}

			// 验证目录是否存在
			if _, err := os.Stat(skillDir); os.IsNotExist(err) {
				fmt.Printf("❌ Error: ginx-skill directory not found: %s\n", skillDir)
				fmt.Println("\n💡 Please ensure you have installed ginx module:")
				fmt.Println("   go get github.com/shrewx/ginx")
				os.Exit(1)
			}

			// 验证 SKILL.md 是否存在
			skillFile := filepath.Join(skillDir, "SKILL.md")
			if _, err := os.Stat(skillFile); os.IsNotExist(err) {
				fmt.Printf("❌ Error: SKILL.md not found in %s\n", skillDir)
				fmt.Println("\n💡 Please ensure ginx-skill is properly installed")
				os.Exit(1)
			}

			fmt.Printf("📦 Found ginx-skill at: %s\n", skillDir)
			fmt.Printf("📁 Installing to: %s/.claude/skills/ginx-skill\n", cwd)
			fmt.Println("🚀 Installing ginx-skill using openskills...")

			// 执行 openskills install，在当前工作目录下执行
			installCmd := exec.Command("openskills", "install", skillDir)
			installCmd.Stdin = os.Stdin  // 连接标准输入以支持交互
			installCmd.Stdout = os.Stdout
			installCmd.Stderr = os.Stderr
			installCmd.Dir = cwd // 在用户当前工作目录下执行，而不是在 skillDir

			err = installCmd.Run()
			if err != nil {
				// 检查是否是用户取消（通常退出码为 130 或 1）
				if exitError, ok := err.(*exec.ExitError); ok {
					exitCode := exitError.ExitCode()
					// 130 通常是 SIGINT (Ctrl+C)，1 可能是用户选择 N
					if exitCode == 130 {
						fmt.Println("\n⚠️  Installation cancelled by user")
						os.Exit(0)
					} else if exitCode == 1 {
						// 可能是用户选择不覆盖，或者其他错误
						// 让 openskills 的错误消息显示出来，不额外显示错误
						os.Exit(1)
					}
				}
				fmt.Printf("\n❌ Error: Failed to install ginx-skill: %v\n", err)
				fmt.Println("\n💡 Please ensure openskills is installed:")
				fmt.Println("   Check openskills installation: openskills --version")
				os.Exit(1)
			}

			fmt.Println("✅ Successfully installed ginx-skill!")
		},
	}

	return cmd
}

// findGinxSkillDir 查找 ginx-skill 目录
// 优先通过 go list 获取模块目录，如果失败则尝试通过运行时路径查找
func findGinxSkillDir() (string, error) {
	// 方法1: 通过 go list 获取模块目录（适用于通过 go get 安装的情况）
	goListCmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/shrewx/ginx")
	output, err := goListCmd.Output()
	if err == nil {
		moduleDir := string(output)
		// 移除末尾的换行符
		moduleDir = trimSpace(moduleDir)
		if moduleDir != "" {
			skillDir := filepath.Join(moduleDir, "ai", "ginx-skill")
			if _, err := os.Stat(skillDir); err == nil {
				return skillDir, nil
			}
		}
	}

	// 方法2: 通过执行文件路径查找（适用于开发环境或直接克隆的情况）
	// 获取当前执行文件的路径
	execPath, err := os.Executable()
	if err == nil {
		execDir := filepath.Dir(execPath)
		// 从执行文件路径向上查找项目根目录
		// 可能的路径: .../ginx/pkg/toolx/cmd/toolx 或 .../ginx/bin/toolx
		dir := execDir
		for i := 0; i < 10; i++ { // 最多向上查找10层
			skillDir := filepath.Join(dir, "ai", "ginx-skill")
			if _, err := os.Stat(skillDir); err == nil {
				return skillDir, nil
			}
			// 检查是否是项目根目录（有 go.mod 文件）
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				// 检查 go.mod 中是否包含 github.com/shrewx/ginx
				goModPath := filepath.Join(dir, "go.mod")
				if data, err := os.ReadFile(goModPath); err == nil {
					if contains(string(data), "github.com/shrewx/ginx") {
						skillDir := filepath.Join(dir, "ai", "ginx-skill")
						if _, err := os.Stat(skillDir); err == nil {
							return skillDir, nil
						}
					}
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// 方法2b: 通过源码文件路径查找（适用于开发环境）
	_, currentFile, _, ok := runtime.Caller(0)
	if ok {
		// 从 pkg/toolx/cmd/install_skill.go 向上找到项目根目录
		cmdDir := filepath.Dir(currentFile)
		toolxDir := filepath.Dir(cmdDir)
		pkgDir := filepath.Dir(toolxDir)
		projectRoot := filepath.Dir(pkgDir)
		skillDir := filepath.Join(projectRoot, "ai", "ginx-skill")
		if _, err := os.Stat(skillDir); err == nil {
			return skillDir, nil
		}
	}

	// 方法3: 尝试从当前工作目录查找
	cwd, err := os.Getwd()
	if err == nil {
		// 尝试在当前目录及其父目录中查找
		dir := cwd
		for i := 0; i < 10; i++ { // 最多向上查找10层
			skillDir := filepath.Join(dir, "ai", "ginx-skill")
			if _, err := os.Stat(skillDir); err == nil {
				return skillDir, nil
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	return "", fmt.Errorf("could not locate ginx-skill directory")
}

// trimSpace 移除字符串首尾的空白字符
func trimSpace(s string) string {
	return strings.TrimSpace(s)
}

// contains 检查字符串是否包含子字符串
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
