package user

import (
	"{{ .ProjectName }}/global"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// DeleteUser 删除用户
type DeleteUser struct {
	ginx.MethodDelete
	// 用户ID
	ID uint `in:"path" validate:"required"`
}

func (d *DeleteUser) Path() string {
	return "/:id"
}

func (d *DeleteUser) Output(ctx *gin.Context) (interface{}, error) {
	if err := global.UserServiceManager.DeleteUser(ctx.Request.Context(), d.ID); err != nil {
		return nil, err
	}
	return ginx.CommonSuccessResponse(), nil
}


