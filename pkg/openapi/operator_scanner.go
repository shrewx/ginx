package openapi

import (
	"bytes"
	"context"
	"fmt"
	"github.com/go-courier/oas"
	"github.com/go-courier/packagesx"
	"github.com/go-courier/reflectx/typesutil"
	"github.com/pkg/errors"
	"github.com/shrewx/ginx"
	"github.com/shrewx/ginx/pkg/statuserror"
	"github.com/sirupsen/logrus"
	"go/ast"
	"go/constant"
	"go/types"
	"net/http"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
)

func NewOperatorScanner(pkg *packagesx.Package) *OperatorScanner {
	return &OperatorScanner{
		pkg:               pkg,
		DefinitionScanner: NewDefinitionScanner(pkg),
		StatusErrScanner:  NewStatusErrScanner(pkg),
		securityScheme:    make(map[string]*oas.SecurityScheme, 0),
	}
}

type OperatorScanner struct {
	*DefinitionScanner
	*StatusErrScanner

	pkg       *packagesx.Package
	operators map[*types.TypeName]*Operator

	securityScheme      map[string]*oas.SecurityScheme
	securityRequirement oas.SecurityRequirement
}

func (scanner *OperatorScanner) Operator(ctx context.Context, typeName *types.TypeName) *Operator {
	if typeName == nil {
		return nil
	}

	//if operator, ok := scanner.operators[typeName]; ok {
	//	return operator
	//}

	logrus.Debugf("scanning Operator `%s.%s`", typeName.Pkg().Path(), typeName.Name())

	defer func() {
		if e := recover(); e != nil {
			logrus.Error(errors.Errorf("scan Operator `%s` failed, panic: %s; calltrace: %s", fullTypeName(typeName), fmt.Sprint(e), string(debug.Stack())))
		}
	}()

	if typeStruct, ok := typeName.Type().Underlying().(*types.Struct); ok {
		operator := &Operator{}

		operator.Tag = scanner.tagFrom(typeName.Pkg().Path())

		scanner.scanRouteMeta(operator, typeName)

		scanner.scanParameterOrRequestBody(ctx, operator, typeStruct)

		scanner.scanReturns(ctx, operator, typeName)

		if scanner.operators == nil {
			scanner.operators = map[*types.TypeName]*Operator{}
		}

		scanner.operators[typeName] = operator

		return operator
	}

	return nil
}

func (scanner *OperatorScanner) singleReturnOf(typeName *types.TypeName, name string) (string, bool) {
	if typeName == nil {
		return "", false
	}

	for _, typ := range []types.Type{
		typeName.Type(),
		types.NewPointer(typeName.Type()),
	} {
		method, ok := typesutil.FromTType(typ).MethodByName(name)
		if ok {
			results, n := scanner.pkg.FuncResultsOf(method.(*typesutil.TMethod).Func)
			if n == 1 {
				for _, v := range results[0] {
					if v.Value != nil {
						s, err := strconv.Unquote(v.Value.ExactString())
						if err != nil {
							panic(errors.Errorf("%s: %s", err, v.Value))
						}
						return s, true
					}
				}
			}
		}
	}

	return "", false
}

func (scanner *OperatorScanner) tagFrom(pkgPath string) string {
	tag := strings.TrimPrefix(pkgPath, scanner.pkg.PkgPath)
	return strings.TrimPrefix(tag, "/")
}

func (scanner *OperatorScanner) scanRouteMeta(op *Operator, typeName *types.TypeName) {
	op.ID = typeName.Name()

	lines := scanner.pkg.CommentsOf(scanner.pkg.IdentOf(typeName))
	comments := strings.Split(lines, "\n")

	for i := range comments {
		if strings.Contains(comments[i], "@deprecated") {
			op.Deprecated = true
		}
	}

	if op.Summary == "" {
		comments = filterMarkedLines(comments)

		if comments[0] != "" {
			op.Summary = comments[0]
			if len(comments) > 1 {
				op.Description = strings.Join(comments[1:], "\n")
			}
		}
	}

	if method, ok := scanner.singleReturnOf(typeName, "Method"); ok {
		op.Method = method
	}

	if path, ok := scanner.singleReturnOf(typeName, "Path"); ok {
		op.Path = path
	}

	if typ, ok := scanner.singleReturnOf(typeName, "Type"); ok {
		switch typ {
		case ginx.APIKey:
			scanner.securityScheme[ginx.APIKey] = oas.NewAPIKeySecurityScheme("AccessToken", oas.PositionHeader)
		case ginx.BasicAuth:
			scanner.securityScheme[ginx.BasicAuth] = oas.NewHTTPSecurityScheme("basic", "")
		case ginx.BearerJWT:
			scanner.securityScheme[ginx.BearerJWT] = oas.NewHTTPSecurityScheme("bearer", "JWT")
		}
	}
}

