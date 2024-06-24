package statuserror

import (
	"fmt"
	"github.com/go-courier/packagesx"
	"github.com/shrewx/stringx"
	"go/types"
	"golang.org/x/tools/go/packages"
	"io/ioutil"
	"os"
	"path/filepath"
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

func (g *StatusErrorGenerator) Output(pwd string) {
	for name, statusErr := range g.statusErrors {
		pkgDir, packageName := getPkgDirAndPackage(statusErr.TypeName.Pkg().Path())
		dir, _ := filepath.Rel(pwd, pkgDir)
		filename := stringx.Camel2Case(name) + "__generated.go"
		buff, err := stringx.ParseTextTemplate("error", StatusErrorTemplate, map[string]interface{}{
			"Package":   packageName,
			"ClassName": name,
			"Errors":    statusErr.Errors,
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
