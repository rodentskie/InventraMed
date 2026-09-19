package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/rodentskiedev/go-libraries/lib/jwt"
	"go.uber.org/zap"

	"apps/api/internal/domain"
	"apps/api/pkg/response"
)

const bearerScheme = "Bearer"

type contextKey struct{}

// Auth returns a middleware that requires a valid access token in the
// "Authorization: Bearer <token>" header. On success the token's
// domain.AccountPayload is available through AccountFromContext. Every failure
// gets the same 401 so a caller can't tell a missing token from a bad one.
func Auth(secret []byte, log *zap.Logger) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			account, ok := authenticate(r, secret)
			if !ok {
				unauthorized(w, log)
				return
			}

			ctx := context.WithValue(r.Context(), contextKey{}, account)
			next(w, r.WithContext(ctx))
		}
	}
}

// AccountFromContext returns the account stored by Auth.
func AccountFromContext(ctx context.Context) (domain.AccountPayload, bool) {
	account, ok := ctx.Value(contextKey{}).(domain.AccountPayload)

	return account, ok
}

func authenticate(r *http.Request, secret []byte) (domain.AccountPayload, bool) {
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		return domain.AccountPayload{}, false
	}

	claims, err := jwt.ParseAccessToken[domain.AccountPayload](token, secret)
	if err != nil || claims.Payload.UserID == "" {
		return domain.AccountPayload{}, false
	}

	return claims.Payload, true
}

// bearerToken extracts the token from a "Bearer <token>" header value. The
// scheme is matched case-insensitively.
func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, bearerScheme) {
		return "", false
	}

	token = strings.TrimSpace(token)

	return token, token != ""
}

func unauthorized(w http.ResponseWriter, log *zap.Logger) {
	w.Header().Set("WWW-Authenticate", bearerScheme)

	if err := response.Error(w, http.StatusUnauthorized, "unauthorized"); err != nil {
		log.Error("write unauthorized response failed", zap.Error(err))
	}
}
