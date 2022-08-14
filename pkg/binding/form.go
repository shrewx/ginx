// Copyright 2014 Manu Martinez-Almeida. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package binding

import (
	"errors"
	"github.com/gin-gonic/gin"
	"net/http"
)

const defaultMemory = 32 << 20

type formBinding struct{}
type formPostBinding struct{}
type formMultipartBinding struct{}

func (formBinding) Name() string {
	return "form"
}

func (formBinding) Bind(ctx *gin.Context, obj any) error {
	if err := ctx.Request.ParseForm(); err != nil {
		return err
	}
	if err := ctx.Request.ParseMultipartForm(defaultMemory); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		return err
	}
	if err := mapName(obj, ctx.Request.Form); err != nil {
		return err
	}
	return mappingNameByPtr(obj, (*MultipartRequest)(ctx.Request))
}

func (formPostBinding) Name() string {
	return "form-urlencoded"
}

func (formPostBinding) Bind(ctx *gin.Context, obj any) error {
	if err := ctx.Request.ParseForm(); err != nil {
		return err
	}
	return mapName(obj, ctx.Request.PostForm)
}

func (formMultipartBinding) Name() string {
	return "multipart/form-data"
}

func (formMultipartBinding) Bind(ctx *gin.Context, obj any) error {
	if err := ctx.Request.ParseMultipartForm(defaultMemory); err != nil {
		return err
	}
	return mappingNameByPtr(obj, (*MultipartRequest)(ctx.Request))
}
