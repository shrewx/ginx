package openapi

import (
	"bytes"
	"context"
	"go/ast"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"github.com/go-courier/packagesx"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/tools/go/packages"
)

type RouterScanner struct {
	pkg             *packagesx.Package
	routers         map[*types.Var]*Router
	operatorScanner *OperatorScanner
}

func NewRouterScanner(pkg *packagesx.Package) *RouterScanner {
	routerScanner := &RouterScanner{
		pkg:             pkg,
		routers:         map[*types.Var]*Router{},
		operatorScanner: NewOperatorScanner(pkg),
	}

	routerScanner.init()

	return routerScanner
}

func (scanner *RouterScanner) init() {

	for _, pkg := range scanner.pkg.AllPackages {
		for ident, obj := range pkg.TypesInfo.Defs {
			if typeVar, ok := obj.(*types.Var); ok {
				if typeVar != nil && !strings.HasSuffix(typeVar.Pkg().Path(), pkgImportGinx) &&
					isGinRouterType(typeVar.Type()) {
					router := NewRouter(typeVar)
					ast.Inspect(ident.Obj.Decl.(ast.Node), func(node ast.Node) bool {
						switch callExpr := node.(type) {
						case *ast.CallExpr:
							operators := scanner.OperatorTypeNamesFromArgs(packagesx.NewPackage(pkg), callExpr.Args...)
							router.AppendOperators(operators...)
							scanner.routers[typeVar] = router
							return false
						}
						return true
					})
				}
			}
		}
	}

	// 处理直接的 Router.Register() 调用
	for _, pkg := range scanner.pkg.AllPackages {
		for selectExpr, selection := range pkg.TypesInfo.Selections {
			if selection.Obj() != nil {
				if typeFunc, ok := selection.Obj().(*types.Func); ok {
					recv := typeFunc.Type().(*types.Signature).Recv()
					if recv != nil && isGinRouterType(recv.Type()) {
						for typeVar, router := range scanner.routers {
							switch selectExpr.Sel.Name {
							case "Register":
								if typeVar == pkg.TypesInfo.ObjectOf(packagesx.GetIdentChainOfCallFunc(selectExpr)[0]) {
									file := scanner.pkg.FileOf(selectExpr)
									ast.Inspect(file, func(node ast.Node) bool {
										scanner.processRegisterCall(pkg, node, selectExpr, router)
										return true
									})
								}
							}
						}
					}
				}
			}
		}
	}

	// 处理 init() 函数中通过包装函数注册的情况
	// 例如: func init() { RegisterRouter(&AddHttpDialRoute{}) }
	// 其中 RegisterRouter 函数内部调用 Router.Register(operator)
	for _, pkg := range scanner.pkg.AllPackages {
		for _, obj := range pkg.TypesInfo.Defs {
			if typeFunc, ok := obj.(*types.Func); ok {
				if typeFunc.Name() == "init" {
					funcDecl := scanner.pkg.FuncDeclOf(typeFunc)
					if funcDecl != nil {
						ast.Inspect(funcDecl, func(node ast.Node) bool {
							if callExpr, ok := node.(*ast.CallExpr); ok {
								// 检查是否是函数调用（不是方法调用）
								if ident, ok := callExpr.Fun.(*ast.Ident); ok {
									if callObj := pkg.TypesInfo.ObjectOf(ident); callObj != nil {
										if wrapperFunc, ok := callObj.(*types.Func); ok {
											// 检查函数签名：只有一个参数，且参数类型是 ginx.Operator
											sig := wrapperFunc.Type().(*types.Signature)
											if sig.Params().Len() == 1 {
												paramType := sig.Params().At(0).Type()
												if isGinxOperatorType(paramType) {
													// 找到包装函数的定义
													wrapperPkg := packagesx.NewPackage(scanner.pkg.Pkg(wrapperFunc.Pkg().Path()))
													wrapperFuncDecl := wrapperPkg.FuncDeclOf(wrapperFunc)
													if wrapperFuncDecl != nil {
														// 从 init() 调用中获取 operator 参数
														if len(callExpr.Args) > 0 {
															operatorArg := callExpr.Args[0]
															operators := scanner.OperatorTypeNamesFromArgs(packagesx.NewPackage(pkg), operatorArg)
															if len(operators) > 0 {
																// 在包装函数内部查找 Router.Register() 调用
																scanner.processRegisterCallInWrapper(wrapperPkg, wrapperFuncDecl, operators)
															}
														}
													}
												}
											}
										}
									}
								}
							}
							return true
						})
					}
				}
			}
		}
	}
}

