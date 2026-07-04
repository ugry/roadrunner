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

// ---------- unified caller identity (Cognito bearer OR demo cookie) ----------

type ctxKey string

const callerKey ctxKey = "insucar.caller"

type caller struct {
	Role string // "user" | "agent"
	ID   string // customers.id / staff.id (app-native id)
	Name string
}

func withCaller(r *http.Request, c *caller) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), callerKey, c))
}

func callerFrom(r *http.Request) (*caller, bool) {
	c, ok := r.Context().Value(callerKey).(*caller)
	return c, ok
}

// resolveCaller establishes identity from a Cognito bearer token (preferred,
// mapping the token 'sub' to an app-native id) or the demo cookie session.
func resolveCaller(r *http.Request) (*caller, bool) {
	if id, ok := bearerIdentity(r); ok {
		role := roleFromGroups(id.Groups)
		c := &caller{Role: role, ID: id.Subject, Name: nzName(id.Username, id.Email)}
		if role == "user" {
			if cid, err := ensureCustomerFromCognito(r.Context(), id); err == nil {
				c.ID = cid
			}
		} else {
			var sid, dn string
			if db.QueryRow(r.Context(), `SELECT id,display_name FROM staff WHERE cognito_subject=$1`, id.Subject).Scan(&sid, &dn) == nil {
				c.ID, c.Name = sid, dn
			}
		}
		return c, true
	}
	if role, id, name, ok := currentSession(r); ok {
		return &caller{Role: role, ID: id, Name: name}, true
	}
	return nil, false
}

// ensureCustomerFromCognito links or JIT-provisions a customer for a Cognito sub.
func ensureCustomerFromCognito(ctx context.Context, id *cognitoIdentity) (string, error) {
	var cid string
	if err := db.QueryRow(ctx, `SELECT id FROM customers WHERE cognito_subject=$1`, id.Subject).Scan(&cid); err == nil {
		return cid, nil
	}
	// Link an existing (e.g. pre-registered) customer by email.
	if id.Email != "" {
		if err := db.QueryRow(ctx,
			`UPDATE customers SET cognito_subject=$1, updated_at=now() WHERE email=$2 AND cognito_subject IS NULL RETURNING id`,
			id.Subject, id.Email).Scan(&cid); err == nil {
			return cid, nil
		}
	}
	// Just-in-time provision a new customer on first Cognito login.
	email := id.Email
	if email == "" {
		email = id.Subject + "@cognito.local"
	}
	first, last := splitName(id.Username, email)
	err := db.QueryRow(ctx, `
		INSERT INTO customers(email,first_name,last_name,status,email_verified,cognito_subject)
		VALUES($1,$2,$3,'active',true,$4)
		ON CONFLICT (email) DO UPDATE SET cognito_subject=EXCLUDED.cognito_subject, updated_at=now()
		RETURNING id`, email, first, last, id.Subject).Scan(&cid)
	return cid, err
}

func splitName(username, email string) (string, string) {
	base := username
	if base == "" {
		base = strings.Split(email, "@")[0]
	}
	parts := strings.Fields(strings.ReplaceAll(base, ".", " "))
	switch {
	case len(parts) >= 2:
		return parts[0], strings.Join(parts[1:], " ")
	case len(parts) == 1:
		return parts[0], ""
	default:
		return "user", ""
	}
}

func nzName(username, email string) string {
	if username != "" {
		return username
	}
	return email
}
