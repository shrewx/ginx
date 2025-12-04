package ginx

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type MethodGet struct{}

func (m *MethodGet) Method() string                  { return http.MethodGet }
func (m *MethodGet) Validate(ctx *gin.Context) error { return nil }

type MethodPost struct{}

func (m *MethodPost) Method() string                  { return http.MethodPost }
func (m *MethodPost) Validate(ctx *gin.Context) error { return nil }

type MethodPut struct{}

func (m *MethodPut) Method() string                  { return http.MethodPut }
func (m *MethodPut) Validate(ctx *gin.Context) error { return nil }

type MethodDelete struct{}

func (m *MethodDelete) Method() string                  { return http.MethodDelete }
func (m *MethodDelete) Validate(ctx *gin.Context) error { return nil }

type MethodOptions struct{}

func (m *MethodOptions) Method() string                  { return http.MethodOptions }
func (m *MethodOptions) Validate(ctx *gin.Context) error { return nil }

type MethodPatch struct{}

func (m *MethodPatch) Method() string                  { return http.MethodPatch }
func (m *MethodPatch) Validate(ctx *gin.Context) error { return nil }
