package dbhelper

import (
	"database/sql/driver"
	"strconv"
	"strings"
)

// DBStringArray 支持数据库的字符串数组
type DBStringArray []string

func (d *DBStringArray) Value() (driver.Value, error) {
	return strings.Join(*d, ","), nil
}

func (d *DBStringArray) Scan(value interface{}) error {
	if value == nil || value.(string) == "" {
		return nil
	}
	*d = strings.Split(value.(string), ",")
	return nil
}

// DBUInt64Array 支持数据库的 uint64 数组
type DBUInt64Array []uint64

func (d *DBUInt64Array) Value() (driver.Value, error) {
	var s []string
	for _, v := range *d {
		s = append(s, strconv.FormatUint(v, 10))
	}
	return strings.Join(s, ","), nil
}

func (d *DBUInt64Array) Scan(value interface{}) error {
	if value == nil || value.(string) == "" {
		return nil
	}
	strs := strings.Split(value.(string), ",")
	for _, s := range strs {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return err
		}
		*d = append(*d, v)
	}
	return nil
}
