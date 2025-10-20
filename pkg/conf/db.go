package conf

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
)

type DBType string

const (
	Sqlite   DBType = "sqlite"
	Postgres DBType = "postgres"
	MySQL    DBType = "mysql"
)

type DB struct {
	Type      DBType `yaml:"type" env:"DB_TYPE"` // sqlite, postgres, mysql
	Host      string `yaml:"host" env:"DB_HOST"`
	Port      int    `yaml:"port" env:"DB_PORT"`
	User      string `yaml:"user" env:"DB_USER"`
	Password  string `yaml:"password" env:"DB_PASSWORD"`
	DBName    string `yaml:"dbname" env:"DB_NAME"`
	SSLMode   string `yaml:"sslmode" env:"DB_SSL_MODE"`     // for postgres
	Charset   string `yaml:"charset" env:"DB_CHARSET"`      // for mysql
	ParseTime bool   `yaml:"parsetime" env:"DB_PARSE_TIME"` // for mysql
	Loc       string `yaml:"loc" env:"DB_LOC"`              // for mysql
	Dsn       string `yaml:"dsn" env:"DB_DSN"`              // 可选，优先级最高：如果非空，直接使用此值
	ShowLog   bool   `yaml:"showlog" env:"DB_SHOW_LOG"`     // 可选，优先级最高：如果非空，直接使用此值

	// 连接池配置
	MaxIdleConns    int           `yaml:"max_idle_conns" env:"DB_MAX_IDLE_CONNS"`         // 最大空闲连接数，默认10
	MaxOpenConns    int           `yaml:"max_open_conns" env:"DB_MAX_OPEN_CONNS"`         // 最大打开连接数，默认100
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" env:"DB_CONN_MAX_LIFETIME"`   // 连接最大生命周期，默认1小时
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" env:"DB_CONN_MAX_IDLE_TIME"` // 连接最大空闲时间，默认15分钟
}

func (cfg DB) BuildDSN() (string, error) {
	if cfg.Dsn != "" {
		return cfg.Dsn, nil
	}

	switch cfg.Type {
	case Sqlite:
		if cfg.DBName == "" {
			return "", errors.New("dbname is required for sqlite")
		}
		return cfg.DBName, nil
	case Postgres:
		ssl := cfg.SSLMode
		if ssl == "" {
			ssl = "disable"
		}
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, ssl), nil
	case MySQL:
		charset := cfg.Charset
		if charset == "" {
			charset = "utf8mb4"
		}
		parseTime := "false"
		if cfg.ParseTime {
			parseTime = "true"
		}
		loc := cfg.Loc
		if loc == "" {
			loc = "Local"
		}
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=%s&loc=%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName,
			charset, parseTime, loc), nil

	default:
		return "", errors.New("unsupported db type: " + string(cfg.Type))
	}
}
