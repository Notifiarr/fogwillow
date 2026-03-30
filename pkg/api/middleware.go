package api

import "net/http"

// authenticate validates the X-Api-Key header against the configured password.
// When password is empty, all requests are allowed through.
func (a *API) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if a.password != "" && req.Header.Get("X-Api-Key") != a.password {
			writeError(resp, http.StatusUnauthorized, "invalid or missing X-Api-Key")
			return
		}

		next.ServeHTTP(resp, req)
	})
}
