// Amazon Cognito token verification (managed auth).
// Validates RS256 access/id tokens against the pool JWKS, checks issuer,
// token_use and client_id/aud, and extracts cognito:groups for RBAC.
// Coexists with the demo cookie session: if COGNITO_ISSUER is unset the
// verifier is nil and the app falls back to cookie auth.
package main

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var cognito *cognitoVerifier

type cognitoVerifier struct {
	issuer    string
	jwksURL   string
	clientIDs map[string]bool

	mu      sync.RWMutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

func newCognitoVerifier() *cognitoVerifier {
	iss := strings.TrimRight(os.Getenv("COGNITO_ISSUER"), "/") // https://cognito-idp.<region>.amazonaws.com/<poolId>
	if iss == "" {
		return nil
	}
	v := &cognitoVerifier{
		issuer:    iss,
		jwksURL:   iss + "/.well-known/jwks.json",
		clientIDs: map[string]bool{},
		keys:      map[string]*rsa.PublicKey{},
	}
	for _, c := range strings.Split(os.Getenv("COGNITO_CLIENT_IDS"), ",") {
		if c = strings.TrimSpace(c); c != "" {
			v.clientIDs[c] = true
		}
	}
	return v
}

type jwksDoc struct {
	Keys []struct {
		Kid string `json:"kid"`
		Kty string `json:"kty"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

func (v *cognitoVerifier) refresh(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", v.jwksURL, nil)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var doc jwksDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		nb, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eb, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: int(new(big.Int).SetBytes(eb).Int64())}
	}
	if len(keys) == 0 {
		return errors.New("jwks: no usable keys")
	}
	v.mu.Lock()
	v.keys, v.fetched = keys, time.Now()
	v.mu.Unlock()
	return nil
}

func (v *cognitoVerifier) keyfunc(t *jwt.Token) (interface{}, error) {
	if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
		return nil, errors.New("unexpected signing method")
	}
	kid, _ := t.Header["kid"].(string)
	v.mu.RLock()
	k := v.keys[kid]
	stale := time.Since(v.fetched) > time.Hour
	v.mu.RUnlock()
	if k == nil || stale {
		if err := v.refresh(context.Background()); err != nil && k == nil {
			return nil, err
		}
		v.mu.RLock()
		k = v.keys[kid]
		v.mu.RUnlock()
	}
	if k == nil {
		return nil, errors.New("unknown kid")
	}
	return k, nil
}

type cognitoIdentity struct {
	Subject  string
	Username string
	Email    string
	Groups   []string
}

func (v *cognitoVerifier) verify(tokenStr string) (*cognitoIdentity, error) {
	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(tokenStr, claims, v.keyfunc,
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.issuer),
	); err != nil {
		return nil, err
	}
	if tu, _ := claims["token_use"].(string); tu != "access" && tu != "id" {
		return nil, errors.New("bad token_use")
	}
	if len(v.clientIDs) > 0 {
		cid, _ := claims["client_id"].(string)
		aud, _ := claims["aud"].(string)
		if !v.clientIDs[cid] && !v.clientIDs[aud] {
			return nil, errors.New("client_id/aud not allowed")
		}
	}
	id := &cognitoIdentity{}
	id.Subject, _ = claims["sub"].(string)
	if id.Username, _ = claims["username"].(string); id.Username == "" {
		id.Username, _ = claims["cognito:username"].(string)
	}
	id.Email, _ = claims["email"].(string)
	if raw, ok := claims["cognito:groups"].([]interface{}); ok {
		for _, g := range raw {
			if gs, ok := g.(string); ok {
				id.Groups = append(id.Groups, gs)
			}
		}
	}
	return id, nil
}

func bearerIdentity(r *http.Request) (*cognitoIdentity, bool) {
	if cognito == nil {
		return nil, false
	}
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return nil, false
	}
	id, err := cognito.verify(strings.TrimSpace(strings.TrimPrefix(h, "Bearer ")))
	if err != nil {
		return nil, false
	}
	return id, true
}

var staffGroups = map[string]bool{
	"operator": true, "supervisor": true, "admin": true, "ops": true, "product_owner": true,
}

// roleFromGroups maps Cognito groups to the app's coarse role.
func roleFromGroups(groups []string) string {
	for _, g := range groups {
		if staffGroups[g] {
			return "agent"
		}
	}
	return "user"
}
