## 接口定义

### 路由

所有的路由定义需要实现`HandleOperator`这个接口，里面包括三个方法

* Path() string // 说明该路由的路径
* Method() string // 说明该路由的HTTP Method
* Validate(ctx *gin.Context)  error // 校验参数，返回校验错误
* Output(ctx *gin.Context) (interface{}, error)  // 接口的具体功能逻辑

例如：

```go
type GetHelloWorld struct {
    ginx.MethodGet
}

type GetHelloWorldResponse struct {
    Message string `json:"message"`
}

func (g *GetHelloWorld) Path() string {
    return "/hello"
}

func (g *GetHelloWorld) Validate(ctx *gin.Context) error {
    return nil
}

func (g *GetHelloWorld) Output(ctx *gin.Context) (interface{}, error) {
    return GetHelloWorldResponse{Message: "hello world"}, nil
}
```

这就是一个接口的完整定义，并且建议一个接口一个文件，文件名可与类名相同，如`get_hello_world.go`，这样开发者方便查看和修改。

* `Validate(ctx *gin.Context) error`方法，对请求参数进行校验。如果校验失败，需要返回一个错误。
    ```go
    func (g *GetUserInfo) Validate(ctx *gin.Context) error {
        if g.Username == "" {
            return errors.BadRequest
        }
        return nil
    }
    ```

