// Copyright 2018 Gin Core Team. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package binding

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v2"
	"io"
)

type yamlBinding struct{}

func (yamlBinding) Name() string {
	return "yaml"
}

func (yamlBinding) Bind(ctx *gin.Context, obj any) error {
	return decodeYAML(ctx.Request.Body, obj)
}

func (yamlBinding) BindBody(body []byte, obj any) error {
	return decodeYAML(bytes.NewReader(body), obj)
}

func decodeYAML(r io.Reader, obj any) error {
	decoder := yaml.NewDecoder(r)
	return decoder.Decode(obj)
}
