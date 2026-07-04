# Spinnaker pipeline — deploy strategy notes

## What the pipeline does now (`pipelines/insucar-deploy.json`)
```
Deploy DEV → Smoke DEV → [judgment: releasers/PO] →
Deploy UAT → Smoke UAT → [judgment: product owner] →
Deploy PROD → Smoke PROD → (Rollback PROD if unhealthy) → Verify PROD
```
- **Smoke tests** are `webhook` stages hitting `/healthz` (internal svc for
  dev/uat, `https://op.unysolar.com/healthz` for prod). Dev/UAT failures stop
  promotion; the PROD smoke does not fail the pipeline directly so the
  rollback branch can run.
- **Auto-rollback**: `Rollback PROD` (`undoRolloutManifest`, 1 revision back)
  runs only when `Smoke test PROD` did not succeed. `Verify PROD health`
  then fails the pipeline so the overall status reflects the bad deploy.

## Follow-up: red/black (blue-green) — not yet wired
`deployManifest` here creates a fixed `Deployment` named `insucar-api`, so it
does a rolling update, not red/black. To get true red/black you either:
1. Switch to **versioned manifests** (Spinnaker-managed ReplicaSet/Deployment
   with `moniker` + `strategy`), deploying `insucar-api-vNNN` and cutting the
   Service selector over on success; or
2. Adopt **Argo Rollouts** (Rollout CR + blue-green/canary strategy) and let
   Spinnaker apply the Rollout.

Rolling update + smoke + auto-rollback (current) is a reasonable interim.

## Follow-up: Kayenta automated canary analysis — not yet wired
Kayenta needs:
- a canary judge service enabled in the SpinnakerService,
- a metrics store (Prometheus — see the planned observability wiring),
- a canary config (baseline vs canary metric groups + scoring),
- a `Canary Analysis` stage between UAT and PROD.
Blocked on the Prometheus/Grafana observability stack being stood up first.

## Follow-up: parameterize namespace
The three deploy stages duplicate one manifest across `insucar-dev/uat/prod`.
Once happy, factor the manifest into a single templated block using a stage
parameter (or a pipeline template / managed-delivery deliveryConfig) to remove
the duplication.
