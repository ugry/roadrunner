########################################
# Amazon Cognito — managed auth (replaces Keycloak)
#
# Three user pools mirror the three surfaces/realms:
#   customer -> consumer app
#   staff    -> operator/coordinator console (MFA required)
#   partner  -> B2B white-label partner portal
# RBAC roles are Cognito GROUPS; they surface in the `cognito:groups` token
# claim and drive app RBAC + Spinnaker fiat roles.
########################################

locals {
  cognito_domain_prefix = "insucar-${var.environment}"
}

# --- CUSTOMER pool -------------------------------------------------------
resource "aws_cognito_user_pool" "customer" {
  name                     = "insucar-${var.environment}-customer"
  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]

  password_policy {
    minimum_length    = 12
    require_lowercase = true
    require_uppercase = true
    require_numbers   = true
    require_symbols   = true
  }

  account_recovery_setting {
    recovery_mechanism {
      name     = "verified_email"
      priority = 1
    }
  }
}

resource "aws_cognito_user_pool_client" "customer_app" {
  name                                 = "consumer-app"
  user_pool_id                         = aws_cognito_user_pool.customer.id
  generate_secret                      = false # public SPA (PKCE)
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email", "profile"]
  allowed_oauth_flows_user_pool_client = true
  supported_identity_providers         = ["COGNITO"]
  callback_urls                        = ["https://unysolar.com/app/callback"]
  logout_urls                          = ["https://unysolar.com/app"]
}

resource "aws_cognito_user_pool_domain" "customer" {
  domain       = "${local.cognito_domain_prefix}-customer"
  user_pool_id = aws_cognito_user_pool.customer.id
}

# --- STAFF pool (operator console; MFA) ----------------------------------
resource "aws_cognito_user_pool" "staff" {
  name                     = "insucar-${var.environment}-staff"
  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]
  mfa_configuration        = "ON"

  software_token_mfa_configuration {
    enabled = true
  }

  password_policy {
    minimum_length    = 14
    require_lowercase = true
    require_uppercase = true
    require_numbers   = true
    require_symbols   = true
  }
}

resource "aws_cognito_user_pool_client" "staff_console" {
  name                                 = "operator-console"
  user_pool_id                         = aws_cognito_user_pool.staff.id
  generate_secret                      = true # confidential client (server-side)
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email", "profile"]
  allowed_oauth_flows_user_pool_client = true
  supported_identity_providers         = ["COGNITO"]
  callback_urls                        = ["https://op.unysolar.com/callback"]
  logout_urls                          = ["https://op.unysolar.com/"]
}

# Spinnaker OAuth client (uses the staff pool + groups for fiat roles)
resource "aws_cognito_user_pool_client" "staff_spinnaker" {
  name                                 = "insucar-spinnaker"
  user_pool_id                         = aws_cognito_user_pool.staff.id
  generate_secret                      = true
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email", "profile"]
  allowed_oauth_flows_user_pool_client = true
  supported_identity_providers         = ["COGNITO"]
  callback_urls                        = ["https://spinnaker.unysolar.com/login"]
}

resource "aws_cognito_user_pool_domain" "staff" {
  domain       = "${local.cognito_domain_prefix}-staff"
  user_pool_id = aws_cognito_user_pool.staff.id
}

# Staff RBAC groups (map to app roles + Spinnaker fiat roles used in the pipeline)
resource "aws_cognito_user_group" "staff_groups" {
  for_each = toset([
    "operator",
    "supervisor",
    "admin",
    "ops",
    "product_owner",
    "insucar-developers",
    "insucar-releasers",
    "insucar-product-owners",
  ])
  name         = each.value
  user_pool_id = aws_cognito_user_pool.staff.id
}

# --- PARTNER pool (B2B white-label) --------------------------------------
resource "aws_cognito_user_pool" "partner" {
  name                     = "insucar-${var.environment}-partner"
  username_attributes      = ["email"]
  auto_verified_attributes = ["email"]
  mfa_configuration        = "OPTIONAL"

  software_token_mfa_configuration {
    enabled = true
  }
}

resource "aws_cognito_user_pool_client" "partner_portal" {
  name                                 = "partner-portal"
  user_pool_id                         = aws_cognito_user_pool.partner.id
  generate_secret                      = true
  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["openid", "email", "profile"]
  allowed_oauth_flows_user_pool_client = true
  supported_identity_providers         = ["COGNITO"]
}

resource "aws_cognito_user_pool_domain" "partner" {
  domain       = "${local.cognito_domain_prefix}-partner"
  user_pool_id = aws_cognito_user_pool.partner.id
}

resource "aws_cognito_user_group" "partner_admin" {
  name         = "partner_admin"
  user_pool_id = aws_cognito_user_pool.partner.id
}

########################################
# Outputs (issuer URLs feed the BFF/console/Spinnaker OIDC config)
########################################
output "cognito_customer_pool_id" { value = aws_cognito_user_pool.customer.id }
output "cognito_staff_pool_id" { value = aws_cognito_user_pool.staff.id }
output "cognito_partner_pool_id" { value = aws_cognito_user_pool.partner.id }

output "cognito_staff_issuer" {
  description = "OIDC issuer for the staff pool (Spinnaker/console fiat)."
  value       = "https://cognito-idp.${var.region}.amazonaws.com/${aws_cognito_user_pool.staff.id}"
}

output "cognito_staff_domain" {
  description = "Hosted-UI domain for the staff pool OAuth endpoints."
  value       = "https://${aws_cognito_user_pool_domain.staff.domain}.auth.${var.region}.amazoncognito.com"
}

output "cognito_spinnaker_client_id" {
  value = aws_cognito_user_pool_client.staff_spinnaker.id
}
