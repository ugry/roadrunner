# Insucar — Gap Analysis (multi-stakeholder review of the agentic prompt)

Review of `prompt/agenticpromptinsucar.md` from three lived perspectives (architect,
end-user in distress, operator) plus a cross-stakeholder gap register.

## Headline findings
1. **Prompt regressed against an explicit instruction.** The user said "do not defer to
   phase 2, include phase 2 in phase 1," but the prompt de-scoped: multi-cloud warm-standby
   is only "can be added" (line 44-45), and chaos/DR game-days, 24/7 on-call + incident
   management, third-party pentest, GDPR legal review, load testing, and feature-flag/canary
   delivery all disappeared. These must be restored.
2. **Telephony is a single point of failure.** Amazon Connect runs in one AWS region with no
   DR path, contradicting the "even if the cloud provider fails" reliability bar. An emergency
   line must not be able to go fully dark.

## Architect observations (structural gaps)
1. No testing strategy — no test pyramid (unit/integration/e2e/contract/load/chaos), no
   coverage gates, no test-data management.
2. No API contract governance — versioning, backward-compat, deprecation policy.
3. Data model is a flat field list, not a relational model — no entity relationships
   (customer<->policy<->vehicle<->case), no reference data (make/model catalogs), no
   multi-vehicle/multi-policy, no multi-jurisdiction residency (AU/NZ/US laws, not just EU).
4. No backup/DR — RPO/RTO, Postgres PITR, cross-region backups, tested restore runbook.
5. No supply-chain hardening — image signing (cosign), SBOM, SLSA provenance.
6. No certificate lifecycle — cert-manager/mTLS rotation, KMS key-rotation policy.
7. No business SLOs/KPIs — time-to-dispatch, time-to-arrival, dispatch-success rate,
   abandonment, first-contact resolution, NPS.
8. No capacity/surge model — mass simultaneous breakdowns (storms); no overflow/partner
   call-center fallback.
9. No telephony fraud controls — toll fraud / robocall protection on the inbound line.
10. No omnichannel or connected-car — phone-only; no app/web/WhatsApp/chat, no eCall/bCall
    telematics (a crash victim may be unable to dial).

## End-user scenario — "weekend, middle of nowhere, car dead"
- No/low signal: IVR assumes a clean call; with "hold, no callback" a dropped caller is
  orphaned. Need call-drop recovery / proactive outbound callback / SMS fallback / case that
  survives reconnection.
- "I don't know where I am": SMS GPS link needs data; need cell-tower/coarse-location fallback
  and "send location when signal returns."
- Cross-border language: caller's language vs country language; interpreter/translation path.
- Accessibility: deaf/HoH/speech-impaired cannot use voice IVR; need text/RTT/chat channel.
- Trust & safety at night: share arriving tow driver ID/plate/photo with the customer.
- Vulnerable occupants (kids/pets/disabled): welfare escalation, interim taxi, safe-wait guidance.
- After the tow: where does the car go, and how does the customer get home? Onward mobility is
  modelled but not in the demo flow.
- Weekend provider availability / public-holiday awareness.
- Dying battery: minimal-step, resumable interaction.
- Payment when not covered: PCI payment-capture for excess/pay-per-use.

## Operator scenario — "logged in, a call lands"
- Screen-pop miss / unknown caller / third-party reporter: manual case-create + search by
  plate/name.
- Duplicate calls for the same incident: dedup / link-to-open-case.
- No provider accepts (rural/weekend): fallback chain / manual sourcing / escalation.
- Language mismatch with caller: interpreter routing.
- Guided playbooks for hazardous cases (EV fire, motorway hard-shoulder, hazmat, injuries).
- Shift handover / case-ownership transfer with warm handoff.
- Supervisor workflow (barge/whisper/escalation).
- SLA/aging timers on the console (customer-waiting clock, breach alerts).
- Operator system fails mid-call: degraded-mode / console failover / contact-center continuity.
- Emergency-services handoff: Tier-0 currently just says "call 112"; need warm transfer/
  coordination to PSAP.

## Prioritized cross-stakeholder gap register
| # | Gap | Stakeholder | Severity |
|---|---|---|---|
| 1 | Telephony single-region SPOF (no DR) | Client/Ops | Critical |
| 2 | Dropped-call recovery / outbound callback | End-user | Critical |
| 3 | Phase-2 items silently de-scoped | All | Critical |
| 4 | No testing strategy / no DR backup-restore | Architect/Ops | Critical |
| 5 | Emergency-services (112/PSAP) warm-transfer | End-user/Legal | Critical |
| 6 | Accessibility channel (deaf/RTT/chat) | End-user/Legal | High |
| 7 | Cross-border language / interpreter | End-user | High |
| 8 | Provider fallback chain + availability + performance scoring | Operator/Client | High |
| 9 | Tow-driver identity/trust shared to customer | End-user | High |
| 10 | Surge/overflow handling (weather events) | Client/Ops | High |
| 11 | Onward mobility + payment (excess/PCI) in the flow | End-user/Business | High |
| 12 | Business SLOs/KPIs + reporting/BI | Business | Medium |
| 13 | Multi-jurisdiction privacy (AU/NZ/US) | Legal | Medium |
| 14 | Dedup / screen-pop-miss / manual case create | Operator | Medium |
| 15 | Shift handover, supervisor barge/whisper, playbooks | Operator | Medium |
| 16 | Supply-chain (SBOM/signing), cert/KMS rotation | Security | Medium |
| 17 | Omnichannel + eCall/connected-car telematics | Product | Medium |
| 18 | Status-link security (token expiry, PII exposure) | Security/End-user | Medium |

## Recommended top fixes before build
- Restore de-scoped phase-2 items (multi-cloud, chaos, on-call, pentest, load, canary).
- Add telephony DR (multi-region Connect or portable-PBX standby + failover DIDs).
- Add call-drop recovery + proactive outbound callback + resumable cases.
- Add emergency-services warm-transfer for Tier-0.
- Add testing strategy + backup/restore/DR runbook.
- Add provider fallback chain + availability/performance scoring.
- Add accessibility channel and cross-border interpreter routing.
- Add tow-driver identity sharing for customer trust/safety.
