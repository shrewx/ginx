package i18nx

import (
	"fmt"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-courier/packagesx"
	"github.com/shrewx/ginx/pkg/utils"
	"golang.org/x/tools/go/packages"
	"gopkg.in/yaml.v2"
)

type IMessage struct {
	TypeName *types.TypeName
	Messages []*Message
}

type I18nMessageGenerator struct {
	pkg      *packagesx.Package
	scanner  *I18nScanner
	messages map[string]*IMessage
}

func NewI18nGenerator(pkg *packagesx.Package) *I18nMessageGenerator {
	return &I18nMessageGenerator{
		pkg:      pkg,
		scanner:  NewI18nScanner(pkg),
		messages: map[string]*IMessage{},
	}
}

func (g *I18nMessageGenerator) Scan(names ...string) {
	for _, name := range names {
		typeName := g.pkg.TypeName(name)
		g.messages[name] = &IMessage{
			TypeName: typeName,
			Messages: g.scanner.LoadMessages(typeName),
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

func (g *I18nMessageGenerator) Output(pwd, prefix string) {
	for name, i18n := range g.messages {
		pkgDir, packageName := getPkgDirAndPackage(i18n.TypeName.Pkg().Path())
		dir, _ := filepath.Rel(pwd, pkgDir)
		filename := utils.Camel2Case(name) + "__generated.go"

		var messages = make(map[string][]*Message)
		for _, m := range i18n.Messages {
			for k, v := range m.Langs {
				messages[k] = append(messages[k], &Message{
					T:       m.T,
					K:       m.Key(),
					Message: v,
				})
			}
		}

		buff, err := utils.ParseTemplate("i18n", I18nTemplate, map[string]interface{}{
			"Package":   packageName,
			"ClassName": name,
			"Keys":      i18n.Messages,
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

func (g *I18nMessageGenerator) OutputYAML(pwd, prefix, outputDir string) {
	// 确保输出目录存在
	if outputDir != "" {
		err := os.MkdirAll(outputDir, os.ModePerm)
		if err != nil {
			panic(fmt.Errorf("failed to create output directory %s: %v", outputDir, err))
		}
	}

	for name, i18n := range g.messages {

		// 按语言分组生成YAML文件
		languageMessages := make(map[string]map[string]string)

		for _, m := range i18n.Messages {
			for lang, message := range m.Langs {
				if languageMessages[lang] == nil {
					languageMessages[lang] = make(map[string]string)
				}
				// 使用消息的ID作为key
				messageID := strings.Trim(m.Key(), "\"")
				if m.Prefix() != "" {
					messageID = m.Prefix() + "." + strings.Trim(m.Key(), "\"")
				}
				languageMessages[lang][messageID] = message
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
