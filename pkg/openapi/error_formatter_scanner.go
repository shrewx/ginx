package openapi

import (
	"go/ast"
	"go/types"
	"reflect"
	"strings"

	"github.com/go-courier/packagesx"
	"github.com/sirupsen/logrus"
)

// ErrorFormatterScanner 扫描代码中注册的 ResponseFormatter
// 自动识别通过 RegisterErrorFormatter 注册的错误类型
type ErrorFormatterScanner struct {
	pkg        *packagesx.Package
	errorTypes []types.Type // 识别到的包含 error tag 的错误类型
}

// NewErrorFormatterScanner 创建错误格式化器扫描器
func NewErrorFormatterScanner(pkg *packagesx.Package) *ErrorFormatterScanner {
	return &ErrorFormatterScanner{
		pkg:        pkg,
		errorTypes: make([]types.Type, 0),
	}
}

// Scan 扫描所有包，查找 RegisterErrorFormatter 调用
// 返回所有识别到的包含 error tag 的错误类型
func (s *ErrorFormatterScanner) Scan() []types.Type {
	logrus.Debug("scanning RegisterErrorFormatter calls...")

	// 遍历所有包
	for _, pkgInfo := range s.pkg.AllPackages {
		pkg := packagesx.NewPackage(pkgInfo)

		// 遍历所有文件
		for _, file := range pkgInfo.Syntax {
			ast.Inspect(file, func(node ast.Node) bool {
				// 查找函数调用
				if callExpr, ok := node.(*ast.CallExpr); ok {
					s.scanRegisterCall(pkg, callExpr)
				}
				return true
			})
		}
	}

	logrus.Debugf("found %d error types with 'error' tag from RegisterErrorFormatter", len(s.errorTypes))
	return s.errorTypes
}

// scanRegisterCall 扫描 RegisterErrorFormatter 调用
func (s *ErrorFormatterScanner) scanRegisterCall(pkg *packagesx.Package, callExpr *ast.CallExpr) {
	// 检查是否是 RegisterErrorFormatter 调用
	if !s.isRegisterErrorFormatterCall(pkg, callExpr) {
		return
	}

	// 获取参数
	if len(callExpr.Args) == 0 {
		return
	}

	arg := callExpr.Args[0]

	// 获取参数类型
	tv, err := pkg.Eval(arg)
	if err != nil {
		return
	}

	// 提取格式化器类型（去掉指针）
	formatterType := tv.Type
	if ptr, ok := formatterType.(*types.Pointer); ok {
		formatterType = ptr.Elem()
	}

	logrus.Debugf("found RegisterErrorFormatter call with type: %s", formatterType.String())

	// 直接检查这个类型是否是包含 error tag 的结构体
	// 因为 ResponseFormatter 本身就是错误结构体
	s.addErrorTypeIfHasTag(formatterType)
}

// isRegisterErrorFormatterCall 判断是否是 RegisterErrorFormatter 调用
func (s *ErrorFormatterScanner) isRegisterErrorFormatterCall(pkg *packagesx.Package, callExpr *ast.CallExpr) bool {
	// 获取函数名
	var funcName string
	var pkgPath string

	switch fun := callExpr.Fun.(type) {
	case *ast.Ident:
		funcName = fun.Name
	case *ast.SelectorExpr:
		funcName = fun.Sel.Name
		// 获取包路径
		if ident, ok := fun.X.(*ast.Ident); ok {
			if obj := pkg.TypesInfo.ObjectOf(ident); obj != nil {
				if pkgName, ok := obj.(*types.PkgName); ok {
					pkgPath = pkgName.Imported().Path()
				}
			}
		}
	default:
		return false
	}

	// 检查函数名
	if funcName != "RegisterErrorFormatter" {
		return false
	}

	// 检查包路径（可以是 ginx 包或当前包）
	if pkgPath != "" && !strings.Contains(pkgPath, "ginx") {
		return false
	}

	return true
}

// addErrorTypeIfHasTag 添加错误类型（仅当包含 error tag 时）
func (s *ErrorFormatterScanner) addErrorTypeIfHasTag(errorType types.Type) {
	if errorType == nil {
		return
	}

	// 去掉指针
	if ptr, ok := errorType.(*types.Pointer); ok {
		errorType = ptr.Elem()
	}

	// 必须是命名类型
	named, ok := errorType.(*types.Named)
	if !ok {
		return
	}

	// 必须是结构体
	structType, ok := named.Underlying().(*types.Struct)
	if !ok {
		return
	}

	// 检查是否有 error tag（关键过滤条件）
	if !s.hasErrorTag(structType) {
		logrus.Debugf("skipping type %s: no 'error' tag found", named.Obj().Name())
		return
	}

	// 避免重复添加
	typeStr := errorType.String()
	for _, existing := range s.errorTypes {
		if existing.String() == typeStr {
			return
		}
	}

	s.errorTypes = append(s.errorTypes, errorType)
}

// hasErrorTag 检查结构体是否有 error tag
// 这是关键的过滤条件：只有包含 error tag 的结构体才会被识别
func (s *ErrorFormatterScanner) hasErrorTag(structType *types.Struct) bool {
	for i := 0; i < structType.NumFields(); i++ {
		tag := reflect.StructTag(structType.Tag(i))
		if tag.Get("error") != "" {
			return true
		}
	}
	return false
}

// GetPrimaryErrorType 获取主要的错误类型（第一个注册的）
func (s *ErrorFormatterScanner) GetPrimaryErrorType() types.Type {
	if len(s.errorTypes) > 0 {
		return s.errorTypes[0]
	}
	return nil
}
