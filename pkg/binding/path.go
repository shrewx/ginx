package binding

import (
	"github.com/gin-gonic/gin"
)

type pathBinding struct{}

func (pathBinding) Name() string {
	return "path"
}

func (pathBinding) Bind(ctx *gin.Context, obj any) error {
	m := make(map[string][]string)
	for _, v := range ctx.Params {
		m[v.Key] = []string{v.Value}
	}
	return mapName(obj, m)
}
