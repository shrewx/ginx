// Copyright 2017 Manu Martinez-Almeida. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package binding

import (
	"github.com/gin-gonic/gin"
)

type queryBinding struct{}

func (queryBinding) Name() string {
	return "query"
}

func (queryBinding) Bind(ctx *gin.Context, obj any) error {
	values := ctx.Request.URL.Query()
	return mapName(obj, values)
}
