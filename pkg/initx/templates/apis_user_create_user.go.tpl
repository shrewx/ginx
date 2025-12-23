package user

import (
	"{{ .ProjectName }}/global"
	"{{ .ProjectName }}/repositories/models"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// 创建用户请求
type CreateUserRequest struct {
	// 用户名
	Username string `json:"username" validate:"required,min=3,max=100"`
	// 邮箱
	Email string `json:"email" validate:"required,email"`
	// 密码
	Password string `json:"password" validate:"required,min=6"`
	// 昵称
	Nickname string `json:"nickname" validate:"max=100"`
	// 状态
	Status string `json:"status" validate:"omitempty,oneof=active inactive"`
}

// CreateUser 创建用户
type CreateUser struct {
	ginx.MethodPost
	// 请求体
	Body CreateUserRequest `in:"body"`
}

func (c *CreateUser) Path() string {
	return ""
}

func (c *CreateUser) Validate(ctx *gin.Context) error {
	return nil
}

func (c *CreateUser) Output(ctx *gin.Context) (interface{}, error) {
	user := &models.User{
		Username: c.Body.Username,
		Email:    c.Body.Email,
		Password: c.Body.Password,
		Nickname: c.Body.Nickname,
		Status:   c.Body.Status,
	}
	if user.Status == "" {
		user.Status = "active"
	}
	if err := global.UserServiceManager.CreateUser(ctx.Request.Context(), user); err != nil {
		return nil, err
	}
	return ginx.CommonSuccessResponse(), nil
}