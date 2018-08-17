package main

import (
	"net/http"
)

func use(h http.HandlerFunc, middleware ...func(http.HandlerFunc) http.HandlerFunc) http.HandlerFunc {
	for _, m := range middleware {
		h = m(h)
	}

	return h
}

func auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if *useBasicAuth == true {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)

			username, password, authOK := r.BasicAuth()
			if authOK == false {
				http.Error(w, "Not authorized", 401)
				return
			}

			if username != *basicAuthUsername || password != *basicAuthPassword {
				http.Error(w, "Not authorized", 401)
				return
			}
		}

		h.ServeHTTP(w, r)
	}
}
