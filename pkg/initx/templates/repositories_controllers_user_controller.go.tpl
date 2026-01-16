package controllers

import (
	"context"
	"{{ .ProjectName }}/repositories/models"
	"{{ .ProjectName }}/constants/status_error"
	"github.com/shrewx/ginx"
	"github.com/shrewx/ginx/pkg/dbhelper"
	"github.com/shrewx/ginx/pkg/logx"
	"gorm.io/gorm"
)

type UserController interface {
	// Create 创建用户
	Create(ctx context.Context, user *models.User) error
	// GetByID 根据ID获取用户
	GetByID(ctx context.Context, id uint) (*models.User, error)
	// GetByUsername 根据用户名获取用户
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	// Update 更新用户
	Update(ctx context.Context, id uint, updates map[string]interface{}) error
	// Delete 删除用户（软删除）
	Delete(ctx context.Context, id uint) error
	// List 获取用户列表
	List(ctx context.Context, offset, limit int) ([]*models.User, int64, error)
	// Count 统计用户总数
	Count(ctx context.Context) (int64, error)
}

type UserControllerImpl struct {
	db *gorm.DB
}

func NewUserController(db *gorm.DB) UserController {
	return &UserControllerImpl{db: db}
}

func (c *UserControllerImpl) Create(ctx context.Context, user *models.User) error {
	if err := dbhelper.GetCtxDB(ctx, c.db).Create(user).Error; err != nil {
		if dbhelper.IsDuplicateError(err) {
			return ginx.WithStack(status_error.Conflict.WithParams(map[string]interface{}{
				"Username": user.Username,
			}))
		}
		logx.Errorf("userController create error: %s", err.Error())
		return ginx.WithStack(status_error.InternalServerError)
	}
	return nil
}

func (c *UserControllerImpl) GetByID(ctx context.Context, id uint) (*models.User, error) {
	var user models.User
	if err := dbhelper.GetCtxDB(ctx, c.db).Where("id = ?", id).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ginx.WithStack(status_error.NotFound.WithParams(map[string]interface{}{
				"ID": id,
			}))
		}
		logx.Errorf("userController getByID error: %s", err.Error())
		return nil, ginx.WithStack(status_error.InternalServerError)
	}
	return &user, nil
}

func (c *UserControllerImpl) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	if err := dbhelper.GetCtxDB(ctx, c.db).Where("username = ?", username).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ginx.WithStack(status_error.NotFound.WithParams(map[string]interface{}{
				"Username": username,
			}))
		}
		logx.Errorf("userController getByUsername error: %s", err.Error())
		return nil, ginx.WithStack(status_error.InternalServerError)
	}
	return &user, nil
}

func (c *UserControllerImpl) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	if err := dbhelper.GetCtxDB(ctx, c.db).Model(&models.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		logx.Errorf("userController update error: %s", err.Error())
		return ginx.WithStack(status_error.InternalServerError)
	}
	return nil
}

func (c *UserControllerImpl) Delete(ctx context.Context, id uint) error {
	if err := dbhelper.GetCtxDB(ctx, c.db).Delete(&models.User{}, id).Error; err != nil {
		logx.Errorf("userController delete error: %s", err.Error())
		return ginx.WithStack(status_error.InternalServerError)
	}
	return nil
}

func (c *UserControllerImpl) List(ctx context.Context, offset, limit int) ([]*models.User, int64, error) {
	var users []*models.User
	var total int64

	query := dbhelper.GetCtxDB(ctx, c.db).Model(&models.User{})
	if err := query.Count(&total).Error; err != nil {
		logx.Errorf("userController list count error: %s", err.Error())
		return nil, 0, ginx.WithStack(status_error.InternalServerError)
	}

	if err := query.Offset(offset).Limit(limit).Find(&users).Error; err != nil {
		logx.Errorf("userController list error: %s", err.Error())
		return nil, 0, ginx.WithStack(status_error.InternalServerError)
	}

	return users, total, nil
}

func (c *UserControllerImpl) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := dbhelper.GetCtxDB(ctx, c.db).Model(&models.User{}).Count(&count).Error; err != nil {
		logx.Errorf("userController count error: %s", err.Error())
		return 0, ginx.WithStack(status_error.InternalServerError)
	}
	return count, nil
}




