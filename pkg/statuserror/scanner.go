package statuserror

import (
	"fmt"
	"go/ast"
	"go/types"
	"golang.org/x/text/language"
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

		messages := ParseMessage(ident.Obj.Decl.(*ast.ValueSpec).Doc.Text())

		scanner.addStatusError(typeName, key, code, messages)
	}

	return sortedStatusErrList(scanner.StatusErrors[typeName])
}

func ParseMessage(s string) map[string]string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	var messages = make(map[string]string)
	for _, line := range lines {
		prefix := "@err"
		if strings.HasPrefix(line, prefix) {
			fields := strings.SplitN(line, " ", 2)
			key := fields[0]
			lang := key[4:]
			t, err := language.Parse(lang)
			if err != nil {
				continue
			}
			messages[t.String()] = fields[1]
		}

	}

	return messages
}

func (scanner *StatusErrorScanner) addStatusError(
	typeName *types.TypeName,
	key string, code int64, messages map[string]string,
) {
	if scanner.StatusErrors == nil {
		scanner.StatusErrors = map[*types.TypeName][]*StatusErr{}
	}

	statusErr := &StatusErr{
		K:         key,
		ErrorCode: code,
		Messages:  messages,
	}

	scanner.StatusErrors[typeName] = append(scanner.StatusErrors[typeName], statusErr)
}
