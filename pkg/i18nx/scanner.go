package i18nx

import (
	"fmt"
	"go/ast"
	"go/types"
	"golang.org/x/text/language"
	"sort"
	"strings"

	"github.com/go-courier/packagesx"
)

func NewI18nScanner(pkg *packagesx.Package) *I18nScanner {
	return &I18nScanner{
		pkg: pkg,
	}
}

type I18nScanner struct {
	pkg      *packagesx.Package
	Messages map[*types.TypeName][]*Message
}

func sortedMessageList(list []*Message) []*Message {
	sort.Slice(list, func(i, j int) bool {
		return list[i].K < list[j].K
	})
	return list
}

func (scanner *I18nScanner) LoadMessages(typeName *types.TypeName) []*Message {
	if typeName == nil {
		return nil
	}

	if statusErrs, ok := scanner.Messages[typeName]; ok {
		return sortedMessageList(statusErrs)
	}

	if !strings.Contains(typeName.Type().Underlying().String(), "string") {
		panic(fmt.Errorf("status error type underlying must be string, but got %s", typeName.String()))
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

		messages := ParseMessage(ident.Obj.Decl.(*ast.ValueSpec).Doc.Text())

		scanner.addMessage(typeName, typeConst.Name(), typeConst.Val().String(), messages)
	}

	return sortedMessageList(scanner.Messages[typeName])
}

func ParseMessage(s string) map[string]string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	var messages = make(map[string]string)
	for _, line := range lines {
		prefix := "@i18n"
		if strings.HasPrefix(line, prefix) {
			fields := strings.SplitN(line, " ", 2)
			key := fields[0]
			lang := key[5:]
			t, err := language.Parse(lang)
			if err != nil {
				continue
			}
			messages[t.String()] = fields[1]
		}

	}

	return messages
}

func (scanner *I18nScanner) addMessage(
	typeName *types.TypeName,
	name, key string, messages map[string]string,
) {
	if scanner.Messages == nil {
		scanner.Messages = map[*types.TypeName][]*Message{}
	}

	statusErr := &Message{
		T:     name,
		K:     key,
		Langs: messages,
	}

	scanner.Messages[typeName] = append(scanner.Messages[typeName], statusErr)
}
