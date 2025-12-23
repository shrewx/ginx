package user

import (
	"{{ .ProjectName }}/global"
	"{{ .ProjectName }}/repositories/models"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx"
)

// UserListResponse 用户列表响应
type UserListResponse struct {
	// 用户列表
	Users []*models.User `json:"users"`
	// 总用户数
	Total int64 `json:"total"`
}

// ListUsers 获取用户列表
type ListUsers struct {
	ginx.MethodGet
	// 页码
	Page int `in:"query" validate:"omitempty,min=1"`
	// 每页数量
	Limit int `in:"query" validate:"omitempty,min=1,max=100"`
}

func (l *ListUsers) Path() string {
	return ""
}

func (l *ListUsers) Output(ctx *gin.Context) (interface{}, error) {
	if l.Page == 0 {
		l.Page = 1
	}
	if l.Limit == 0 {
		l.Limit = 10
	}
	offset := (l.Page - 1) * l.Limit
	users, total, err := global.UserServiceManager.ListUsers(ctx, offset, l.Limit)
	if err != nil {
		return nil, err
	}
	return UserListResponse{
		Users: users,
		Total: total,
	}, nil
}