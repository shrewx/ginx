package statuserror

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"github.com/go-courier/packagesx"
)

func NewStatusErrorScanner(pkg *packagesx.Package) *StatusErrorScanner {
	return &StatusErrorScanner{
		pkg: pkg,
	}
}

type StatusErrorScanner struct {
	pkg          *packagesx.Package
	StatusErrors map[*types.TypeName][]*StatusErr
}

func sortedStatusErrList(list []*StatusErr) []*StatusErr {
	sort.Slice(list, func(i, j int) bool {
		return list[i].Code() < list[j].Code()
	})
	return list
}

func (scanner *StatusErrorScanner) StatusError(typeName *types.TypeName) []*StatusErr {
	if typeName == nil {
		return nil
	}

	if statusErrs, ok := scanner.StatusErrors[typeName]; ok {
		return sortedStatusErrList(statusErrs)
	}

	if !strings.Contains(typeName.Type().Underlying().String(), "int") {
		panic(fmt.Errorf("status error type underlying must be an int or uint, but got %s", typeName.String()))
	}

	pkgInfo := scanner.pkg.Pkg(typeName.Pkg().Path())
	if pkgInfo == nil {
		return nil
	}

	for ident, def := range pkgInfo.TypesInfo.Defs {
		typeConst, ok := def.(*types.Const)
		if !ok {
			continue
		}
		if typeConst.Type() != typeName.Type() {
			continue
		}

		key := typeConst.Name()
		code, _ := strconv.ParseInt(typeConst.Val().String(), 10, 64)

		zh, en := ParseMessage(ident.Obj.Decl.(*ast.ValueSpec).Doc.Text())

		scanner.addStatusError(typeName, key, int(code), zh, en)
	}

	return sortedStatusErrList(scanner.StatusErrors[typeName])
}

func ParseMessage(s string) (zhMsg string, enMsg string) {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for _, line := range lines {
		prefix := "@errZH "
		if strings.HasPrefix(line, prefix) {
			zhMsg = line[len(prefix):]
		}

		prefix = "@errEN "
		if strings.HasPrefix(line, prefix) {
			enMsg = line[len(prefix):]
		}
	}

	return
}

func (scanner *StatusErrorScanner) addStatusError(
	typeName *types.TypeName,
	key string, code int, zhMessage, enMessage string,
) {
	if scanner.StatusErrors == nil {
		scanner.StatusErrors = map[*types.TypeName][]*StatusErr{}
	}

	statusErr := &StatusErr{
		Key:       key,
		ErrorCode: code,
		ZHMessage: zhMessage,
		ENMessage: enMessage,
	}

	scanner.StatusErrors[typeName] = append(scanner.StatusErrors[typeName], statusErr)
}
