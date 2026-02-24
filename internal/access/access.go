package access

import (
	"context"
	"net/http"
	"strings"

	sdkaccess "github.com/router-for-me/CLIProxyAPI/v6/sdk/access"
)

func NewManager(keys []string) *sdkaccess.Manager {
	manager := sdkaccess.NewManager()
	provider := NewAPIKeyProvider(keys)
	manager.SetProviders([]sdkaccess.Provider{provider})
	return manager
}

func WrapHTTP(next http.Handler, manager *sdkaccess.Manager, allowPaths []string) http.Handler {
	if manager == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isAllowedPath(r.URL.Path, allowPaths) {
			next.ServeHTTP(w, r)
			return
		}
		_, authErr := manager.Authenticate(r.Context(), r)
		if authErr == nil {
			next.ServeHTTP(w, r)
			return
		}
		if sdkaccess.IsAuthErrorCode(authErr, sdkaccess.AuthErrorCodeNoCredentials) || sdkaccess.IsAuthErrorCode(authErr, sdkaccess.AuthErrorCodeInvalidCredential) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	})
}

func isAllowedPath(path string, allowPaths []string) bool {
	for _, p := range allowPaths {
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(path, prefix) {
				return true
			}
			continue
		}
		if path == p {
			return true
		}
	}
	return false
}

func Authenticate(ctx context.Context, manager *sdkaccess.Manager, r *http.Request) (*sdkaccess.Result, *sdkaccess.AuthError) {
	if manager == nil {
		return nil, nil
	}
	return manager.Authenticate(ctx, r)
}
