package client

import (
	_ "embed"
)

//go:embed template/service_client.tpl
var TplServiceClient string

//go:embed template/operation.tpl
var TplOperation string

//go:embed template/options.tpl
var TplOptions string

