package dbhelper

import (
	"context"
	"fmt"
	"github.com/glebarez/sqlite"
	"github.com/shrewx/ginx/pkg/conf"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/shrewx/ginx/pkg/logx"
	"golang.org/x/exp/maps"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var (
	tables = make(map[string]interface{}, 0)
)

const DBKey = "db-instance-key"

type DB struct {
	*gorm.DB
}

func NewDB(cfg conf.DB) (*DB, error) {
	if cfg.Dsn == "" {
		dsn, err := cfg.BuildDSN()
		if err != nil {
			return nil, err
		}
		cfg.Dsn = dsn
	}

	var dialector gorm.Dialector
	switch cfg.Type {
	case conf.Postgres:
		dialector = postgres.Open(cfg.Dsn)
	case conf.MySQL:
		dialector = mysql.Open(cfg.Dsn)
	case conf.Sqlite:
		dialector = sqlite.Open(cfg.Dsn)
	default:
		return nil, errors.New("unsupported db type: " + string(cfg.Type))
	}

	gormCfg := &gorm.Config{}
	if cfg.ShowLog {
		gormCfg.Logger = logger.New(
			logx.Instance(),
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  logger.Info,
				IgnoreRecordNotFoundError: true,
				ParameterizedQueries:      false,
				Colorful:                  true,
			})
	}

	db, err := gorm.Open(dialector, gormCfg)
	if err != nil {
		return nil, err
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// 设置连接池参数，使用默认值如果未配置
	maxIdleConns := cfg.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = 10
	}

	maxOpenConns := cfg.MaxOpenConns
	if maxOpenConns == 0 {
		maxOpenConns = 100
	}

	connMaxLifetime := cfg.ConnMaxLifetime
	if connMaxLifetime == 0 {
		connMaxLifetime = time.Hour
	}

	connMaxIdleTime := cfg.ConnMaxIdleTime
	if connMaxIdleTime == 0 {
		connMaxIdleTime = 15 * time.Minute
	}

	sqlDB.SetMaxIdleConns(maxIdleConns)       // 最大空闲连接数
	sqlDB.SetMaxOpenConns(maxOpenConns)       // 最大打开连接数
	sqlDB.SetConnMaxLifetime(connMaxLifetime) // 连接最大生命周期
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime) // 连接最大空闲时间

	if cfg.Migrate {
		err = db.AutoMigrate(maps.Values(tables)...)
		if err != nil {
			return nil, err
		}
	}

	return &DB{db}, nil
}

func RegisterTable(table interface{}) {
	switch t := table.(type) {
	case schema.Tabler:
		name := t.TableName()
		if _, ok := tables[name]; ok {
			panic("duplicate table name " + name)
		}
		tables[name] = table
	default:
		panic("unsupported table type")
	}
}

func Migrate(db *DB) error {
	return db.AutoMigrate(maps.Values(tables)...)
}

func SetCtxDB(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, DBKey, db)
}

func GetCtxDB(ctx context.Context, defaultDB *gorm.DB) *gorm.DB {
	data := ctx.Value(DBKey)
	switch data.(type) {
	case *gorm.DB:
		return data.(*gorm.DB)
	default:
		return defaultDB
	}
}

func DBConditionWithSearchAttrs(db *gorm.DB, searchAttrs map[string]interface{}, eqAttrs map[string]struct{}) *gorm.DB {
	if eqAttrs == nil {
		eqAttrs = map[string]struct{}{}
	}
	for k, v := range searchAttrs {
		if strings.Contains(k, "|") {
			fields := strings.Split(k, "|")

			db = applyOrCondition(db, fields, v, eqAttrs)
		} else {
			_, useEq := eqAttrs[k]
			db = applyCondition(db, k, v, useEq)
		}
	}
	return db
}

func applyOrCondition(tx *gorm.DB, fields []string, value interface{}, eqAttrs map[string]struct{}) *gorm.DB {
	var conditions []string
	var args []interface{}

	for _, f := range fields {
		if arr, ok := isSlice(value); ok {
			conditions = append(conditions, fmt.Sprintf("%s IN (?)", strings.TrimSuffix(f, "s")))
			args = append(args, arr)
		} else if _, ok := eqAttrs[f]; ok {
			conditions = append(conditions, fmt.Sprintf("%s = ?", f))
			args = append(args, value)
		} else {
			conditions = append(conditions, fmt.Sprintf("%s LIKE ?", f))
			args = append(args, "%"+fmt.Sprint(value)+"%")
		}
	}

	return tx.Where(fmt.Sprintf("(%s)", strings.Join(conditions, " OR ")), args...)
}

func applyCondition(tx *gorm.DB, field string, value interface{}, useEq bool) *gorm.DB {
	// IN 永远优先
	if arr, ok := isSlice(value); ok {
		return tx.Where(fmt.Sprintf("%s IN ?", strings.TrimSuffix(field, "s")), arr)
	}

	if useEq {
		return tx.Where(fmt.Sprintf("%s = ?", field), value)
	}

	// 默认 LIKE
	return tx.Where(fmt.Sprintf("%s LIKE ?", field), "%"+fmt.Sprint(value)+"%")
}

func isSlice(v interface{}) ([]interface{}, bool) {
	if v == nil {
		return nil, false
	}

	switch s := v.(type) {

	case []interface{}:
		return s, true

	case []string:
		out := make([]interface{}, 0, len(s))
		for _, v := range s {
			out = append(out, v)
		}
		return out, true

	case []int:
		out := make([]interface{}, 0, len(s))
		for _, v := range s {
			out = append(out, v)
		}
		return out, true

	case []int64:
		out := make([]interface{}, 0, len(s))
		for _, v := range s {
			out = append(out, v)
		}
		return out, true

	case []float64:
		out := make([]interface{}, 0, len(s))
		for _, v := range s {
			out = append(out, v)
		}
		return out, true
	}

	return nil, false
}
