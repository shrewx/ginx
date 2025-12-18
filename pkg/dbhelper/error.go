package dbhelper

import (
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"strings"
)

func IsDuplicateError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}

	// ===== 唯一性错误关键词（兜底）=====
	msg := err.Error()
	if strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "Duplicate entry") ||
		strings.Contains(msg, "UNIQUE constraint failed") {
		return true
	}

	return false
}
