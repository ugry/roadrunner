# Insucar — Security Rotation & Hardening Runbook

Actionable steps to close the prototype's known security shortcuts. Do these in
order; they are code/IaC-ready in this repo (`terraform/iam.tf`,
`spinnaker/spinnakerservice.yml`, `ci/jenkins-values.yaml`) but must be applied
by an operator with confirmation — none of this runs automatically.

> Context: `access.md` documents live secrets that were committed on purpose for
> the prototype. Everything below assumes those are compromised and must be
> replaced, not reused.

## 1. Rotate the AWS ROOT access keys (highest risk)
The account currently uses ROOT access keys inlined in tooling and (historically)
in the SpinnakerService S3 config.

1. Create a scoped IAM user/role for humans (or SSO) — never use root for CLI.
2. Apply `terraform/iam.tf` so each workload gets its own IRSA role:
   - `insucar-<env>-app`       -> SA `insucar/insucar-api`        (SNS SMS)
   - `insucar-<env>-spinnaker` -> SAs `spinnaker/spin-front50`, `spin-clouddriver` (S3)
   - `insucar-<env>-ci`        -> SA `jenkins/jenkins`            (ECR push only)
3. Annotate the ServiceAccounts with the ARNs from `terraform output`:
   ```
   kubectl -n insucar   annotate sa insucar-api      eks.amazonaws.com/role-arn=$(terraform output -raw app_irsa_role_arn) --overwrite
   kubectl -n spinnaker annotate sa spin-front50     eks.amazonaws.com/role-arn=$(terraform output -raw spinnaker_irsa_role_arn) --overwrite
   kubectl -n spinnaker annotate sa spin-clouddriver eks.amazonaws.com/role-arn=$(terraform output -raw spinnaker_irsa_role_arn) --overwrite
   kubectl -n jenkins   annotate sa jenkins          eks.amazonaws.com/role-arn=$(terraform output -raw ci_irsa_role_arn) --overwrite
   ```
4. Remove SNS/ECR permissions from the shared node instance role.
5. Remove any inlined AWS creds from the SpinnakerService (now uses IRSA).
6. In the AWS console: **delete** the root access keys and enable root MFA.

## 2. Rotate the GitHub PAT
1. Regenerate the PAT (scope it to `repo:read` for a single repo, or use a GitHub App / deploy key).
2. Update the cluster secret:
   ```
   kubectl -n jenkins create secret generic github-pat \
     --from-literal=token=<NEW_PAT> --dry-run=client -o yaml | kubectl apply -f -
   ```
3. Restart the Jenkins controller so JCasC re-reads it.

## 3. Rotate the Jenkins admin password
```
helm upgrade jenkins jenkins/jenkins -n jenkins -f ci/jenkins-values.yaml \
  --set controller.admin.password='<NEW_STRONG_PASSWORD>'
```

## 4. Add auth + TLS + LB lockdown to Jenkins and Spinnaker
- Spinnaker: OIDC authn + fiat authz are now configured in
  `spinnaker/spinnakerservice.yml`. Create the client secret first:
  ```
  kubectl -n spinnaker create secret generic spin-secrets \
    --from-literal=oidc-client-secret=<SECRET>
  ```
- Jenkins: enable OIDC/SAML SSO (matrix-auth already installed) and set
  `controller.loadBalancerSourceRanges` in `ci/jenkins-values.yaml` to your
  admin CIDRs (placeholder `203.0.113.0/24` must be replaced).
- Prefer: set both services to `ClusterIP` and expose via the existing
  ingress-nginx + cert-manager (TLS) with OIDC auth, rather than public LBs.
- Restrict the Spinnaker LB via the annotation shown in `spinnakerservice.yml`.

## 5. Verify
- `aws iam list-access-keys --user-name <root>` shows no active root keys.
- Spinnaker Deck now requires login; anonymous cannot approve manualJudgment.
- Jenkins LB rejects connections outside `loadBalancerSourceRanges`.
- App/Spinnaker/CI pods carry no static AWS keys (`env | grep AWS_` empty; IRSA token only).

## 6. Follow-ups (tracked elsewhere)
- Enable Terraform remote state + locking (`versions.tf` backend block).
- 3-case IAM groups + JIT prod access — see `terraform/iam-groups.tf`.
- Move the demo DB password (`postgres/test`) to a managed secret.
