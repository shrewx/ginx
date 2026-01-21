package dbhelper

import (
	"context"
	"gorm.io/gorm"
)

// TransactionManager 事务管理接口（定义在domain层，实现依赖倒置）
type TransactionManager interface {
	// Transaction 在事务中执行函数
	// fn 函数接收一个context，该context包含事务信息
	Transaction(ctx context.Context, fn func(ctx context.Context) error) error
}

// GormTransactionManager GORM事务管理器实现
type GormTransactionManager struct {
	db *gorm.DB
}

// NewGormTransactionManager 创建GORM事务管理器
func NewGormTransactionManager(db *gorm.DB) TransactionManager {
	return &GormTransactionManager{db: db}
}

// Transaction 在事务中执行函数
func (m *GormTransactionManager) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return GetCtxDB(ctx, m.db).Transaction(func(tx *gorm.DB) error {
		// 将事务数据库连接设置到上下文中
		ctx = SetCtxDB(ctx, tx)
		// 执行用户提供的函数
		return fn(ctx)
	})
}
