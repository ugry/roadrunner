# Insucar — Production Security Checklist
Date: 2026-07-05 | One-page reference for final deployment sign-off

---

## ✅ PRE-DEPLOY (must be complete before PROD)

### Authentication
- [ ] **Password hashing:** Replace SHA-256 with bcrypt (cost 12+) or argon2id
- [ ] **Session secret:** Rotate `SESSION_SECRET` from hardcoded default to 64-char random hex
- [ ] **Webhook secret:** Rotate `WEBHOOK_SECRET` from hardcoded default
- [ ] **Cognito MFA:** Enable TOTP MFA for staff/operator pool
- [ ] **Cognito advanced security:** Enable adaptive auth + compromised credential detection
- [ ] **Token lifetimes:** Set staff access token to 30 min, refresh to 1 hour
- [ ] **Account lockout:** Implement (or enable Cognito) after 5 failed attempts

### Data Protection
- [ ] **Database:** Migrate from in-cluster Postgres to RDS Multi-AZ with encryption at rest
- [ ] **TLS:** Ensure RDS uses `sslrootcert=rds-ca-2019-root.pem`
- [ ] **Secrets:** All credentials in `insucar-app` Secret (not ConfigMap, not hardcoded)
- [ ] **IRSA:** Wire ServiceAccount IRSA roles (no static IAM keys in pods)
- [ ] **S3 encryption:** Enable SSE-S3 or SSE-KMS on Spinnaker S3 bucket

### Code Quality
- [ ] **Test coverage:** Minimum 30% on handler/auth/connector files (currently 1.0%)
- [ ] **govulncheck:** Zero HIGH/CRITICAL findings (blocking in Jenkins)
- [ ] **gosec:** Zero HIGH severity findings (blocking in Jenkins)
- [ ] **Dependency update:** Update golang.org/x/net (v0.10.0→v0.56.0) and x/crypto (v0.17.0→v0.53.0)
- [ ] **go vet:** Pass clean (confirmed ✅)
- [ ] **Trivy scan:** Zero CRITICAL CVEs on final image

### RLS / Multi-Tenant
- [ ] **Fix SQL injection:** Replace `fmt.Sprintf("SET LOCAL app.current_tenant = '%s'", tid)` with parameterized or validated input (tenant.go:114)
- [ ] **Fix connection pinning:** Ensure SET LOCAL persists across pgxpool connections for the request duration
- [ ] **Tenant isolation test:** Verify agent from tenant-A cannot see tenant-B cases

### Network & Infrastructure
- [ ] **Pod security:** Set `securityContext: { runAsNonRoot: true, readOnlyRootFilesystem: true }`
- [ ] **NetworkPolicy:** Restrict egress from API pod to only RDS, ElastiCache, SNS, Cognito
- [ ] **Ingress:** Enforce HTTP→HTTPS redirect (confirmed ✅)
- [ ] **TLS:** Minimum TLS 1.2, HSTS header (max-age=31536000; includeSubDomains; preload)
- [ ] **WAF:** Deploy AWS WAF with OWASP Top 10 rule set in front of ALB/CloudFront
- [ ] **DDoS:** Enable AWS Shield Advanced if budget allows, at minimum Shield Standard

### CI/CD
- [ ] **Image signing:** Enable cosign signing in Jenkins pipeline
- [ ] **SBOM verification:** Verify SBOM integrity before deploy
- [ ] **Immutable tags:** Use SHA-based image tags (not `:latest`)

---

## 🟡 POST-DEPLOY (within 30 days of PROD)

### Monitoring
- [ ] **CloudWatch alarms:** API 5xx rate, latency p95, DB connection count, memory OOM
- [ ] **Auth monitoring:** Alert on unusual login patterns (geo, time, frequency)
- [ ] **Audit logging:** Ship structured JSON logs to CloudWatch Logs Insights
- [ ] **SNS alerting:** Configure SNS→email/Slack for P0 alarms

### Operational
- [ ] **Backup verification:** Test RDS automated backup restore within 2 hours
- [ ] **DR runbook:** Document database failover procedure (Multi-AZ automatic, but test manual)
- [ ] **Key rotation plan:** KMS keys rotated annually, session secrets quarterly
- [ ] **Certificate renewal:** Monitor ACM cert expiry (90-day LetsEncrypt auto-renew confirmed ✅)

### Compliance
- [ ] **GDPR:** Data retention policy documented, right-to-erasure process tested
- [ ] **Cookie consent:** Add cookie consent banner for non-essential cookies
- [ ] **Privacy policy:** Published at unysolar.com/privacy
- [ ] **Terms of service:** Published at unysolar.com/terms

---

## 🔴 NEVER DEPLOY TO PROD WITHOUT

1. ✅ RDS Multi-AZ (not in-cluster single pod)
2. ✅ Session/Webhook secrets rotated from defaults
3. ✅ Password storage with bcrypt (not SHA-256)
4. ✅ SQL injection fix in tenant middleware
5. ✅ All CRITICAL trivy findings resolved
6. ✅ Basic auth tests passing (login, register, dispatch)
7. ✅ TLS validated (grade A+ on SSL Labs)
8. ✅ Backup restore tested

---

## Quick Validation Commands

```bash
# Check TLS
curl -sI https://unysolar.com | grep -i 'strict-transport-security\|server'

# Check secure cookies
curl -sv https://unysolar.com/api/me 2>&1 | grep -i 'set-cookie'

# Check RLS isolation
kubectl exec -it deploy/insucar-api -n insucar-prod -- \
  curl -s -H "Host: tenant-b.unysolar.com" http://localhost:8080/api/agent/cases | jq

# Check dependency CVEs
trivy image --scanners vuln --severity CRITICAL \
  326804802908.dkr.ecr.eu-west-1.amazonaws.com/insucar-api:latest
```
