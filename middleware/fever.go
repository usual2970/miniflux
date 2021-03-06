// Copyright 2018 Frédéric Guillot. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package middleware // import "miniflux.app/middleware"

import (
	"context"
	"net/http"

	"miniflux.app/http/request"
	"miniflux.app/http/response/json"
	"miniflux.app/logger"
)

// FeverAuth handles Fever API authentication.
func (m *Middleware) FeverAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.FormValue("api_key")

		user, err := m.store.UserByFeverToken(apiKey)
		if err != nil {
			logger.Error("[Middleware:Fever] %v", err)
			json.OK(w, r, map[string]int{"api_version": 3, "auth": 0})
			return
		}

		if user == nil {
			logger.Info("[Middleware:Fever] Fever authentication failure")
			json.OK(w, r, map[string]int{"api_version": 3, "auth": 0})
			return
		}

		logger.Info("[Middleware:Fever] User #%d is authenticated", user.ID)
		m.store.SetLastLogin(user.ID)

		ctx := r.Context()
		ctx = context.WithValue(ctx, request.UserIDContextKey, user.ID)
		ctx = context.WithValue(ctx, request.UserTimezoneContextKey, user.Timezone)
		ctx = context.WithValue(ctx, request.IsAdminUserContextKey, user.IsAdmin)
		ctx = context.WithValue(ctx, request.IsAuthenticatedContextKey, true)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
