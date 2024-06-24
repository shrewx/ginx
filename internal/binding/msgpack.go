// Copyright 2017 Manu Martinez-Almeida. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build !nomsgpack
// +build !nomsgpack

package binding

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"github.com/ugorji/go/codec"
	"io"
)

type msgpackBinding struct{}

func (msgpackBinding) Name() string {
	return "msgpack"
}

func (msgpackBinding) Bind(ctx *gin.Context, obj any) error {
	return decodeMsgPack(ctx.Request.Body, obj)
}

func (msgpackBinding) BindBody(body []byte, obj any) error {
	return decodeMsgPack(bytes.NewReader(body), obj)
}

func decodeMsgPack(r io.Reader, obj any) error {
	cdc := new(codec.MsgpackHandle)
	return codec.NewDecoder(r, cdc).Decode(&obj)
}
