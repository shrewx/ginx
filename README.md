# 使用文档

## 介绍
接口结构参考[httptransport](https://github.com/go-courier/httptransport)设计思想，对gin做了封装，并使用[httptransport](https://github.com/go-courier/httptransport)的openapi和client自动生成工具适配


## 快速上手

## 说明

### 路由

所有的路由定义需要实现`HandleOperator`这个接口，里面包括三个方法

* Path() string // 说明该路由的路径
* Method() string // 说明该路由的HTTP Method
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

func (g *GetHelloWorld) Output(ctx *gin.Context) (interface{}, error) {
    return GetHelloWorldResponse{Message: "hello world"}, nil
}
```

这就是一个接口的完整定义，并且建议一个接口一个文件，文件名可与类名相同，如`get_hello_world.go`，这样开发者方便查看和修改。

其中`Output(ctx *gin.Context) (interface{}, error)` 有两个返回值。

第一个定义为`interface`，即返回任何类型的对象都可以， 框架会判断其类型来设置不同的ContextType(默认使用`application/json`)

第二个是`error`,为了规范错误码的定义，使用[错误码工具](https://github.com/shrewx/ginx/toolX)进行生成，错误码返回结构定义为：
```json
{
  "key": "DDIResourceNotFound", 
  "code": "404000000001", 
  "message": "视图未找到",
}
```
如果返回的error没有实现`CommonError`这个接口，错误就会封装成`status_error.CommonError`
```json
status_error.CommonError{
    Key:     "InternalServerError",
    Code:    http.StatusBadGateway,
    Message: e.Error(),
}
```

和gin对比

```go

ir.GET("/hello", func (context *gin.Context) {
    context.JSON(http.StatusOK, struct {
        Message string `json:"message"`
        }{
	 Message: "hello world"
	})
    }
)
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

和gin对比
```go
group := ir.Group("v0")
group.GET("/hello", func (context *gin.Context) {
context.JSON(http.StatusOK, struct {
Message string `json:"message"`
    }{
	Message: "hello world"
    })
  }
)
```

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
和gin对比

```go
group := ir.Group("v0",gin.BasicAuth(map[string]string{
    "admin": "admin",
}))
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
	Name    string `in:"form" name:"name"`
	Age     int    `in:"form" name:"age"`
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
		Name    string `json:"name"`
		Age     int    `json:"age"`
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
如果框架中列出的Mine都不满足，可直接使用gin的ctx.Data方法设置
```go
func (g *OtherType) Output(ctx *gin.Context) (interface{}, error) {
	ctx.Data(http.StatusOK, "other_type", []byte("hello world"))
	return nil, nil
}
```

### 注释Swagger
生成swagger文档go常见方式是使用go-swagger库搭配注释的形式，该库同样也是通过注释的形式来实现swagger文档的生成。
有所不同的是不需要特定的tag说明，而是使用ast库对代码进行所有注释的扫描，并且对响应结果和错误都会进行类型判断。
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

由于go-swagger暂时只支持到openapi2.0, 而本库使用的是openapi3.0，所以就没有直接通过引入go-swagger库来展示swagger ui，而是通过docker启动了swaggerui达到相同效果。使用到的命令是：
```shell
go install github.com/shrewx/toolx

toolx swagger -p "swagger ui 页面的端口,默认9200" -s "后台提供服务的地址，默认http://127.0.0.1:8888"
```
容器启动以后，就可以访问对应的页面

[![2022-07-31-16-38-36.png](https://i.postimg.cc/vBvHNm0F/2022-07-31-16-38-36.png)](https://postimg.cc/LYnpqmdN)


如果只需要生成openapi则使用命令：
```shell
toolx gen openapi -p "后台服务代码路径，默认为当前路径"
```

### Client生成



### 提高开发效率

#### Goland添加接口模版
在 Preferences --> Editor --> Live Template 添加一个Go Template
```go
import (
	"github.com/shrewx/ginx"
	"github.com/gin-gonic/gin"
)

func init() {
	Router.Register(&$Struct${})
}

type $Struct$ struct {
	ginx.MethodGet
}

func (g *$Struct$) Path() string {
	return ""
}

func (g *$Struct$) Output(ctx *gin.Context) (interface{}, error) {
	$END$
	return nil, nil
}
```