// BindSecuritySchemas https://swagger.io/docs/specification/authentication/
func (scanner *OperatorScanner) BindSecuritySchemas(openapi *oas.OpenAPI) {
	if len(scanner.securityScheme) > 0 {
		for k, v := range scanner.securityScheme {
			openapi.AddSecurityScheme(k, v)
			security := make(oas.SecurityRequirement, 0)
			security[k] = make([]string, 0)
			openapi.AddSecurityRequirement(&security)
		}
	}
}

func (scanner *OperatorScanner) scanReturns(ctx context.Context, op *Operator, typeName *types.TypeName) {
	for _, typ := range []types.Type{
		typeName.Type(),
		types.NewPointer(typeName.Type()),
	} {
		method, ok := typesutil.FromTType(typ).MethodByName("Output")
		if ok {
			results, n := scanner.pkg.FuncResultsOf(method.(*typesutil.TMethod).Func)
			fmt.Printf("%s:%+v\n", op.ID, results)
			if n == 2 {
				for _, v := range results[0] {
					if v.Type != nil {
						if v.Type.String() != types.Typ[types.UntypedNil].String() {
							if op.SuccessType != nil && op.SuccessType.String() != v.Type.String() {
								logrus.Warn(errors.Errorf("%s success result must be same struct, but got %v, already set %v", op.ID, v.Type, op.SuccessType))
							}
							op.SuccessType = v.Type
							op.SuccessStatus, op.SuccessResponse = scanner.getResponse(ctx, v.Type, v.Expr)
						}
					}
				}
			}

			if scanner.StatusErrScanner.StatusErrType != nil {
				op.StatusErrors = scanner.StatusErrScanner.StatusErrorsInFunc(method.(*typesutil.TMethod).Func)
				schema := scanner.DefinitionScanner.GetSchemaByType(ctx, scanner.StatusErrScanner.StatusErrType)

				op.StatusErrorSchema = schema
			}
		}
	}
}

func (scanner *OperatorScanner) firstValueOfFunc(named *types.Named, name string) (interface{}, bool) {
	method, ok := typesutil.FromTType(types.NewPointer(named)).MethodByName(name)
	if ok {
		results, n := scanner.pkg.FuncResultsOf(method.(*typesutil.TMethod).Func)
		if n == 1 {
			for _, r := range results[0] {
				if r.IsValue() {
					if v := valueOf(r.Value); v != nil {
						return v, true
					}
				}
			}
			return nil, true
		}
	}
	return nil, false
}

func (scanner *OperatorScanner) getResponse(ctx context.Context, tpe types.Type, expr ast.Expr) (statusCode int, response *oas.Response) {
	response = &oas.Response{}

	if tpe.String() == "error" {
		statusCode = http.StatusNoContent
		return
	}

	contentType := ""

	if true {
		scanResponseWrapper := func(expr ast.Expr) {
			//firstCallExpr := true
			ast.Inspect(expr, func(node ast.Node) bool {
				switch callExpr := node.(type) {
				case *ast.CallExpr:
					//needEval := true
					switch e := callExpr.Fun.(type) {
					case *ast.SelectorExpr:
						switch e.Sel.Name {
						case "WithSchema":
							v, _ := scanner.pkg.Eval(callExpr.Args[0])
							tpe = v.Type
						case "WithStatusCode":
							v, _ := scanner.pkg.Eval(callExpr.Args[0])
							if code, ok := valueOf(v.Value).(int); ok {
								statusCode = code
							}
							return false
						case "WithContentType":
							v, _ := scanner.pkg.Eval(callExpr.Args[0])
							if code, ok := valueOf(v.Value).(string); ok {
								contentType = code
							}
							return false
						case "NewAttachment":
							v, _ := scanner.pkg.Eval(callExpr.Args[1])
							if code, ok := valueOf(v.Value).(string); ok {
								contentType = code
							} else {
								contentType = ginx.MineApplicationOctetStream
							}
							//needEval = false
							return false
						}
						//if firstCallExpr {
						//	firstCallExpr = false
						//	if len(callExpr.Args) > 0 && needEval {
						//		v, _ := scanner.pkg.Eval(callExpr.Args[0])
						//		tpe = v.Type
						//	}
						//}
					}
				}
				return true
			})
		}

		if ident, ok := expr.(*ast.Ident); ok && ident.Obj != nil {
			if stmt, ok := ident.Obj.Decl.(*ast.AssignStmt); ok {
				for _, e := range stmt.Rhs {
					scanResponseWrapper(e)
				}
			}
		} else {
			scanResponseWrapper(expr)
		}
	}

	if pointer, ok := tpe.(*types.Pointer); ok {
		tpe = pointer.Elem()
	}

	if named, ok := tpe.(*types.Named); ok {
		if v, ok := scanner.firstValueOfFunc(named, "ContentType"); ok {
			if s, ok := v.(string); ok {
				contentType = s
			}
			if contentType == "" {
				contentType = "*"
			}
		}
		if v, ok := scanner.firstValueOfFunc(named, "StatusCode"); ok {
			if i, ok := v.(int64); ok {
				statusCode = int(i)
			}
		}
	}

	if contentType == "" {
		contentType = ginx.MineApplicationJson
	}
	s := scanner.DefinitionScanner.GetSchemaByType(ctx, tpe)
	response.AddContent(contentType, oas.NewMediaTypeWithSchema(s))

	return
}

