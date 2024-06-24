package middleware

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"github.com/shrewx/ginx/pkg/logx"
	"io"
	"net/http"
)

func Recovery() gin.HandlerFunc {
	writer := bytes.NewBuffer(nil)
	return gin.CustomRecoveryWithWriter(writer, func(c *gin.Context, err any) {
		if info, err := io.ReadAll(writer); err == nil {
			logx.Error(string(info))
		}
		c.AbortWithStatus(http.StatusInternalServerError)
	})
}
