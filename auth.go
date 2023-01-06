package csgo

import (
	"encoding/base64"
	"net/http"
)

type Accounts struct {
	UnAuthHandler func(ctx *Context)
	Users         map[string]string
}

// BasicAuth 中间件
func (a *Accounts) BasicAuth(next HandleFunc) HandleFunc {
	return func(ctx *Context) {
		//从ctx中获得用户名和密码并且验证
		username, password, ok := ctx.R.BasicAuth()
		if !ok {
			a.unAuthHandler(ctx)
			return
		}
		pwd, exist := a.Users[username]
		if !exist {
			a.unAuthHandler(ctx)
			return
		}
		if pwd != password {
			a.unAuthHandler(ctx)
			return
		}
		//如果认证通过那么，将用户名放入ctx中
		ctx.Set("user", username)
		next(ctx)
	}
}

func (a *Accounts) unAuthHandler(ctx *Context) {
	if a.UnAuthHandler != nil {
		a.UnAuthHandler(ctx)
	} else {
		ctx.W.WriteHeader(http.StatusUnauthorized)
	}
}

func BasicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
