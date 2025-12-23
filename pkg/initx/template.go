package initx

import (
	_ "embed"
)

//go:embed templates/apis_root.go.tpl
var ApisRootTemplate string

//go:embed templates/global_config.go.tpl
var GlobalConfigTemplate string

//go:embed templates/cmd_main.go.tpl
var CmdMainTemplate string

//go:embed templates/constants_error.go.tpl
var ConstantsErrorTemplate string

//go:embed templates/local_config.yaml.tpl
var LocalConfigTemplate string

//go:embed templates/repositories_models_user.go.tpl
var RepositoriesModelsUserTemplate string

//go:embed templates/repositories_controllers_user_controller.go.tpl
var RepositoriesControllersUserControllerTemplate string

//go:embed templates/services_user_service.go.tpl
var ServicesUserServiceTemplate string

//go:embed templates/apis_user_router.go.tpl
var ApisUserRouterTemplate string

//go:embed templates/apis_user_create_user.go.tpl
var ApisUserCreateUserTemplate string

//go:embed templates/apis_user_get_user.go.tpl
var ApisUserGetUserTemplate string

//go:embed templates/apis_user_update_user.go.tpl
var ApisUserUpdateUserTemplate string

//go:embed templates/apis_user_delete_user.go.tpl
var ApisUserDeleteUserTemplate string

//go:embed templates/apis_user_list_users.go.tpl
var ApisUserListUsersTemplate string
