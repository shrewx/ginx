package ginx

import "net/http"

type MethodGet struct{}

func (m *MethodGet) Method() string { return http.MethodGet }

type MethodPost struct{}

func (m *MethodPost) Method() string { return http.MethodPost }

type MethodPut struct{}

func (m *MethodPut) Method() string { return http.MethodPut }

type MethodDelete struct{}

func (m *MethodDelete) Method() string { return http.MethodDelete }

type MethodOptions struct{}

func (m *MethodOptions) Method() string { return http.MethodOptions }

type MethodPatch struct{}

func (m *MethodPatch) Method() string { return http.MethodPatch }
