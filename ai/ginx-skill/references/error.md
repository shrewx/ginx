## 错误处理
### 接口错误文件定义
目前错误定义设计的结构如下：
```go
//go:generate toolx gen error -p error_codes -c StatusError
//go:generate toolx gen errorYaml -p error_codes -o ../i18n -c StatusError
type StatusError int

const (
	// @errZH 请求参数错误
	// @errEN bad request
	BadRequest StatusError = http.StatusBadRequest*1e8 + iota + 1
)

const (
	// @errZH 未授权，请先授权
	// @errEN unauthorized
	Unauthorized StatusError = http.StatusUnauthorized*1e8 + iota + 1
)

const (
	// @errZH 禁止操作
	// @errEN forbidden
	Forbidden StatusError = http.StatusForbidden*1e8 + iota + 1
)

const (
	// @errZH 资源未找到
	// @errEN not found
	NotFound StatusError = http.StatusNotFound*1e8 + iota + 1
)

const (
	// @errZH 资源冲突
	// @errEN conflict
	Conflict StatusError = http.StatusConflict*1e8 + iota + 1
)

const (
	// @errZH 未知的异常信息：请联系技术服务工程师进行排查
	// @errEN internal server error
	InternalServerError StatusError = http.StatusInternalServerError*1e8 + iota + 1
)

```
1. 其中每个错误码的定义都包含了中文和英文的描述，错误描述信息对应的就是最终错误返回的I18N信息：
    * errZH 表示中文描述
    * errEN 表示英文描述
2. 默认错误的定义要符合HTTP状态码的定义，即错误码的前三位就是HTTP状态码,错误信息最好和状态码表达的含义一致，比如：
    * 404表示资源未找到，则比如用户未找到错误可定义为`40400000001`
    * 409表示资源冲突, 则比如用户已存在错误可定义为`40900000001`
    如果当前项目有其他错误定义风格保持和当前风格一致即可，无需遵循HTTP状态码的定义。
### 接口错误文件生成
1. 需要在错误文件所在目录执行`toolx gen error -p error_codes -c StatusError`命令，就会在该错误文件目录下生成一个带__generated.go文件,该文件是自动生成的不要修改里面的内容否则下一次
   程序生成后就会被覆盖，生成文件主要创建了相关方法以及I18N注册。
2. 需要在错误文件所在目录执行`toolx gen errorYaml -p error_codes -o ../i18n -c StatusError`命令，会在../i18n目录下生成对应的i18yaml文件。

### 错误参数注入
1. 错误定义里面有相关参数：
   ```go
   // @errZH 用户不存在，名称：{{.Name}}
   // @errEN user not found, name: {{.Name}}
   UserNameNotFound StatusError = http.StatusNotFound*1e8 + iota + 1
   ```
   则在使用的时候需要传入参数：
   ```go
   UserNameNotFound.WithParams(map[string]interface{}{
       "Name": "ryan",
   })
   ```
   最终的错误信息为：
   ```
   用户不存在，名称：ryan
   ```
2. 错误信息里面，字段是动态的，且也需要I18N
   首先先定义定义一个string类型的常量，使用toolx生成对应的i18n
   ```go
   //go:generate toolx gen i18n -p errors.references -c Field
   type Field string
   
   const (
       // @i18nZH 年龄
       // @i18nEN age
       Age Field = "age"
   )
   ```

   ```go
   UserNameNotFound.WithParams(map[string]interface{}{
       "Name": "ryan",
   }).WithField(Age, 18)
   ```
   最终的错误信息为：
   ```
   用户不存在，名称：ryan
   >> 年龄：18
   ```
3. 如果需要捕获循环中的多个错误展示，则可以搭配error_list使用，比如：
   ```go
   func (g *Name) Output(ctx *gin.Context) (interface{}, error) {
       var errlist = statuserror.WithErrorList()
       for i, name := range g.Names {
           if err := g.checkName(name); err != nil {
               errlist.DoWithIndex(func() error {
                   return statuserror.UserNameNotFound.WithParams(map[string]interface{}{
                       "Name": name,
                   })
               }, int64(i)+1)
           }
       }
       return nil, errlist.Return()
   }
   ```
   最终的错误信息为：
   ```
   索引：1
   用户不存在，名称：a
   索引：2
   用户不存在，名称：b
   索引：3
   用户不存在，名称：c
   ```

## I18N
### 字段定义
```go
//go:generate toolx gen i18n prefix errors.references CommonField
//go:generate toolx gen i18nYaml -p errors.references -o ../i18n -c CommonField
type CommonField string

const (
	// @i18nZH 行
	// @i18nEN line
	ErrorLine CommonField = "line"
	// @i18nZH 索引
	// @i18nEN index
	ErrorIndex CommonField = "err_index"
)
```
和错误定义类似，使用
* i18nZH 标识中文信息
* i18nEN 标识英文信息
### 生成i18n文件
执行`toolx gen i18nYaml -p errors.references -c CommonField`命令
```yaml
zh:
  errors:
    references:
      err_index: 索引
      line: 行
```
可以看出，-p参数指定的是i18n的key前缀，使用.表示多级关系