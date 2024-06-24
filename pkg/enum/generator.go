package enum

import (
	"bytes"
	"fmt"
	"github.com/go-courier/packagesx"
	"go/types"
	"golang.org/x/tools/go/packages"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
)

type Enumeration struct {
	TypeName   *types.TypeName
	Enum       []Value
	StringType bool
}

type EnumerationGenerator struct {
	pkg     *packagesx.Package
	scanner *EnumScanner
	enums   map[string]*Enumeration
}

func NewEnumGenerator(pkg *packagesx.Package) *EnumerationGenerator {
	return &EnumerationGenerator{
		pkg:     pkg,
		scanner: NewEnumScanner(pkg),
		enums:   map[string]*Enumeration{},
	}
}

func (g *EnumerationGenerator) Scan(names ...string) {
	for _, name := range names {
		typeName := g.pkg.TypeName(name)
		g.enums[name] = &Enumeration{
			TypeName: typeName,
			Enum:     g.scanner.Enum(typeName),
		}
		if typeName != nil && strings.Contains(typeName.Type().Underlying().String(), "string") {
			g.enums[name].StringType = true
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

func (g *EnumerationGenerator) Output(pwd string) {
	for name, enum := range g.enums {
		if enum.TypeName == nil {
			continue
		}
		pkgDir, packageName := getPkgDirAndPackage(enum.TypeName.Pkg().Path())
		dir, _ := filepath.Rel(pwd, pkgDir)
		filename := camel2Case(name) + "__generated.go"

		var keys []string
		for _, e := range enum.Enum {
			keys = append(keys, e.Key)
		}

		var basicType = "int"
		if enum.StringType {
			basicType = "string"
		}

		buff, err := parseTextTemplate("enum", TplEnum, map[string]interface{}{
			"Package":   packageName,
			"ClassName": name,
			"Keypair":   enum.Enum,
			"Keys":      strings.Join(keys, ","),
			"Type":      fmt.Sprintf("%s.%s", enum.TypeName.Pkg().Path(), enum.TypeName.Name()),
			"BasicType": basicType,
		})
		if err != nil {
			panic(err)
		}

		err = ioutil.WriteFile(filepath.Join(dir, filename), buff.Bytes(), os.ModePerm)
		if err != nil {
			panic(err)
		}
	}
}

func camel2Case(name string) string {
	buffer := new(bytes.Buffer)
	for i, r := range name {
		if unicode.IsUpper(r) {
			if i != 0 {
				buffer.WriteByte('_')
			}
			buffer.WriteRune(unicode.ToLower(r))
		} else {
			buffer.WriteRune(r)
		}
	}
	return buffer.String()
}

// parseTextTemplate parse data into the template
func parseTextTemplate(tmplName, tmplConst string, data map[string]interface{}) (*bytes.Buffer, error) {
	tmp, err := template.New(tmplName).Parse(tmplConst)

	if err != nil {
		return nil, err
	}
	buff := new(bytes.Buffer)
	err = tmp.Execute(buff, data)
	if err != nil {
		return nil, err
	}

	return buff, nil
}
