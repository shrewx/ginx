// Copyright 2022 Gin Core Team. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package binding

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml/v2"
	"io"
)

type tomlBinding struct{}

func (tomlBinding) Name() string {
	return "toml"
}

func decodeToml(r io.Reader, obj any) error {
	decoder := toml.NewDecoder(r)
	if err := decoder.Decode(obj); err != nil {
		return err
	}
	return decoder.Decode(obj)
}

func (tomlBinding) Bind(ctx *gin.Context, obj any) error {
	return decodeToml(ctx.Request.Body, obj)
}

func (tomlBinding) BindBody(body []byte, obj any) error {
	return decodeToml(bytes.NewReader(body), obj)
}
