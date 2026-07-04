#!/bin/bash
# Insucar Cognito provisioning — creates 3 user pools + app clients via AWS CLI.
# Idempotent: pass --destroy to tear down.
# Requires: awscli v2, jq.  Run:  bash scripts/cognito-setup.sh [--destroy]

set -euo pipefail
REGION="${AWS_REGION:-eu-west-1}"
ENV="${ENVIRONMENT:-dev}"
ACCOUNT=$(aws sts get-caller-identity --query Account --output text 2>/dev/null || echo "unknown")
DESTROY=false
[[ "${1:-}" == "--destroy" ]] && DESTROY=true

info() { echo -e "\033[1;32m>\033[0m $*"; }
warn() { echo -e "\033[1;33m!\033[0m $*"; }

# --- Helpers ---
pool_id_by_name() {
  aws cognito-idp list-user-pools --max-results 50 --region "$REGION" \
    --query "UserPools[?Name=='$1'].Id" --output text 2>/dev/null || echo ""
}

client_id_by_name() {
  local pid="$1" name="$2"
  aws cognito-idp list-user-pool-clients --user-pool-id "$pid" --max-results 20 --region "$REGION" \
    --query "UserPools[?ClientName=='$1']" 2>/dev/null || true
}

# --- Destroy mode ---
if $DESTROY; then
  for pool_name in "insucar-${ENV}-customer" "insucar-${ENV}-staff" "insucar-${ENV}-partner"; do
    pid=$(pool_id_by_name "$pool_name")
    if [[ -n "$pid" ]]; then
      info "Deleting pool $pool_name ($pid)..."
      aws cognito-idp delete-user-pool --user-pool-id "$pid" --region "$REGION" || true
    fi
  done
  info "Cognito pools destroyed."
  exit 0
fi

# --- Create / idempotent-ensure pools ---

# CUSTOMER pool
CUSTOMER_POOL_ID=$(pool_id_by_name "insucar-${ENV}-customer")
if [[ -z "$CUSTOMER_POOL_ID" ]]; then
  info "Creating customer user pool..."
  CUSTOMER_POOL_ID=$(aws cognito-idp create-user-pool --pool-name "insucar-${ENV}-customer" \
    --region "$REGION" \
    --username-attributes email \
    --auto-verified-attributes email \
    --policies "PasswordPolicy={MinimumLength=12,RequireUppercase=true,RequireLowercase=true,RequireNumbers=true,RequireSymbols=true}" \
    --account-recovery-setting '{"RecoveryMechanisms":[{"Name":"verified_email","Priority":1}]}' \
    --query 'UserPool.Id' --output text)
  info "  Customer pool: $CUSTOMER_POOL_ID"
else
  info "Customer pool exists: $CUSTOMER_POOL_ID"
fi

# STAFF pool (MFA)
STAFF_POOL_ID=$(pool_id_by_name "insucar-${ENV}-staff")
if [[ -z "$STAFF_POOL_ID" ]]; then
  info "Creating staff user pool..."
  STAFF_POOL_ID=$(aws cognito-idp create-user-pool --pool-name "insucar-${ENV}-staff" \
    --region "$REGION" \
    --username-attributes email \
    --auto-verified-attributes email \
    --policies "PasswordPolicy={MinimumLength=14,RequireUppercase=true,RequireLowercase=true,RequireNumbers=true,RequireSymbols=true}" \
    --account-recovery-setting '{"RecoveryMechanisms":[{"Name":"verified_email","Priority":1}]}' \
    --query 'UserPool.Id' --output text)
  info "  Staff pool: $STAFF_POOL_ID"
else
  info "Staff pool exists: $STAFF_POOL_ID"
fi

# PARTNER pool
PARTNER_POOL_ID=$(pool_id_by_name "insucar-${ENV}-partner")
if [[ -z "$PARTNER_POOL_ID" ]]; then
  info "Creating partner user pool..."
  PARTNER_POOL_ID=$(aws cognito-idp create-user-pool --pool-name "insucar-${ENV}-partner" \
    --region "$REGION" \
    --username-attributes email \
    --auto-verified-attributes email \
    --policies "PasswordPolicy={MinimumLength=12,RequireUppercase=true,RequireLowercase=true,RequireNumbers=true,RequireSymbols=true}" \
    --query 'UserPool.Id' --output text)
  info "  Partner pool: $PARTNER_POOL_ID"
