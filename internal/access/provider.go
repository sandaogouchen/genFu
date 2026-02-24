package access

import (
	"context"
	"net/http"
	"strings"

	sdkaccess "github.com/router-for-me/CLIProxyAPI/v6/sdk/access"
)

type APIKeyProvider struct {
	keys map[string]struct{}
}

func NewAPIKeyProvider(keys []string) *APIKeyProvider {
	keyMap := map[string]struct{}{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		keyMap[key] = struct{}{}
	}
	return &APIKeyProvider{keys: keyMap}
}

func (p *APIKeyProvider) Identifier() string {
	return "config-api-key"
}

func (p *APIKeyProvider) Authenticate(ctx context.Context, r *http.Request) (*sdkaccess.Result, *sdkaccess.AuthError) {
	_ = ctx
	token, source := extractToken(r)
	if token == "" {
		return nil, sdkaccess.NewNoCredentialsError()
	}
	if _, ok := p.keys[token]; !ok {
		return nil, sdkaccess.NewInvalidCredentialError()
	}
	return &sdkaccess.Result{
		Provider:  p.Identifier(),
		Principal: "api-key",
		Metadata:  map[string]string{"source": source},
	}, nil
}

func extractToken(r *http.Request) (string, string) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:]), "authorization"
	}
	if v := strings.TrimSpace(r.Header.Get("X-Goog-Api-Key")); v != "" {
		return v, "x-goog-api-key"
	}
	if v := strings.TrimSpace(r.Header.Get("X-Api-Key")); v != "" {
		return v, "x-api-key"
	}
	query := r.URL.Query()
	if v := strings.TrimSpace(query.Get("key")); v != "" {
		return v, "query-key"
	}
	if v := strings.TrimSpace(query.Get("auth_token")); v != "" {
		return v, "query-auth-token"
	}
	return "", ""
}
