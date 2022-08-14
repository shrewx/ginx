package openapi

import (
	"context"
	"encoding/json"
	"github.com/fatih/color"
	"github.com/go-courier/oas"
	"github.com/go-courier/packagesx"
	"github.com/pkg/errors"
	"go/ast"
	"go/types"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type OpenAPIGenerator struct {
	pkg           *packagesx.Package
	openapi       *oas.OpenAPI
	routerScanner *RouterScanner
}

func NewOpenAPIGenerator(pkg *packagesx.Package) *OpenAPIGenerator {
	return &OpenAPIGenerator{
		pkg:           pkg,
		openapi:       oas.NewOpenAPI(),
		routerScanner: NewRouterScanner(pkg),
	}
}
func (g *OpenAPIGenerator) SetServer(url string) {
	g.openapi.Servers = append(g.openapi.Servers, oas.NewServer(url))
}

func (g *OpenAPIGenerator) Scan(ctx context.Context) {
	defer func() {
		g.routerScanner.operatorScanner.BindSchemas(g.openapi)
		g.routerScanner.operatorScanner.BindSecuritySchemas(g.openapi)
	}()

	for ident, def := range g.pkg.TypesInfo.Defs {
		if typFunc, ok := def.(*types.Func); ok {
			if typFunc.Name() != "main" {
				continue
			}

			ast.Inspect(ident.Obj.Decl.(*ast.FuncDecl), func(node ast.Node) bool {
				switch n := node.(type) {
				case *ast.CallExpr:
					if rootRouterVar := rootRouter(g.pkg, n); rootRouterVar != nil {
						router := g.routerScanner.Router(rootRouterVar)

						routes := router.Routes()

						operationIDs := map[string]*Route{}

						for _, route := range routes {
							method := route.Method()

							operation := g.OperationByOperatorTypes(method, route.Operators...)
							if operation.OperationId == "" {
								continue
							}
							if _, exists := operationIDs[operation.OperationId]; exists && operation.OperationId != "" {
								panic(errors.Errorf("operationID %s should be unique", operation.OperationId))
							}

							operationIDs[operation.OperationId] = route

							g.openapi.AddOperation(oas.HttpMethod(strings.ToLower(method)), g.patchPath(route.Path(), operation), operation)
						}
					}
				}
				return true
			})
			return
		}
	}

}

func rootRouter(pkgInfo *packagesx.Package, callExpr *ast.CallExpr) *types.Var {
	if len(callExpr.Args) > 0 {
		if selectorExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
			if typesFunc, ok := pkgInfo.TypesInfo.ObjectOf(selectorExpr.Sel).(*types.Func); ok {
				if signature, ok := typesFunc.Type().(*types.Signature); ok {
					if isGinRouterType(signature.Params().At(0).Type()) {
						if selectorExpr.Sel.Name == "Start" {
							if len(callExpr.Args) == 2 {
								switch node := callExpr.Args[1].(type) {
								case *ast.SelectorExpr:
									return pkgInfo.TypesInfo.ObjectOf(node.Sel).(*types.Var)
								case *ast.Ident:
									return pkgInfo.TypesInfo.ObjectOf(node).(*types.Var)
								}
							}
						}
					}
				}
			}
		}
	}
	return nil
}

var reHttpRouterPath = regexp.MustCompile("/:([^/]+)")

func (g *OpenAPIGenerator) patchPath(openapiPath string, operation *oas.Operation) string {
	return reHttpRouterPath.ReplaceAllStringFunc(openapiPath, func(str string) string {
		name := reHttpRouterPath.FindAllStringSubmatch(str, -1)[0][1]

		var isParameterDefined = false

		for _, parameter := range operation.Parameters {
			if parameter.In == "path" && parameter.Name == name {
				isParameterDefined = true
			}
		}

		if isParameterDefined {
			return "/{" + name + "}"
		}

		return "/0"
	})
}

func (g *OpenAPIGenerator) OperationByOperatorTypes(method string, operatorTypes ...*OperatorWithTypeName) *oas.Operation {
	operation := &oas.Operation{}

	length := len(operatorTypes)

	for idx := range operatorTypes {
		if operatorTypes[idx].ID == "GinGroup" {
			continue
		}
		operatorTypes[idx].BindOperation(method, operation, idx == length-1)
	}

	return operation
}

func (g *OpenAPIGenerator) Output(cwd string) {
	file := filepath.Join(cwd, "openapi.json")
	data, err := json.MarshalIndent(g.openapi, "", "  ")
	if err != nil {
		return
	}
	_ = ioutil.WriteFile(file, data, os.ModePerm)
	log.Printf("generated openapi spec into %s", color.MagentaString(file))
}
