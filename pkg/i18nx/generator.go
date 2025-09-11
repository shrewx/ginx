package i18nx

import (
	"fmt"
	"github.com/go-courier/packagesx"
	"github.com/shrewx/ginx/pkg/utils"
	"go/types"
	"golang.org/x/tools/go/packages"
	"os"
	"path/filepath"
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
