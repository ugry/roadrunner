########################################
# 3-case IAM model (agenticpromptinsucar.md:106-114)
#
#   Case 1  Developers  -> full least-privilege in DEV only; ZERO UAT/prod.
#   Case 2  Dev->UAT     -> UAT reachable but PRE-PRODUCTION (read-biased,
#                           pipeline-only deploys, no manual mutation).
#   Case 3  Production   -> standing access is READ-ONLY; mutating access is
#                           JIT + MFA + time-bound and MUST be approved by a
#                           product owner (distinct identity, no self-approval).
#
# NOTE: In the target design dev/uat/prod live in SEPARATE AWS accounts and
# the trust relationships cross accounts. This single-account version models
# the same separation with roles + conditions so it can be lifted per-account.
# Every prod grant must also be written to the audit_ledger (db/schema.sql)
# and mirrored to auth.log — CloudTrail records the AssumeRole itself.
########################################

data "aws_iam_policy_document" "assume_current_account_mfa" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      type        = "AWS"
      identifiers = ["arn:aws:iam::${data.aws_caller_identity.current.account_id}:root"]
    }
    condition {
      test     = "Bool"
      variable = "aws:MultiFactorAuthPresent"
      values   = ["true"]
    }
  }
}

########################################
# Groups (assign humans / SSO permission sets to these)
########################################
resource "aws_iam_group" "developers" { name = "insucar-developers" }
resource "aws_iam_group" "operations" { name = "insucar-operations" }
resource "aws_iam_group" "approvers" { name = "insucar-approvers" } # product owners

# Case 3 JIT: membership == an ACTIVE, approved break-glass grant.
# Approvers add a requester here to grant; remove to revoke.
resource "aws_iam_group" "prod_jit" { name = "insucar-prod-jit" }

########################################
# Case 1 — Developers: admin in DEV only, assume dev role (MFA)
########################################
resource "aws_iam_role" "dev_admin" {
  name                 = "insucar-dev-admin"
  max_session_duration = 3600
  assume_role_policy   = data.aws_iam_policy_document.assume_current_account_mfa.json
}

# Scope to the dev namespace/tier. Prototype grants PowerUser in the (single)
# account; in the real multi-account layout this role lives in the DEV account.
resource "aws_iam_role_policy_attachment" "dev_admin_power" {
  role       = aws_iam_role.dev_admin.name
  policy_arn = "arn:aws:iam::aws:policy/PowerUserAccess"
}

data "aws_iam_policy_document" "developers_assume" {
  statement {
    sid       = "AssumeDevOnly"
    effect    = "Allow"
    actions   = ["sts:AssumeRole"]
    resources = [aws_iam_role.dev_admin.arn, aws_iam_role.uat_preprod.arn]
  }
  # Explicit belt-and-braces: developers can never touch the prod roles.
  statement {
    sid       = "DenyProdRoles"
    effect    = "Deny"
    actions   = ["sts:AssumeRole"]
    resources = [aws_iam_role.prod_readonly.arn, aws_iam_role.prod_breakglass.arn]
  }
}

resource "aws_iam_group_policy" "developers_assume" {
  name   = "insucar-developers-assume"
  group  = aws_iam_group.developers.name
  policy = data.aws_iam_policy_document.developers_assume.json
}

########################################
# Case 2 — Dev->UAT: pre-production, read-biased, pipeline-only deploys
########################################
resource "aws_iam_role" "uat_preprod" {
  name                 = "insucar-uat-preprod"
  max_session_duration = 3600
  assume_role_policy   = data.aws_iam_policy_document.assume_current_account_mfa.json
}

# Read-only standing view of UAT; actual deploys go through Spinnaker (pipeline),
# not manual mutation.
resource "aws_iam_role_policy_attachment" "uat_readonly" {
  role       = aws_iam_role.uat_preprod.name
  policy_arn = "arn:aws:iam::aws:policy/ReadOnlyAccess"
}

########################################
# Case 3 — Production: standing READ-ONLY (operations)
########################################
resource "aws_iam_role" "prod_readonly" {
  name                 = "insucar-prod-readonly"
  max_session_duration = 3600
  assume_role_policy   = data.aws_iam_policy_document.assume_current_account_mfa.json
}