* `Output(ctx *gin.Context) (interface{}, error)` 有两个返回值。
    * 第一个定义为`interface`，即返回任何类型的对象都可以， 框架会判断其类型来设置不同的ContextType(默认使用`application/json`)
    * 第二个是`error`,为了规范错误码的定义，使用[statuserror](https://github.com/shrewx/ginx/pkg/statuserror)库和[自动化工具](https://github.com/shrewx/ginx/pkg/toolx)进行生成，错误码返回结构定义为：
        ```json
        {
          "key": "DDIResourceNotFound", 
          "code": "404000000001", 
          "message": "视图未找到",
        }
        ```
        如果返回的error没有实现`CommonError`这个接口，错误就会封装成`status_error.CommonError`
        ```go
        status_error.CommonError{
            Key:     "InternalServerError",
            Code:    http.StatusBadGateway,
            Message: e.Error(),
        }
        ```

### 路由组
路由组定义一组路由的相同的路径前缀，和gin的Group是一个概念。

例如：
```go
var (
	V0Router = ginx.NewRouter(ginx.Group("v0"))
)

func init() {
	V0Router.Register(&GetHelloWorld{})
}
```
那么`GetHelloWorld`这个接口的完整路径就是`/v0/hello`

### 中间件

中间件需要实现的接口是`TypeOperator`,里面包括两个方法：
* Output(ctx *gin.Context) (interface{}, error)
* Type() string

若需要实现一个认证功能，所有接口都必须进行用户密码认证后才能访问则代码实现如下：
```go
type BaseAuth struct {
	ginx.MiddlewareType
}

func (g *BaseAuth) Output(ctx *gin.Context) (interface{}, error) {
	return gin.BasicAuth(map[string]string{
		"admin": "admin",
	}), nil
}

var V0Router = ginx.NewRouter(ginx.Group("v0"), &BaseAuth{})
```

如果希望实现before和after，则实现`MiddlewareOperator`接口，里面包括四个方法：
* Output(ctx *gin.Context) (interface{}, error)
* Type() string
* Before(ctx *gin.Context) error
* After(ctx *gin.Context) error
  举个例子：
```go
type LoggingMiddleware struct {
	ginx.EmptyMiddlewareOperator
}

func (g *LoggingMiddleware) Before(ctx *gin.Context) error {
	logfile.Info("request: ", ctx.Request.URL.Path)
	return nil
}

func (g *LoggingMiddleware) After(ctx *gin.Context) error {
	logfile.Info("response: ", ctx.Writer.Status())
	return nil
}
```

### 请求参数

请求参数类型通过tag进行区分，使用关键字`in`声明参数类型，`name`声明参数名称,框架会自动解析请求的参数，并填充到结构体对应的成员变量中方便实用

同时使用了[validator](https://github.com/go-playground/validator)库，对参数进行校验, 使用关键字`validate`声明校验规则

参数类型如下：

#### Query、Path
```go
type GetUserInfo struct {
	ginx.MethodGet
	Username string `in:"query" validate:"required"`
	ID       int    `in:"path" validate:"min=10"`
}

func (g *GetUserInfo) Path() string {
	return "/:id"
}

func (g *GetUserInfo) Output(ctx *gin.Context) (interface{}, error) {
	logfile.Info("id: ", g.ID, " username: ", g.Username)
	return nil, nil
}
```

#### Header
```go
type BaseAuth struct {
	ginx.HTTPBasicAuthSecurityType
	Name string `in:"header" name:"Authorization"`
}

func (g *BaseAuth) Output(ctx *gin.Context) (interface{}, error) {
	if g.Name != authorization("admin", "admin") {
		return nil, errors.Unauthorized
	}

	return nil, nil
}
```

#### Form-data
````go
type PutUserInfo struct {
	ginx.MethodPost
	// 名称
	Name    string `in:"form" name:"name"`
	// 年龄
	Age     int    `in:"form" name:"age"`
	// 地址
	Address string `in:"form" name:"address"`
}
````
#### Form-Multipart
```go
type UploadFile struct {
	ginx.MethodPost
	File *multipart.FileHeader `in:"multipart" name:"file1"`
}

func (u *UploadFile) Output(ctx *gin.Context) (interface{}, error) {
	if u.File == nil {
		return nil, errors.UploadFileIsNotExist
	}

	if err := ctx.SaveUploadedFile(u.File, u.File.Filename); err != nil {
		return nil, err
	}

	return nil, nil
}
```
#### Form-urlencoded
```go
type ModifyUserInfo struct {
	ginx.MethodPut
	Name string `in:"urlencoded" name:"name"`
}
```
#### Body
body类型里面tag直接使用`json`就可
```go
type CreateUserInfo struct {
	ginx.MethodPost
	Body struct {
		// 名称
		Name    string `json:"name"`
		// 年龄
		Age     int    `json:"age"`
		// 地址
		Address string `json:"address"`
	} `in:"body"`
}
```

#### 其他
如果以上都不满足要求，可直接使用gin的库的对应的方法获取请求参数。

### ContentType

如上面提到的，默认的情况下response的contentType是`application/json`, 如果需要其他类型的，框架封装了一些[Mine](https://github.com/shrewx/ginx/-/blob/master/mine.go)结构体可使用。

#### 文件下载
```go
func (g *DownloadFile) Output(ctx *gin.Context) (interface{}, error) {
	file := ginx.NewAttachment("text.txt", ginx.MineApplicationOctetStream)
	file.Write([]byte("hello world"))
	return file, nil
}
```

#### HTML

```go
func (g *HTML) Output(ctx *gin.Context) (interface{}, error) {
	html := ginx.NewHTML()
	html.Write([]byte("<body> hello world</body>"))
	return html, nil
}
```
#### Image
```go
func (g *Image) Output(ctx *gin.Context) (interface{}, error) {
	png := ginx.NewImagePNG()
	file, err := os.ReadFile("./router/file/go.png")
	if err != nil {
		return nil, err
	}
	png.Write(file)
	return png, nil
}
```
#### 其他
如果框架中列出的Mine都不满足，可以自行实现MineDescriber接口或者直接使用gin的ctx.Data方法设置
```go
func (g *OtherType) Output(ctx *gin.Context) (interface{}, error) {
	ctx.Data(http.StatusOK, "other_type", []byte("hello world"))
	return nil, nil
}
```

## 注释Swagger
生成swagger文档go常见方式是使用go-swagger库搭配注释的形式，该库同样也是通过注释的形式来实现swagger文档的生成。
有所不同的是不需要特定的tag说明，而是使用ast库对代码进行所有注释的扫描，并且对响应结果和错误都会进行类型判断。

**重要** 请求参数、API结构体的定义都需要在注释中进行说明，框架会根据注释自动生成swagger文档！！！。
```go
func init() {
	Router.Register(&CreateUserInfo{})
}

// 创建用户信息
type CreateUserInfo struct {
	ginx.MethodPost
	ID int   `in:"path"`
	Body struct {
		// 名称
		Name    string ` json:"name"`
		// 年龄
		Age     int    `json:"age"`
		// 地址
		Address string `json:"address"`
	} `in:"body"`
}

type CreateUserInfoResponse struct{
	
}

func (g *CreateUserInfo) Path() string {
	return "/:id"
}

func (g *CreateUserInfo) Output(ctx *gin.Context) (interface{}, error) {
	logfile.Info(g.Body.Name, g.Body.Age, g.Body.Address)
	return nil, nil
}

```