func (scanner *OperatorScanner) scanParameterOrRequestBody(ctx context.Context, op *Operator, typeStruct *types.Struct) {
	var needScanForm bool
	typesutil.EachField(typesutil.FromTType(typeStruct), "name", func(field typesutil.StructField, fieldDisplayName string, omitempty bool) bool {

		location, _ := tagValueAndFlagsByTagString(field.Tag().Get("in"))

		if location == "" {
			panic(errors.Errorf("missing tag `in` for %s of %s", field.Name(), op.ID))
		}

		if location == "urlencoded" || location == "form" {
			needScanForm = true
			return true
		}

		name, flags := tagValueAndFlagsByTagString(field.Tag().Get("name"))

		schema := scanner.DefinitionScanner.propSchemaByField(
			ctx,
			field.Name(),
			field.Type().(*typesutil.TType).Type,
			field.Tag(),
			name,
			flags,
			scanner.pkg.CommentsOf(scanner.pkg.IdentOf(field.(*typesutil.TStructField).Var)),
		)

		switch location {
		case "query":
			op.AddNonBodyParameter(oas.QueryParameter(fieldDisplayName, schema, !omitempty))
		case "path":
			op.AddNonBodyParameter(oas.PathParameter(fieldDisplayName, schema))
		case "cookie":
			op.AddNonBodyParameter(oas.CookieParameter(fieldDisplayName, schema, !omitempty))
		case "header":
			// https://swagger.io/docs/specification/authentication/basic-authentication/
			op.AddNonBodyParameter(oas.HeaderParameter(fieldDisplayName, schema, false))
		case "body":
			reqBody := oas.NewRequestBody("", true)
			reqBody.AddContent("application/json", oas.NewMediaTypeWithSchema(schema))
			op.SetRequestBody(reqBody)
		}

		return true
	}, "in")

	if needScanForm {
		scanner.scanForm(ctx, op, typeStruct)
	}
}

func (scanner *OperatorScanner) scanForm(ctx context.Context, op *Operator, t *types.Struct) {
	structSchema := oas.ObjectOf(nil)
	schemas := make([]*oas.Schema, 0)
	var location = "form"

	for i := 0; i < t.NumFields(); i++ {
		field := t.Field(i)

		if !field.Exported() {
			continue
		}

		structFieldType := field.Type()

		tags := reflect.StructTag(t.Tag(i))

		tagValueForName := tags.Get("json")
		if tagValueForName == "" {
			tagValueForName = tags.Get("name")
		}
		tagValueForLocation := tags.Get("in")
		if tagValueForLocation != "form" &&
			tagValueForLocation != "urlencoded" &&
			tagValueForLocation != "multipart" {
			continue
		}

		if tagValueForLocation == "urlencoded" {
			location = tagValueForLocation
		}

		name, flags := tagValueAndFlagsByTagString(tagValueForName)
		if name == "-" {
			continue
		}

		if name == "" && field.Anonymous() {
			if field.Type().String() == "bytes.Buffer" {
				structSchema = oas.Binary()
				break
			}
			s := scanner.GetSchemaByType(ctx, structFieldType)
			if s != nil {
				schemas = append(schemas, s)
			}
			continue
		}

		if name == "" {
			name = field.Name()
		}

		required := true
		if hasOmitempty, ok := flags["omitempty"]; ok {
			required = !hasOmitempty
		}

		structSchema.SetProperty(
			name,
			scanner.propSchemaByField(ctx, field.Name(), structFieldType, tags, name, flags, scanner.pkg.CommentsOf(scanner.pkg.IdentOf(field))),
			required,
		)
	}

	if len(schemas) > 0 {
		structSchema = oas.AllOf(append(schemas, structSchema)...)
	}

	switch location {
	case "urlencoded":
		reqBody := oas.NewRequestBody("", true)
		reqBody.AddContent("application/x-www-form-urlencoded", oas.NewMediaTypeWithSchema(structSchema))
		op.SetRequestBody(reqBody)
	case "form", "multipart":
		reqBody := oas.NewRequestBody("", true)
		reqBody.AddContent("multipart/form-data", oas.NewMediaTypeWithSchema(structSchema))
		op.SetRequestBody(reqBody)
	}
}