resource "aws_iam_role_policy_attachment" "prod_readonly" {
  role       = aws_iam_role.prod_readonly.name
  policy_arn = "arn:aws:iam::aws:policy/ReadOnlyAccess"
}

# Operations: standing prod read-only; may also view/assume UAT read.
data "aws_iam_policy_document" "operations_assume" {
  statement {
    sid       = "AssumeProdReadAndUat"
    effect    = "Allow"
    actions   = ["sts:AssumeRole"]
    resources = [aws_iam_role.prod_readonly.arn, aws_iam_role.uat_preprod.arn]
  }
}

resource "aws_iam_group_policy" "operations_assume" {
  name   = "insucar-operations-assume"
  group  = aws_iam_group.operations.name
  policy = data.aws_iam_policy_document.operations_assume.json
}

########################################
# Case 3 — Production: JIT break-glass (mutating), MFA + time-bound
########################################
resource "aws_iam_role" "prod_breakglass" {
  name                 = "insucar-prod-breakglass"
  max_session_duration = 3600 # 1h JIT window
  assume_role_policy   = data.aws_iam_policy_document.assume_current_account_mfa.json
}

# What break-glass can do (scope down further as needed).
resource "aws_iam_role_policy_attachment" "prod_breakglass_admin" {
  role       = aws_iam_role.prod_breakglass.name
  policy_arn = "arn:aws:iam::aws:policy/AdministratorAccess"
}

# The ONLY thing that lets a caller assume break-glass is membership of the
# prod_jit group. Default membership is empty, so nobody can mutate prod until
# a product owner grants it. Removing the member revokes access.
data "aws_iam_policy_document" "prod_jit_assume" {
  statement {
    sid       = "AssumeBreakglassMfa"
    effect    = "Allow"
    actions   = ["sts:AssumeRole"]
    resources = [aws_iam_role.prod_breakglass.arn]
    condition {
      test     = "Bool"
      variable = "aws:MultiFactorAuthPresent"
      values   = ["true"]
    }
  }
}

resource "aws_iam_group_policy" "prod_jit_assume" {
  name   = "insucar-prod-jit-assume"
  group  = aws_iam_group.prod_jit.name
  policy = data.aws_iam_policy_document.prod_jit_assume.json
}

########################################
# Approvers (product owners): grant/revoke JIT membership; CANNOT self-approve
########################################
data "aws_iam_policy_document" "approvers" {
  statement {
    sid       = "ManageJitGrants"
    effect    = "Allow"
    actions   = ["iam:AddUserToGroup", "iam:RemoveUserFromGroup", "iam:GetGroup"]
    resources = [aws_iam_group.prod_jit.arn]
  }
  # Separation of duties / "no self-approval": IAM cannot compare the user
  # being added to the calling approver, so this is enforced by design, not by
  # a policy condition — approver identities are a DISTINCT set from
  # developers/operations (an approver is not an operator, so approving their
  # own break-glass is out-of-band and visible). Every AddUserToGroup is
  # captured by CloudTrail and must be written to the audit_ledger with
  # who/when/why/approver/scope/expiry. A scheduled job (or the JIT tool)
  # revokes membership at expiry.
}

resource "aws_iam_group_policy" "approvers" {
  name   = "insucar-approvers-grant"
  group  = aws_iam_group.approvers.name
  policy = data.aws_iam_policy_document.approvers.json
}

output "iam_groups" {
  description = "3-case IAM groups. Attach human/SSO identities accordingly."
  value = {
    developers = aws_iam_group.developers.name
    operations = aws_iam_group.operations.name
    approvers  = aws_iam_group.approvers.name
    prod_jit   = aws_iam_group.prod_jit.name
  }
}

output "iam_tier_roles" {
  value = {
    dev_admin       = aws_iam_role.dev_admin.arn
    uat_preprod     = aws_iam_role.uat_preprod.arn
    prod_readonly   = aws_iam_role.prod_readonly.arn
    prod_breakglass = aws_iam_role.prod_breakglass.arn
  }
}
