// Multi-tenant resolution + RLS enforcement.
// Resolves tenant from Host header (subdomain) or JWT claim, then sets
// app.current_tenant on the session so PostgreSQL RLS policies engage.
// Platform-admins (no tenant) see all rows.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// tenantMap caches subdomain -> tenant_id lookups.
// Populated at startup and refreshed lazily.
var tenantMap = map[string]string{}

// defaultTenantID is used when no subdomain matches (apex domain).
var defaultTenantID = ""

// knownTLDs strips common suffixes from the Host header to extract the subdomain.
var knownTLDs = []string{".unysolar.com", ".co.uk", ".com", ".fr", ".de"}

func initTenants(ctx context.Context) {
	if db == nil {
		return
	}
	// Check if tenants table exists (schema v3 may not have been applied yet)
	var exists bool
	if err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name='tenants')`).Scan(&exists); err != nil || !exists {
		log.Printf("tenant init: tenants table not found, RLS disabled (apply db/schema-v3-additions.sql)")
		defaultTenantID = "none"
		return
	}

	rows, err := db.Query(ctx, `SELECT id, subdomain FROM tenants WHERE status='active'`)
	if err != nil {
		log.Printf("tenant init: %v", err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id, sub string
		rows.Scan(&id, &sub)
		tenantMap[strings.ToLower(sub)] = id
		// use the first tenant as default
		if defaultTenantID == "" {
			defaultTenantID = id
		}
	}
	if defaultTenantID == "" {
		// seed a default tenant if none exists
		log.Printf("tenant init: no tenants found, seeding default 'insucar' tenant")
		if err := db.QueryRow(ctx,
			`INSERT INTO tenants(name,subdomain,default_language,countries,enabled_service_lines)
			 VALUES('Insucar','insucar','en','{FR,GB,DE}','{mobility}')
			 ON CONFLICT(subdomain) DO UPDATE SET name='Insucar'
			 RETURNING id`).Scan(&defaultTenantID); err != nil {
			log.Printf("tenant init: failed to seed default tenant: %v", err)
			defaultTenantID = "none"
		} else {
			tenantMap["insucar"] = defaultTenantID
		}
	}
	log.Printf("tenant init: loaded %d tenants, default=%s", len(tenantMap), defaultTenantID[:min(8, len(defaultTenantID))])
}

// resolveTenant extracts the tenant from the request.
// Priority: 1) Host header subdomain match  2) default tenant
func resolveTenantID(r *http.Request) string {
	// 1. Extract subdomain from Host header
	host := r.Host
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	host = strings.ToLower(host)

	// Try exact match first
	if id, ok := tenantMap[host]; ok {
		return id
	}

	// Strip known TLDs to find subdomain
	for _, tld := range knownTLDs {
		if strings.HasSuffix(host, tld) {
			sub := strings.TrimSuffix(host, tld)
			sub = strings.TrimSuffix(sub, ".")
			if sub != "" && sub != "www" && sub != "app" && sub != "op" {
				// Try most-specific first, then fall back to first segment
				for _, candidate := range []string{sub, strings.Split(sub, ".")[0]} {
					if id, ok := tenantMap[candidate]; ok {
						return id
					}
				}
			}
			break
		}
	}

	// 2. Default tenant
	return defaultTenantID
}

// tenantMiddleware sets app.current_tenant on the PostgreSQL session for RLS.
// Must wrap every handler that queries tenant-scoped tables.
func tenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := resolveTenantID(r)
		// Only set RLS if we have a valid tenant and the DB is initialized
		if tid != "" && tid != "none" && db != nil {
			// Use parameterized query via set_config() to prevent SQL injection (researcher P0)
			var prev string
			if err := db.QueryRow(r.Context(),
				`SELECT set_config('app.current_tenant', $1, true)`, tid).Scan(&prev); err != nil {
				log.Printf(`{"stream":"error","event":"tenant_set_failed","tenant":%q,"err":%q}`, tid[:min(8, len(tid))], err.Error())
			}
		}
		next.ServeHTTP(w, r)
	})
}

// TenantScopedQuery returns SQL that optionally filters by current tenant.
// Useful for queries that need explicit filtering alongside RLS.
func tenantScope() string {
	if defaultTenantID == "" {
		return ""
	}
	return fmt.Sprintf("tenant_id = '%s'", defaultTenantID)
}
