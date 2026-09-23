# LESSONS - auto-maintained by scripts/lessons.py

> Machine-owned. Do NOT hand-edit. Changes are overwritten on the next `lessons.py` write.
> Canonical state lives in `.specs/lessons.json`. Edit lessons only via the script.
> promote_threshold=2 distinct features · window_days=45 · quarantine_threshold=2

## Confirmed (load these at Specify/Design)

Corroborated across multiple features. Safe to apply as guidance.

_none_

## Candidates (under observation - do NOT load as guidance yet)

Seen once or not yet corroborated. Tracked, not trusted.

### L-001 - O gate local do backend roda golangci-lint v2.13.2 com o .golangci.yml do repo, porque go vet e go test sozinhos nao pegam errcheck/staticcheck e o erro so aparece no CI.
- signal: `gate_fail` · recurrence: 1 feature(s) · scope: `backend` · harmful: 0
- features: task-time-tracking
- evidence: PR#2 ci job backend + sonar (backend)
- last seen: 2026-09-23T02:17:07Z

### L-002 - Componentes React declaram props como Readonly<Props> e evitam texto solto ao lado de elemento JSX; o quality gate do Sonar reprova qualquer violacao nova no codigo novo.
- signal: `gate_fail` · recurrence: 1 feature(s) · scope: `frontend` · harmful: 0
- features: task-time-tracking
- evidence: PR#2 sonar quality gate: 6 new_violations (frontend)
- last seen: 2026-09-23T02:17:08Z

## Quarantined (failed when applied - ignore)

A confirmed lesson that recurred alongside failure. Kept for the maintainer to review.

_none_
