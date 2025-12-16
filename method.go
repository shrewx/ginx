package ginx

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

type MethodGet struct{}

func (m *MethodGet) Method() string                  { return http.MethodGet }
func (m *MethodGet) Validate(ctx *gin.Context) error { return nil }
func (m *MethodGet) Path() string                    { return "" }

type MethodPost struct{}

func (m *MethodPost) Method() string                  { return http.MethodPost }
func (m *MethodPost) Validate(ctx *gin.Context) error { return nil }
func (m *MethodPost) Path() string                    { return "" }

type MethodPut struct{}

func (m *MethodPut) Method() string                  { return http.MethodPut }
func (m *MethodPut) Validate(ctx *gin.Context) error { return nil }
func (m *MethodPut) Path() string                    { return "" }

type MethodDelete struct{}

func (m *MethodDelete) Method() string                  { return http.MethodDelete }
func (m *MethodDelete) Validate(ctx *gin.Context) error { return nil }
func (m *MethodDelete) Path() string                    { return "" }

type MethodOptions struct{}

func (m *MethodOptions) Method() string                  { return http.MethodOptions }
func (m *MethodOptions) Validate(ctx *gin.Context) error { return nil }
func (m *MethodOptions) Path() string                    { return "" }

type MethodPatch struct{}

func (m *MethodPatch) Method() string                  { return http.MethodPatch }
func (m *MethodPatch) Validate(ctx *gin.Context) error { return nil }
func (m *MethodPatch) Path() string                    { return "" }