type Operator struct {
	ID         string
	Method     string
	Path       string
	BasePath   string
	Summary    string
	Deprecated bool

	Tag         string
	Description string

	NonBodyParameters map[string]*oas.Parameter
	RequestBody       *oas.RequestBody

	StatusErrors      []*statuserror.StatusErr
	StatusErrorSchema *oas.Schema

	SuccessStatus   int
	SuccessType     types.Type
	SuccessResponse *oas.Response
}

func (operator *Operator) AddNonBodyParameter(parameter *oas.Parameter) {
	if operator.NonBodyParameters == nil {
		operator.NonBodyParameters = map[string]*oas.Parameter{}
	}
	parameter.Description = parameter.Schema.Description
	operator.NonBodyParameters[parameter.Name] = parameter
}

func (operator *Operator) SetRequestBody(requestBody *oas.RequestBody) {
	operator.RequestBody = requestBody
}

func (operator *Operator) BindOperation(method string, operation *oas.Operation, last bool) {
	parameterNames := map[string]bool{}
	for _, parameter := range operation.Parameters {
		parameterNames[parameter.Name] = true
	}

	for _, parameter := range operator.NonBodyParameters {
		if !parameterNames[parameter.Name] {
			operation.Parameters = append(operation.Parameters, parameter)
		}
	}

	if operator.RequestBody != nil {
		operation.SetRequestBody(operator.RequestBody)
	}

	for _, statusError := range operator.StatusErrors {
		statusErrorList := make([]string, 0)

		if operation.Responses.Responses != nil {
			code := statusError.StatusCode()
			if statusError.StatusCode() < 400 {
				code = 500
			}
			if resp, ok := operation.Responses.Responses[code]; ok {
				if resp.Extensions != nil {
					if v, ok := resp.Extensions[XStatusErrs]; ok {
						if list, ok := v.([]string); ok {
							statusErrorList = append(statusErrorList, list...)
						}
					}
				}
			}
		}

		for _, summary := range statusErrorList {
			if summary == statusError.Summary() {
				continue
			}
		}

		statusErrorList = append(statusErrorList, statusError.Summary())
		statusErrorList = removeDuplicate(statusErrorList)

		sort.Strings(statusErrorList)

		resp := oas.NewResponse("")
		resp.AddExtension(XStatusErrs, statusErrorList)
		if len(statusErrorList) > 0 {
			var description = new(bytes.Buffer)
			fmt.Fprintln(description, ">")
			for _, err := range statusErrorList {
				fmt.Fprintln(description, fmt.Sprintf("* `%s`", err))
			}
			resp.Description = description.String()
		}
		resp.AddContent("application/json", oas.NewMediaTypeWithSchema(operator.StatusErrorSchema))
		code := statusError.StatusCode()
		if statusError.StatusCode() < 400 {
			code = 500
		}
		operation.AddResponse(code, resp)
	}

	if last {
		operation.OperationId = operator.ID
		operation.Deprecated = operator.Deprecated
		operation.Summary = operator.Summary
		operation.Description = operator.Description

		if operator.Tag != "" {
			operation.Tags = []string{operator.Tag}
		}

		if operator.SuccessType == nil {
			operation.AddResponse(http.StatusNoContent, &oas.Response{})
		} else {
			status := operator.SuccessStatus
			if status == 0 {
				status = http.StatusOK
				if method == http.MethodPost {
					status = http.StatusCreated
				}
			}
			if status >= http.StatusMultipleChoices && status < http.StatusBadRequest {
				operator.SuccessResponse = oas.NewResponse(operator.SuccessResponse.Description)
			}
			operation.Responses.AddResponse(status, operator.SuccessResponse)
		}
	}

	// sort all parameters by postion and name
	//if len(operation.Parameters) > 0 {
	//	sort.Slice(operation.Parameters, func(i, j int) bool {
	//		return positionOrders[operation.Parameters[i].In]+operation.Parameters[i].Name <
	//			positionOrders[operation.Parameters[j].In]+operation.Parameters[j].Name
	//	})
	//}
}

func valueOf(v constant.Value) interface{} {
	if v == nil {
		return nil
	}

	switch v.Kind() {
	case constant.Float:
		v, _ := strconv.ParseFloat(v.String(), 64)
		return v
	case constant.Bool:
		v, _ := strconv.ParseBool(v.String())
		return v
	case constant.String:
		v, _ := strconv.Unquote(v.String())
		return v
	case constant.Int:
		v, _ := strconv.ParseInt(v.String(), 10, 64)
		return v
	}

	return nil
}

func removeDuplicate(slc []string) []string {
	result := []string{}
	for i := range slc {
		flag := true
		for j := range result {
			if slc[i] == result[j] {
				flag = false
				break
			}
		}
		if flag {
			result = append(result, slc[i])
		}
	}
	return result
}
