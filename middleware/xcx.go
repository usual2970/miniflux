package middleware

import (
	"context"
	json2 "encoding/json"
	"fmt"
	"miniflux.app/http/client"
	"miniflux.app/http/request"
	"miniflux.app/http/response/json"
	"miniflux.app/logger"
	"net/http"
)

type XcxObject struct {
	Openid     string `json:"openid"`
	SessionKey string `json:"session_key"`
	Unionid    string `json:"unionid"`
	Errcode    int    `json:"errcode"`
	ErrMsg     string `json:"errMsg"`
}

func (m *Middleware) Xcx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		code := request.QueryStringParam(r, "code", "")
		if code == "" {

			logger.Debug("[Middleware:Xcx] No code sent")
			json.Unauthorized(w)
			return
		}

		appId := m.cfg.XcxId()
		appSecret := m.cfg.XcxSecret()
		if appId == "" || appSecret == "" {
			logger.Debug("[Middleware:Xcx] No xcxid or xcxsecret setted")
			json.Unauthorized(w)
			return
		}

		xcxObject, err := code2Session(fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code", appId, appSecret, code))
		if err != nil {
			logger.Debug("[Middleware:Xcx] %v", err)
			json.Unauthorized(w)
			return
		}

		if xcxObject.Errcode != 0 {
			logger.Debug("[Middleware:Xcx] %s", xcxObject.ErrMsg)
			json.Unauthorized(w)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, "xcxObject", xcxObject)

		next.ServeHTTP(w, r.WithContext(ctx))
	})

}

func code2Session(url string) (*XcxObject, error) {
	cli := client.New(url)
	res, err := cli.Get()
	if err != nil {
		return nil, err
	}

	xcxObject := new(XcxObject)
	decoder := json2.NewDecoder(res.Body)
	if err := decoder.Decode(xcxObject); err != nil {
		return nil, err
	}

	return xcxObject, nil
}
