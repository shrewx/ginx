package statuserror

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-courier/packagesx"
	"github.com/shrewx/ginx/pkg/utils"
	"golang.org/x/tools/go/packages"
	"gopkg.in/yaml.v2"
)

type StatusError struct {
	TypeName *types.TypeName
	Errors   []*StatusErr
}

type StatusErrorGenerator struct {
	pkg          *packagesx.Package
	scanner      *StatusErrorScanner
	statusErrors map[string]*StatusError
}

func NewStatusErrorGenerator(pkg *packagesx.Package) *StatusErrorGenerator {
	return &StatusErrorGenerator{
		pkg:          pkg,
		scanner:      NewStatusErrorScanner(pkg),
		statusErrors: map[string]*StatusError{},
	}
}

func (g *StatusErrorGenerator) Scan(names ...string) {
	for _, name := range names {
		typeName := g.pkg.TypeName(name)
		g.statusErrors[name] = &StatusError{
			TypeName: typeName,
			Errors:   g.scanner.StatusError(typeName),
		}
	}

}

func getPkgDirAndPackage(importPath string) (string, string) {
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles,
	}, importPath)
	if err != nil {
		panic(err)
	}
	if len(pkgs) == 0 {
		panic(fmt.Errorf("package `%s` not found", importPath))
	}

	return filepath.Dir(pkgs[0].GoFiles[0]), pkgs[0].Name
}

// createNestedMap 将点分隔的 prefix 转换为嵌套的 map 结构
func createNestedMap(prefix string, messages map[string]string) map[string]interface{} {
	parts := strings.Split(prefix, ".")
	if len(parts) == 1 {
		// 如果只有一个部分，直接返回
		return map[string]interface{}{
			prefix: messages,
		}
	}

	// 从最深层开始构建嵌套结构
	result := map[string]interface{}{
		parts[len(parts)-1]: messages,
	}

	// 从后往前构建嵌套结构
	for i := len(parts) - 2; i >= 0; i-- {
		result = map[string]interface{}{
			parts[i]: result,
		}
	}

	return result
}

func (g *StatusErrorGenerator) Output(pwd, prefix string) {
	for name, statusErr := range g.statusErrors {
		pkgDir, packageName := getPkgDirAndPackage(statusErr.TypeName.Pkg().Path())
		dir, _ := filepath.Rel(pwd, pkgDir)
		filename := utils.Camel2Case(name) + "__generated.go"

		var messages = make(map[string][]*StatusErr)
		for _, e := range statusErr.Errors {
			for k, message := range e.Messages {
				messages[k] = append(messages[k], &StatusErr{
					K:       e.K,
					Message: message,
				})
			}
		}
		buff, err := utils.ParseTemplate("error", StatusErrorTemplate, map[string]interface{}{
			"Package":   packageName,
			"ClassName": name,
			"Errors":    statusErr.Errors,
			"Messages":  messages,
			"Prefix":    prefix,
		})
		if err != nil {
			panic(err)
		}
		err = os.WriteFile(filepath.Join(dir, filename), buff.Bytes(), os.ModePerm)
		if err != nil {
			panic(err)
		}
	}
}

func (g *StatusErrorGenerator) OutputYAML(pwd, prefix, outputDir string) {
	// 确保输出目录存在
	if outputDir != "" {
		err := os.MkdirAll(outputDir, os.ModePerm)
		if err != nil {
			panic(fmt.Errorf("failed to create output directory %s: %v", outputDir, err))
		}
	}

	for name, statusErr := range g.statusErrors {
		// 按语言分组生成YAML文件
		languageMessages := make(map[string]map[string]string)

		for _, e := range statusErr.Errors {
			for lang, message := range e.Messages {
				if languageMessages[lang] == nil {
					languageMessages[lang] = make(map[string]string)
				}
				// 使用错误代码作为key
				languageMessages[lang][strconv.FormatInt(e.ErrorCode, 10)] = message
			}
		}

		// 为每种语言生成单独的YAML文件
		for lang, messages := range languageMessages {
			// 将 prefix 转换为嵌套结构
			nestedPrefix := createNestedMap(prefix, messages)
			yamlData := map[string]interface{}{
				lang: nestedPrefix,
			}

			yamlContent, err := yaml.Marshal(yamlData)
			if err != nil {
				panic(fmt.Errorf("failed to marshal YAML for language %s: %v", lang, err))
			}

			// 生成文件名：{lang}_{prefix}.yaml
			filename := fmt.Sprintf("%s_%s.yaml", lang, utils.Camel2Case(name))
			filePath := filename
			if outputDir != "" {
				filePath = filepath.Join(outputDir, filename)
			}

			err = os.WriteFile(filePath, yamlContent, os.ModePerm)
			if err != nil {
				panic(fmt.Errorf("failed to write YAML file %s: %v", filePath, err))
			}
		}
	}
}
