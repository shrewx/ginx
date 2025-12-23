package apis

import (
	"{{ .ProjectName }}/apis/user"
	"github.com/shrewx/ginx"
)

var RootRouter = ginx.NewRouter(ginx.Group("/api/v1"))

func init() {
	RootRouter.Register(user.Router)
}

