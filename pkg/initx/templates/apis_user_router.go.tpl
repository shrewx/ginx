package user

import "github.com/shrewx/ginx"

var Router = ginx.NewRouter(ginx.Group("/users"))

func init() {
	Router.Register(&CreateUser{})
	Router.Register(&GetUser{})
	Router.Register(&UpdateUser{})
	Router.Register(&DeleteUser{})
	Router.Register(&ListUsers{})
}

