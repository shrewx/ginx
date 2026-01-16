package user

import (
	"{{ .ProjectName }}/global"
	"github.com/gin-gonic/gin"
    "github.com/shrewx/ginx"
)

// GetUser 获取用户
type GetUser struct {
	ginx.MethodGet
	// 用户ID
	ID uint `in:"path" validate:"required"`
}

func (g *GetUser) Path() string {
	return "/:id"
}

func (g *GetUser) Output(ctx *gin.Context) (interface{}, error) {
	user, err := global.UserServiceManager.GetUserByID(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	return user, nil
}



