// Copyright 2014 Manu Martinez-Almeida. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package binding

import (
	"errors"
	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
	"io/ioutil"
)

type protobufBinding struct{}

func (protobufBinding) Name() string {
	return "protobuf"
}

func (b protobufBinding) Bind(ctx *gin.Context, obj any) error {
	buf, err := ioutil.ReadAll(ctx.Request.Body)
	if err != nil {
		return err
	}
	return b.BindBody(buf, obj)
}

func (protobufBinding) BindBody(body []byte, obj any) error {
	msg, ok := obj.(proto.Message)
	if !ok {
		return errors.New("obj is not ProtoMessage")
	}
	if err := proto.Unmarshal(body, msg); err != nil {
		return err
	}
	// Here it's same to return validate(obj), but util now we can't add
	// `binding:""` to the struct which automatically generate by cmd-proto
	return nil
	// return validate(obj)
}
