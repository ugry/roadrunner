# Insucar CI/CD (Jenkins + Spinnaker on EKS)

Everything here is infrastructure-as-code for the delivery pipeline.

## Layout
- `ci/Jenkinsfile` — CI pipeline: Go build/test -> Docker build -> push to ECR -> trigger Spinnaker.
- `ci/plugins.txt` — Jenkins plugin set.
- `ci/jenkins-values.yaml` — Helm values for the Jenkins chart (JCasC: admin, plugins, seeded
  `insucar-ci` pipeline job pulling this repo's `ci/Jenkinsfile`).
- `spinnaker/spinnakerservice.yml` — `SpinnakerService` CR (installed by the spinnaker-operator):
  S3 persistence, Jenkins (igor) integration, in-cluster Kubernetes deploy account, LoadBalancer expose.

## Provisioning (what the automation does)
1. Create EKS cluster `insucar` (eu-west-1) via `eksctl`.
2. Install Jenkins via Helm using `jenkins-values.yaml`
   (a `github-pat` k8s secret is created first so JCasC can check out the private repo).
3. Install the spinnaker-operator (cluster mode) and apply `spinnakerservice.yml`.
4. Create S3 bucket `insucar-spinnaker-326804802908` for Front50 persistence.

## Flow
Git push -> Jenkins `insucar-ci` -> build/test/push image to ECR -> webhook triggers Spinnaker
pipeline: bake -> deploy dev -> tests -> manual judgment -> UAT -> canary/red-black -> manual
judgment (product_owner) -> prod.

## Access
- Jenkins: LoadBalancer URL, user `admin` (password set at install; rotate).
- Spinnaker Deck/Gate: LoadBalancer URLs from the operator.

## Notes / hardening TODO
- Replace the GitHub PAT credential with a scoped deploy token; rotate anything exposed.
- Add SSO (OIDC) in front of Jenkins and Spinnaker; restrict LoadBalancers via SG/WAF.
- Move to Kayenta automated canary analysis and product_owner-gated manual judgment (per prompt).
