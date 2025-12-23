package global

import (
	"{{ .ProjectName }}/services"
	"github.com/shrewx/ginx/pkg/conf"
	"github.com/shrewx/ginx/pkg/dbhelper"
	"gorm.io/gorm"
)

var (
	// DB 数据库连接
	DB *gorm.DB
	// Config 全局配置
	Config = &Configuration{}
	// UserServiceManager 用户服务管理器
	UserServiceManager *services.UserService
)

// Configuration 配置
type Configuration struct {
	// 服务配置
	Server conf.Server `yaml:"server"`
	// 数据库配置
	DB conf.DB `yaml:"db"`
}

func DBLoad(conf conf.DB) {
	// 初始化数据库连接
	db, err := dbhelper.NewDB(conf)
	if err != nil {
		panic(err)
	}

	DB = db.DB

	UserServiceManager = services.NewUserService(db.DB)
}