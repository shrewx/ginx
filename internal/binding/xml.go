// Copyright 2014 Manu Martinez-Almeida. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package binding

import (
	"bytes"
	"encoding/xml"
	"github.com/gin-gonic/gin"
	"io"
)

type xmlBinding struct{}

func (xmlBinding) Name() string {
	return "xml"
}

func (xmlBinding) Bind(ctx *gin.Context, obj any) error {
	return decodeXML(ctx.Request.Body, obj)
}

func (xmlBinding) BindBody(body []byte, obj any) error {
	return decodeXML(bytes.NewReader(body), obj)
}
func decodeXML(r io.Reader, obj any) error {
	decoder := xml.NewDecoder(r)
	return decoder.Decode(obj)
}
