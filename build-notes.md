# Insucar — Build Notes / Decisions (session 2026-07-03)

Decisions taken by the product owner for this build phase:
- TELEPHONY: **Amazon Connect is MOCKED** (a mock telephony-adapter service that emits
  Connect-shaped events / screen-pop). Do NOT provision live Connect for now.
- PROVIDERS: **real provider connections** (HTTP connector calling a real external provider API;
  registry-driven). Plug real AXA/Towpal sandbox creds when available.
- SMS: **real SMS** (AWS SNS/Pinpoint). Note: a new AWS account may be in the SNS SMS sandbox
  (destinations must be verified) — delivery to arbitrary numbers may require moving out of sandbox.
- #13 (immutable prod-access blockchain ledger + JIT IAM approval): **IGNORED for now.**
- KUBERNETES: **2 worker nodes in HA** + **autoscaling** (cluster autoscaler for nodes, HPA for
  pods) to absorb demand/traffic spikes.
  NOTE (quorum caveat, documented): true stateful auto-failover needs an ODD quorum (3, or 2+witness).
  Compute HA at 2 nodes + autoscaling is fine; for HA Postgres add a witness later (Patroni/etcd).
- CI/CD: **Jenkins + Spinnaker must be functional** -> full Spinnaker pipeline
  (bake -> dev -> manual judgment -> UAT -> prod) is a MUST (#14).
- TESTING (#15): unit/integration + smoke tests wired into CI — important, include now.

Action: apply the v3 prompt from this baseline; (re)start the first prototype with the above.