else
  info "Partner pool exists: $PARTNER_POOL_ID"
fi

# --- Domains ---
declare -A DOMAINS=(
  ["customer"]="$CUSTOMER_POOL_ID:insucar-${ENV}-customer"
  ["staff"]="$STAFF_POOL_ID:insucar-${ENV}-staff"
  ["partner"]="$PARTNER_POOL_ID:insucar-${ENV}-partner"
)
for kind in customer staff partner; do
  IFS=':' read -r pid domain <<< "${DOMAINS[$kind]}"
  existing=$(aws cognito-idp describe-user-pool --user-pool-id "$pid" --region "$REGION" --query 'UserPool.Domain' --output text 2>/dev/null || echo "None")
  if [[ "$existing" == "None" ]] || [[ -z "$existing" ]]; then
    info "Creating hosted UI domain for $kind: $domain"
    aws cognito-idp create-user-pool-domain --user-pool-id "$pid" --domain "$domain" --region "$REGION" || true
  else
    info "Domain for $kind: $existing"
  fi
done

# --- App clients ---

# Customer — public SPA (PKCE)
info "Ensuring customer app client..."
CUSTOMER_CLIENT_ID=$(aws cognito-idp list-user-pool-clients --user-pool-id "$CUSTOMER_POOL_ID" --max-results 20 --region "$REGION" \
  --query "UserPoolClients[?ClientName=='consumer-app'].ClientId" --output text 2>/dev/null || echo "")
if [[ -z "$CUSTOMER_CLIENT_ID" || "$CUSTOMER_CLIENT_ID" == "None" ]]; then
  CUSTOMER_CLIENT_ID=$(aws cognito-idp create-user-pool-client --user-pool-id "$CUSTOMER_POOL_ID" \
    --client-name "consumer-app" --region "$REGION" --no-generate-secret \
    --allowed-o-auth-flows "code" \
    --allowed-o-auth-scopes "openid" "email" "profile" \
    --allowed-o-auth-flows-user-pool-client \
    --supported-identity-providers "COGNITO" \
    --callback-urls "https://unysolar.com/app/callback" \
    --logout-urls "https://unysolar.com/app" \
    --query 'UserPoolClient.ClientId' --output text)
fi
info "  Customer client: $CUSTOMER_CLIENT_ID"

# Staff — public SPA (PKCE) for operator console
info "Ensuring staff public client (operator PKCE)..."
STAFF_PUBLIC_CLIENT_ID=$(aws cognito-idp list-user-pool-clients --user-pool-id "$STAFF_POOL_ID" --max-results 20 --region "$REGION" \
  --query "UserPoolClients[?ClientName=='operator-console-pkce'].ClientId" --output text 2>/dev/null || echo "")
if [[ -z "$STAFF_PUBLIC_CLIENT_ID" || "$STAFF_PUBLIC_CLIENT_ID" == "None" ]]; then
  STAFF_PUBLIC_CLIENT_ID=$(aws cognito-idp create-user-pool-client --user-pool-id "$STAFF_POOL_ID" \
    --client-name "operator-console-pkce" --region "$REGION" --no-generate-secret \
    --allowed-o-auth-flows "code" \
    --allowed-o-auth-scopes "openid" "email" "profile" \
    --allowed-o-auth-flows-user-pool-client \
    --supported-identity-providers "COGNITO" \
    --callback-urls "https://op.unysolar.com/callback" \
    --logout-urls "https://op.unysolar.com/" \
    --query 'UserPoolClient.ClientId' --output text)
fi
info "  Staff public client: $STAFF_PUBLIC_CLIENT_ID"

# --- Staff groups (RBAC) ---
info "Creating staff RBAC groups..."
for group in operator supervisor admin ops product_owner; do
  aws cognito-idp create-group --user-pool-id "$STAFF_POOL_ID" --group-name "$group" --region "$REGION" 2>/dev/null || info "  Group $group (exists or created)"
done