// processRegisterCall 处理 Router.Register() 调用
func (scanner *RouterScanner) processRegisterCall(pkg *packages.Package, node ast.Node, selectExpr *ast.SelectorExpr, router *Router) {
	switch node.(type) {
	case *ast.CallExpr:
		callExpr := node.(*ast.CallExpr)
		if callExpr.Fun == selectExpr {
			routerIdent := callExpr.Args[0]
			switch v := routerIdent.(type) {
			case *ast.SelectorExpr:
				argTypeVar := pkg.TypesInfo.ObjectOf(v.Sel).(*types.Var)
				if r, ok := scanner.routers[argTypeVar]; ok {
					router.Register(r)
				}
			}
		}
	case *ast.ExprStmt:
		exprStmt := node.(*ast.ExprStmt)
		if callExpr, ok := exprStmt.X.(*ast.CallExpr); ok {
			if callExpr.Fun == selectExpr {
				// parse Router.Register() argTypeVar ==> Router
				argTypeVar := pkg.TypesInfo.ObjectOf(callExpr.Fun.(*ast.SelectorExpr).X.(*ast.Ident)).(*types.Var)
				if r, ok := scanner.routers[argTypeVar]; ok {
					switch a := callExpr.Args[0].(type) {
					case *ast.UnaryExpr:
						// for middleware
						if scanner.checkMiddleware(pkg, a) {
							r.AppendOperators(scanner.OperatorTypeNamesFromArgs(packagesx.NewPackage(pkg), callExpr.Args...)...)
						} else {
							r.With(scanner.OperatorTypeNamesFromArgs(packagesx.NewPackage(pkg), callExpr.Args...)...)
						}
					}
				}
			}
		}
	}
}

// processRegisterCallInWrapper 在包装函数内部查找并处理 Router.Register() 调用
func (scanner *RouterScanner) processRegisterCallInWrapper(wrapperPkg *packagesx.Package, wrapperFuncDecl *ast.FuncDecl, operators []*OperatorWithTypeName) {
	ast.Inspect(wrapperFuncDecl, func(n ast.Node) bool {
		if exprStmt, ok := n.(*ast.ExprStmt); ok {
			if innerCallExpr, ok := exprStmt.X.(*ast.CallExpr); ok {
				if selectorExpr, ok := innerCallExpr.Fun.(*ast.SelectorExpr); ok {
					if selectorExpr.Sel.Name == "Register" {
						// 找到 Router.Register() 调用
						if routerIdent, ok := selectorExpr.X.(*ast.Ident); ok {
							if routerObj := wrapperPkg.TypesInfo.ObjectOf(routerIdent); routerObj != nil {
								if routerVar, ok := routerObj.(*types.Var); ok {
									if router, ok := scanner.routers[routerVar]; ok {
										// 将 operator 注册到 router
										router.With(operators...)
									}
								}
							}
						}
					}
				}
			}
		}
		return true
	})
}

func (scanner *RouterScanner) checkMiddleware(pkg *packages.Package, expr *ast.UnaryExpr) bool {
	if c, ok := expr.X.(*ast.CompositeLit); ok {
		if s, ok := c.Type.(*ast.SelectorExpr); ok {
			typ := pkg.TypesInfo.TypeOf(s.Sel)
			var typeName *types.TypeName
			switch t := typ.(type) {
			case *types.Pointer:
				typeName = t.Elem().(*types.Named).Obj()
			case *types.Named:
				typeName = t.Obj()
			default:
				return false
			}
			if _, ok := scanner.operatorScanner.singleReturnOf(typeName, "Type"); ok {
				return true
			}
		}
	}

	return false
}

func (scanner *RouterScanner) Router(typeName *types.Var) *Router {
	return scanner.routers[typeName]
}

type OperatorWithTypeName struct {
	*Operator
	TypeName *types.TypeName
}

func (operator *OperatorWithTypeName) String() string {
	return operator.TypeName.Pkg().Name() + "." + operator.TypeName.Name()
}

