package httpapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/auth"
)

type identityContextKey struct{}

func authenticate(tokens *auth.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			parts := strings.Fields(request.Header.Get("Authorization"))
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "A valid bearer token is required")
				return
			}
			identity, err := tokens.Parse(parts[1])
			if err != nil {
				writeError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "A valid bearer token is required")
				return
			}
			ctx := context.WithValue(request.Context(), identityContextKey{}, identity)
			next.ServeHTTP(response, request.WithContext(ctx))
		})
	}
}

func identityFromContext(ctx context.Context) (auth.Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(auth.Identity)
	return identity, ok
}
