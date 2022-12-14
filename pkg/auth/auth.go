package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-kit/log"
	"github.com/ricoberger/script_exporter/pkg/config"

	jwt "github.com/golang-jwt/jwt/v4"
)

func Auth(h http.Handler, exporterConfig config.Config, logger log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Basic authentication
		if exporterConfig.BasicAuth.Enabled {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)

			username, password, authOK := r.BasicAuth()
			if !authOK {
				http.Error(w, "Not authorized", http.StatusUnauthorized)
				return
			}

			if username != exporterConfig.BasicAuth.Username || password != exporterConfig.BasicAuth.Password {
				http.Error(w, "Not authorized", http.StatusUnauthorized)
				return
			}
		}

		// Authentication using bearer token
		if exporterConfig.BearerAuth.Enabled {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Not authorized", http.StatusUnauthorized)
				return
			}

			authHeaderParts := strings.Split(authHeader, " ")
			if len(authHeaderParts) != 2 || strings.ToLower(authHeaderParts[0]) != "bearer" {
				http.Error(w, "Not authorized", http.StatusUnauthorized)
				return
			}

			err := checkJWT(authHeaderParts[1], exporterConfig)
			if err != nil {
				http.Error(w, "Not authorized", http.StatusUnauthorized)
				return
			}
		}

		h.ServeHTTP(w, r)
	})
}

// CheckJWT validates jwt tokens
func checkJWT(jwtToken string, exporterConfig config.Config) error {
	token, err := jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(exporterConfig.BearerAuth.SigningKey), nil
	})

	if err != nil {
		return err
	}

	if _, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return nil
	}

	return errors.New("not authorized")
}

// createJWT creates jwt tokens
func CreateJWT(exporterConfig config.Config) (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	tokenString, err := token.SignedString([]byte(exporterConfig.BearerAuth.SigningKey))
	return tokenString, err
}
