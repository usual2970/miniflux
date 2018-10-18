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
		user = model.NewUser()
		user.Username = xcxObject.Openid
		user.IsAdmin = false
		user.Extra["unionid"] = xcxObject.Unionid
		user.Extra["openid"] = xcxObject.Openid
		err := c.store.CreateUser(user)

		if err != nil {
			logger.Error("[Controller:XcxLogin] %v", err)
			json.ServerError(w, err)
			return
		}

		cate, err := c.store.CategoryByTitle(user.ID, "All")
		if err != nil || cate == nil {
			logger.Error("[Controller:XcxLogin] %v", err)
			json.ServerError(w, err)
			return
		}

		c.feedHandler.CreateFeed(user.ID, cate.ID, "http://www.zhihu.com/rss", false, "", "", "")

	}



	json.OK(w, r, xcxObject)

}