func (scanner *RouterScanner) OperatorTypeNamesFromArgs(pkg *packagesx.Package, args ...ast.Expr) []*OperatorWithTypeName {
	opTypeNames := make([]*OperatorWithTypeName, 0)

	for _, arg := range args {
		opTypeName := scanner.OperatorTypeNameFromType(pkg.TypesInfo.TypeOf(arg))

		if opTypeName == nil {
			continue
		}

		if callExpr, ok := arg.(*ast.CallExpr); ok {
			if selectorExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				if containsGinx(pkg.TypesInfo.ObjectOf(selectorExpr.Sel).Type()) {
					switch selectorExpr.Sel.Name {
					case "Group":
						switch v := callExpr.Args[0].(type) {
						case *ast.BasicLit:
							opTypeName.Path, _ = strconv.Unquote(v.Value)
						}
					}
				}
			}
		}

		opTypeNames = append(opTypeNames, opTypeName)
	}

	return opTypeNames
}

func (scanner *RouterScanner) OperatorTypeNameFromType(typ types.Type) *OperatorWithTypeName {
	switch t := typ.(type) {
	case *types.Pointer:
		return scanner.OperatorTypeNameFromType(t.Elem())
	case *types.Named:
		typeName := t.Obj()

		if operator := scanner.operatorScanner.Operator(context.Background(), typeName); operator != nil {
			return &OperatorWithTypeName{
				Operator: operator,
				TypeName: typeName,
			}
		}

		return nil
	default:
		return nil
	}
}

func NewRouter(typeVar *types.Var, operators ...*OperatorWithTypeName) *Router {
	return &Router{
		typeVar:   typeVar,
		operators: operators,
	}
}

func (r *Router) Name() string {
	if r.typeVar == nil {
		return "Anonymous"
	}
	return r.typeVar.Pkg().Name() + "." + r.typeVar.Name()
}

func (r *Router) String() string {
	buf := bytes.NewBufferString(r.Name())

	buf.WriteString("<")
	for i := range r.operators {
		o := r.operators[i]
		if i != 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(o.String())
	}
	buf.WriteString(">")

	buf.WriteString("[")

	i := 0
	for sub := range r.children {
		if i != 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(sub.Name())
		i++
	}
	buf.WriteString("]")

	return buf.String()
}

type Router struct {
	typeVar   *types.Var
	parent    *Router
	operators []*OperatorWithTypeName
	children  map[*Router]bool
}

func (router *Router) AppendOperators(operators ...*OperatorWithTypeName) {
	router.operators = append(router.operators, operators...)
}

func (router *Router) With(operators ...*OperatorWithTypeName) {
	router.Register(NewRouter(nil, operators...))
}

func (router *Router) Register(r *Router) {
	if router.children == nil {
		router.children = map[*Router]bool{}
	}
	r.parent = router
	router.children[r] = true
}

func (router *Router) Route() *Route {
	parent := router.parent
	operators := router.operators

	for parent != nil {
		operators = append(parent.operators, operators...)
		parent = parent.parent
	}

	route := Route{
		last:      router.children == nil,
		Operators: operators,
	}

	return &route
}

func (router *Router) Routes() (routes []*Route) {
	for child := range router.children {
		route := child.Route()

		if route.last {
			routes = append(routes, route)
		}

		if child.children != nil {
			routes = append(routes, child.Routes()...)
		}
	}

	sort.Slice(routes, func(i, j int) bool {
		return routes[i].String() < routes[j].String()
	})

	return routes
}

type Route struct {
	Operators []*OperatorWithTypeName
	last      bool
}

func (route *Route) String() string {
	buf := bytes.NewBufferString(route.Method())
	buf.WriteString(" ")
	buf.WriteString(route.Path())

	for i := range route.Operators {
		buf.WriteString(" ")
		buf.WriteString(route.Operators[i].String())
	}

	return buf.String()
}

func (route *Route) Method() string {
	method := ""
	for _, m := range route.Operators {
		if m.Method != "" {
			method = m.Method
		}
	}
	return method
}

func (route *Route) Path() string {
	basePath := ""

	for _, operator := range route.Operators {
		if operator.Path != "" {
			basePath += httprouter.CleanPath(operator.Path)
		}
	}

	return httprouter.CleanPath(basePath)
}
