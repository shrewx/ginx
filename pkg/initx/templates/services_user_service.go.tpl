package services

import (
	"{{ .ProjectName }}/repositories/controllers"
	"{{ .ProjectName }}/repositories/models"
	"context"
	"github.com/shrewx/ginx/pkg/dbhelper"
	"gorm.io/gorm"
)

type UserService struct {
	txController   dbhelper.TransactionManager
	userController controllers.UserController
}

func NewUserService(db *gorm.DB) *UserService {
	return &UserService{
		txController:   dbhelper.NewGormTransactionManager(db),
		userController: controllers.NewUserController(db),
	}
}

// CreateUser 创建用户
func (s *UserService) CreateUser(ctx context.Context, user *models.User) error {
	return s.userController.Create(ctx, user)
}

// GetUserByID 根据ID获取用户
func (s *UserService) GetUserByID(ctx context.Context, id uint) (*models.User, error) {
	return s.userController.GetByID(ctx, id)
}

// GetUserByUsername 根据用户名获取用户
func (s *UserService) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	return s.userController.GetByUsername(ctx, username)
}

// UpdateUser 更新用户
func (s *UserService) UpdateUser(ctx context.Context, id uint, updates map[string]interface{}) error {
	return s.userController.Update(ctx, id, updates)
}

// DeleteUser 删除用户
func (s *UserService) DeleteUser(ctx context.Context, id uint) error {
	return s.userController.Delete(ctx, id)
}

// ListUsers 获取用户列表
func (s *UserService) ListUsers(ctx context.Context, offset, limit int) ([]*models.User, int64, error) {
	return s.userController.List(ctx, offset, limit)
}

// CountUsers 统计用户总数
func (s *UserService) CountUsers(ctx context.Context) (int64, error) {
	return s.userController.Count(ctx)
}