# --- Seed test users ---
info "Seeding test users..."
# Customer
aws cognito-idp admin-create-user --user-pool-id "$CUSTOMER_POOL_ID" --username "claire.martin@example.fr" \
  --user-attributes Name=email,Value="claire.martin@example.fr" Name=email_verified,Value=true \
  --region "$REGION" 2>/dev/null || info "  claire.martin (exists)"
aws cognito-idp admin-set-user-password --user-pool-id "$CUSTOMER_POOL_ID" --username "claire.martin@example.fr" \
  --password "Claire#2026Secure" --permanent --region "$REGION" 2>/dev/null || true

# Staff operator
aws cognito-idp admin-create-user --user-pool-id "$STAFF_POOL_ID" --username "operator@insucar.demo" \
  --user-attributes Name=email,Value="operator@insucar.demo" Name=email_verified,Value=true \
  --region "$REGION" 2>/dev/null || info "  operator@insucar.demo (exists)"
aws cognito-idp admin-add-user-to-group --user-pool-id "$STAFF_POOL_ID" --username "operator@insucar.demo" \
  --group-name "operator" --region "$REGION" 2>/dev/null || true
aws cognito-idp admin-set-user-password --user-pool-id "$STAFF_POOL_ID" --username "operator@insucar.demo" \
  --password "InsucarOps#2026!" --permanent --region "$REGION" 2>/dev/null || true

# Product owner
aws cognito-idp admin-create-user --user-pool-id "$STAFF_POOL_ID" --username "po@insucar.demo" \
  --user-attributes Name=email,Value="po@insucar.demo" Name=email_verified,Value=true \
  --region "$REGION" 2>/dev/null || info "  po@insucar.demo (exists)"
aws cognito-idp admin-add-user-to-group --user-pool-id "$STAFF_POOL_ID" --username "po@insucar.demo" \
  --group-name "product_owner" --region "$REGION" 2>/dev/null || true
aws cognito-idp admin-set-user-password --user-pool-id "$STAFF_POOL_ID" --username "po@insucar.demo" \
  --password "InsucarPO#2026!" --permanent --region "$REGION" 2>/dev/null || true

# --- Summary ---
echo ""
echo "==============================================="
echo " Cognito pools ready (env=$ENV, region=$REGION)"
echo "==============================================="
echo ""
echo "  CUSTOMER pool: $CUSTOMER_POOL_ID"
echo "  STAFF pool:    $STAFF_POOL_ID"
echo "  PARTNER pool:  $PARTNER_POOL_ID"
echo ""
echo "  Issuers:"
echo "    customer: https://cognito-idp.$REGION.amazonaws.com/$CUSTOMER_POOL_ID"
echo "    staff:    https://cognito-idp.$REGION.amazonaws.com/$STAFF_POOL_ID"
echo "    partner:  https://cognito-idp.$REGION.amazonaws.com/$PARTNER_POOL_ID"
echo ""
echo "  Public client IDs (PKCE, no secret):"
echo "    customer (consumer-app):        $CUSTOMER_CLIENT_ID"
echo "    staff    (operator-console-pkce): $STAFF_PUBLIC_CLIENT_ID"
echo ""
echo "  Test users:"
echo "    Customer: claire.martin@example.fr / Claire#2026Secure"
echo "    Staff:    operator@insucar.demo     / InsucarOps#2026!"
echo "    PO:       po@insucar.demo           / InsucarPO#2026!"
echo ""
echo "  Configure the backend with:"
echo "    COGNITO_ISSUER=https://cognito-idp.$REGION.amazonaws.com/$STAFF_POOL_ID"
echo "    COGNITO_CLIENT_IDS=$CUSTOMER_CLIENT_ID,$STAFF_PUBLIC_CLIENT_ID"
echo "    COGNITO_CUSTOMER_DOMAIN=insucar-${ENV}-customer"
echo "    COGNITO_CUSTOMER_CLIENT_ID=$CUSTOMER_CLIENT_ID"
echo "    COGNITO_STAFF_DOMAIN=insucar-${ENV}-staff"
echo "    COGNITO_STAFF_CLIENT_ID=$STAFF_PUBLIC_CLIENT_ID"
echo ""
echo "  Teardown: bash scripts/cognito-setup.sh --destroy"
