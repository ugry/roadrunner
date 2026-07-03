# Operator/Dispatch Console GUI — research (via exa.ai)

Reference systems for emergency/roadside dispatch consoles (CAD) and what to copy.

## Sources
- HxGN OnCall Dispatch (Hexagon) — CAD tailored to roadside assistance (tow/tyre/battery).
- Unified Solutions CAD Dashboard — real-time command center.
- Motorola CallWorks + Spillman Flex CAD — integrated call + CAD single screen.
- daniyal.design — 911 CAD modernization case study.
- rossul.com — emergency dispatch workstation UI.
- YUJ Designs — roadside assistance UX case study (customer + technician + admin).
- Frontiers (NG 9-1-1) — call-taking + map dual-screen study.

## Best-practice patterns to adopt
1. Single-screen workflow: call control + incident creation + dispatch + close in ONE screen
   (fewer keystrokes, fewer errors, less time). No tab-hunting during an emergency.
2. Prioritize the dispatched path: make the active-incident flow fast and linear; keep secondary
   flows uncluttered; minimize cognitive load (life-critical, stressed operators).
3. Real-time command dashboard: active incidents + in-service units + statuses, auto-refreshing.
4. Color-coded status everywhere: e.g. Green = available, Amber = en route, Red = on-site/urgent.
   Priority pills (EMERGENCY/HIGH/NORMAL) always visible.
5. Map-centric: incident location + live unit positions + ETA; "Media screen + Map screen" split
   for richer channels. "Recommend nearest unit" auto-suggestion (operator confirms).
6. Modular, customizable panels (units / calls / map / detail); layout presets (map+grids, map only).
7. Timestamped status trail: status times, call/radio history, address lookup + directions.
8. Ergonomics: function-first, keyboard shortcuts, high-contrast dark theme, big touch targets.

## Insucar operator console — target layout (from these patterns)
- Top bar: identity + status pill, live queue/SLA counters, red 112/PSAP button.
- Left: softphone/call controls + SAFETY TRIAGE first (color-coded warnings).
- Center: screen-pop (caller/policy/vehicle + PRIORITY) -> incident + map + coverage decision +
  suggested service.
- Right: DISPATCH — ranked/recommended providers (nearest + availability + score), one-click
  dispatch, live mission status + shrinking ETA, driver-trust card, send-status-link.
- Bottom: case timeline / interaction log with timestamps + quick actions.
- Cross-cutting: auto-refresh, color-coded statuses, keyboard-first, single-screen.

## Gaps vs current prototype console (to close)
- Live auto-refresh (currently manual), map + live ETA, "recommend nearest" + provider choice,
  coverage-decision action, screen-pop auto-binds to case, softphone controls, SLA/aging timers,
  color-coded status system, keyboard shortcuts, supervisor barge/escalation.
(The Open Design "Mission Control" run is generating a production-grade version of this layout.)
