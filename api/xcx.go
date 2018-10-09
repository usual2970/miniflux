package api

import (
	"miniflux.app/http/response/json"
	"miniflux.app/logger"
	"miniflux.app/middleware"
	"miniflux.app/model"
	"net/http"
)

func (c *Controller) XcxLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	xcxObject := ctx.Value("xcxObject").(*middleware.XcxObject)

	user, err := c.store.UserByUsername(xcxObject.Openid)
	if err != nil {
		logger.Error("[Controller:XcxLogin] %v", err)
		json.ServerError(w, err)
		return
	}

	if user == nil {
		newUser := model.NewUser()
		newUser.Username = xcxObject.Openid
		newUser.IsAdmin = false
		newUser.Extra["unionid"] = xcxObject.Unionid
		newUser.Extra["openid"] = xcxObject.Openid
		err := c.store.CreateUser(newUser)

		if err != nil {
			logger.Error("[Controller:XcxLogin] %v", err)
			json.ServerError(w, err)
			return
		}
	}
	json.OK(w, r, xcxObject)

}
