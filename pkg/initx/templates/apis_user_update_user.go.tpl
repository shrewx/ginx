package user

import (
	"{{ .ProjectName }}/global"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 更新用户请求
type UpdateUserRequest struct {
	// 邮箱
	Email string `json:"email" validate:"omitempty,email"`
	// 密码
	Password string `json:"password" validate:"omitempty,min=6"`
	// 昵称
	Nickname string `json:"nickname" validate:"omitempty,max=100"`
	// 状态
	Status string `json:"status" validate:"omitempty,oneof=active inactive"`
}

// UpdateUser 更新用户
type UpdateUser struct {
	ginx.MethodPut
	// 请求体
	Body UpdateUserRequest `in:"body" json:"body"`
	// 用户ID
	ID uint `in:"path" validate:"required"`
}

func (u *UpdateUser) Path() string {
	return "/:id"
}

func (u *UpdateUser) Output(ctx *gin.Context) (interface{}, error) {
	updates := make(map[string]interface{})
	if u.Body.Email != "" {
		updates["email"] = u.Body.Email
	}
	if u.Body.Password != "" {
		updates["password"] = u.Body.Password
	}
	if u.Body.Nickname != "" {
		updates["nickname"] = u.Body.Nickname
	}
	if u.Body.Status != "" {
		updates["status"] = u.Body.Status
	}
	if err := global.UserServiceManager.UpdateUser(ctx.Request.Context(), u.ID, updates); err != nil {
		return nil, err
	}

	return ginx.CommonSuccessResponse(), nil
}




