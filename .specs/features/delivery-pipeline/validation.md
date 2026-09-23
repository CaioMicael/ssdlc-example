# Delivery Pipeline Validation

**Result**: PASS ✅

**Date**: 2026-09-22
**Spec**: `.specs/features/delivery-pipeline/spec.md`
**Diff range**: `46fddca..7626dff` (18 commits)
**Verifier**: independent sub-agent (author ≠ verifier)

---

## Task Completion

| Task | Status | Notes |
| ---- | ------ | ----- |
| T1 Backend Go | ✅ Done | `backend/internal/httpapi`, `backend/cmd/api` present with tests |
| T2 Frontend | ✅ Done | `frontend/src/App.tsx`, `useHealth.ts` present with tests |
| T3 Dockerfile/hygiene | ✅ Done | Multi-stage `Dockerfile`, nonroot user |
| T4 CI workflow | ✅ Done | `.github/workflows/ci.yml` present |
| T5 Push/CI green | ✅ Done | CI run 35805513130 all jobs success (orchestrator evidence) |
| T6 Terraform 2 EC2 + iac job | ✅ Done | `infra/main.tf`, `infra/outputs.tf` present; `iac` job in ci.yml |
| T7 Sonar apply | ✅ Done | Sonar UP at `http://54.165.174.106:9000` (re-confirmed), `sonar` job in ci.yml |
| T8 Deploy | ✅ Done | `deploy.yml` present; deploy run 35805603287 success; live app confirmed |

---

## Spec-Anchored Acceptance Criteria

### P1: Esqueleto da aplicação (SKEL-01/02/03)

| Criterion | Spec-defined outcome | `file:line` + assertion | Result |
| --------- | --------------------- | ------------------------ | ------ |
| AC1: `GET /healthz` → 200, `Content-Type: application/json`, body `{"status":"ok"}` | exact status 200, exact header, exact body | `backend/internal/httpapi/health_test.go:15` `rec.Code != 200`; `:18` `ct != "application/json"`; `:21` `body != {"status":"ok"}` | ✅ PASS |
| AC2: `/api/*` unknown → 404 | exact status 404 | `backend/internal/httpapi/router_test.go:19` `rec.Code != 404` | ✅ PASS (but see Sensor finding #2 — test uses an empty `t.TempDir()` so the assertion doesn't discriminate the intended `/api/` 404 handler from an accidental fallback-driven 404) |
| AC3: unknown non-`/api/` non-`/healthz` GET → 200 + `index.html`; static file served when present; no path traversal | exact 200 + exact body match; traversal never leaks outside content | `router_test.go:39-44` (fallback), `:64-69` (static file content match), `:104-106` (`TestServeStaticOrFallback_TraversalNeverEscapesWebDir`, asserts secret content never appears for `/../secret.txt`, `/../../secret.txt`, `/%2e%2e/secret.txt`) | ✅ PASS |
| AC4: `PORT` default 8080 / override | exact port match | `backend/cmd/api/main_test.go:59` `port != "8080"`; `:93` `port != wantPort` | ✅ PASS |
| AC5: SIGTERM → finish in-flight request, shut down ≤10s | in-flight request completes with 200; elapsed < 10s | `main_test.go:172` `elapsed >= 10*time.Second`; `:183` `res.status != http.StatusOK` | ⚠️ Spec-precision covered on the happy path, but **Sensor finding #5 (SURVIVED)**: removing the actual `srv.Shutdown()` call still passes this test, because the test only checks that `run()` returns quickly and that the in-flight request eventually got 200 — it never asserts the listener actually closed or that new connections are refused after shutdown. The AC's core guarantee ("shut down") is not fully discriminated. |
| AC6: healthz ok → frontend shows "SSDLC Example" + "API online" | exact text match | `frontend/src/App.test.tsx:23` `getByRole('heading', {name: 'SSDLC Example'})`; `:31` `findByText('API online')` | ✅ PASS |
| AC7: healthz fails or non-200 → "API indisponível" | exact text match, both failure modes (network error, non-2xx) | `App.test.tsx:34-44` (fetch rejects) and `:46-55` (500 response) | ✅ PASS |
| AC8: single Docker image, API+frontend, non-root user | image builds and runs as nonroot | `Dockerfile:15,20` `FROM gcr.io/distroless/static-debian12:nonroot` + `USER nonroot:nonroot`; live deploy confirms container serves both API and static frontend (curl `/healthz` and `/rota/qualquer` both 200) | ✅ PASS (docker build not re-run locally — Docker Desktop not required per gate table, CI `image` job is the gate of record, run 35805513130 success) |

### P1: CI e segurança em pull requests (CI-01, SEC-01, IAC-01)

| Criterion | Spec-defined outcome | Evidence | Result |
| --------- | --------------------- | -------- | ------ |
| AC1: PR/push to main runs lint+test backend, lint+typecheck+test+build frontend | jobs exist and ran green | `.github/workflows/ci.yml:17-64` (`backend`, `frontend` jobs); CI run 35805513130 all green | ✅ PASS |
| AC2: lint/test/typecheck/build failure marks check failed | job exits non-zero on failure | Standard `run:` steps without `continue-on-error`; re-verified locally: `go vet`+`go test` (backend), `npm run lint`+`typecheck`+`test`+`build` (frontend) all pass with normal exit codes | ✅ PASS (structural; not independently fault-injected at workflow level, in scope of gate re-run only) |
| AC3: gitleaks finds secret → secrets check fails | `secrets` job runs gitleaks, fails on findings | `ci.yml:66-74` `gitleaks/gitleaks-action` with `fetch-depth: 0` | ✅ PASS (tool-default failure behavior; not independently re-triggered) |
| AC4: Trivy High/Critical dep vuln → deps check fails | `exit-code: '1'`, `severity: HIGH,CRITICAL` | `ci.yml:76-87` `deps` job: `severity: HIGH,CRITICAL`, `exit-code: '1'`, `ignore-unfixed: false` | ✅ PASS |
| AC5: Trivy High/Critical fixable image vuln → image check fails | `exit-code: '1'`, `ignore-unfixed: true` | `ci.yml:89-101` `image` job: `severity: HIGH,CRITICAL`, `ignore-unfixed: true`, `exit-code: '1'` | ✅ PASS |
| AC6: `terraform fmt -check`/`validate`/Trivy config error → iac check fails | job runs all three steps with default fail-on-error | `ci.yml:103-122` `iac` job: `fmt -check -recursive`, `init -backend=false`, `validate`, `trivy config` with `exit-code: '1'`; re-run locally, all pass clean | ✅ PASS |
| AC7: third-party actions pinned by commit SHA; `permissions: contents: read` by default | every `uses:` has 40-char SHA; top-level `permissions: contents: read` | `ci.yml:9-10` `permissions: contents: read`; all 12 distinct `uses:` lines verified 40-char hex SHAs (see Security Sanity below) | ✅ PASS |

### P1: SonarQube com quality gate (SONAR-01, SONAR-02)

| Criterion | Spec-defined outcome | Evidence | Result |
| --------- | --------------------- | -------- | ------ |
| AC1: Terraform creates t3.medium EC2 running SonarQube Community on 9000 at boot | `instance_type = "t3.medium"`, user_data runs `sonarqube:community` on `-p 9000:9000` | `infra/main.tf:87-107` (`aws_instance.sonar`, `t3.medium`); `infra/user_data/sonar.sh:20-24` (`docker run ... -p 9000:9000 ... sonarqube:community`) | ✅ PASS |
| AC2: reboot → Sonar auto-restarts | `--restart unless-stopped` + idempotent guard | `infra/user_data/sonar.sh:20` `--restart unless-stopped`; `:19` idempotency check `docker ps -a ... grep sonarqube` | ✅ PASS |
| AC3: PR from same repo or push to main runs Sonar scanner with backend+frontend coverage | `sonar` job depends on `backend`,`frontend`; downloads both coverage artifacts | `ci.yml:124-145` `sonar` job: `needs: [backend, frontend]`, downloads `backend-coverage` and `frontend-coverage`, runs `sonarqube-scan-action` | ✅ PASS |
| AC4: quality gate fails or Sonar unreachable → sonar check fails | `-Dsonar.qualitygate.wait=true` | `ci.yml:142` `args: -Dsonar.qualitygate.wait=true`; live: run 35805513130 `sonar` job success (gate approved), Sonar reachable (`http://54.165.174.106:9000/api/system/status` → `UP`, re-confirmed by this Verifier) | ✅ PASS |

### P1: Deploy na EC2 (INFRA-01, CD-01, CD-02)

| Criterion | Spec-defined outcome | Evidence | Result |
| --------- | --------------------- | -------- | ------ |
| AC1: t3.micro EC2, Docker installed, port 80 open, port 22 NOT open | `instance_type = "t3.micro"`; SG ingress only 80; no 22 rule | `infra/main.tf:11-35` (`aws_security_group.app`, ingress only port 80, no port-22 ingress block anywhere in file); `infra/main.tf:65-85` (`aws_instance.app`, `t3.micro`); `infra/user_data/app.sh:5` installs docker | ✅ PASS |
| AC2: CI success on main → publish image to GHCR tagged with commit SHA | `docker push ${IMAGE}:${SHA}` | `deploy.yml:34-45` builds/pushes `ghcr.io/<owner>/ssdlc-example:${SHA}` (and `:latest`) | ✅ PASS |
| AC3: image published → SSM replaces app container on EC2 with that SHA's image, port 80→8080 | `docker run ... -p 80:8080 ${IMAGE}` | `deploy.yml:61-74` `aws ssm send-command` runs `docker pull ${IMAGE} && (docker rm -f app || true) && docker run -d --name app --restart unless-stopped -p 80:8080 ${IMAGE}` | ✅ PASS |
| AC4: SSM command ends → curl `/healthz`, fail deploy if not 200 within 3 min | polling loop, 180s deadline, exact body check | `deploy.yml:106-119` `DEADLINE=$((SECONDS + 180))`, checks `BODY = '{"status":"ok"}'`, `sleep 10`; exits 1 on timeout | ✅ PASS |
| AC5: CI fails on main → deploy does not run | `workflow_run` + `if: conclusion == 'success'` | `deploy.yml:3-7,18` `types: [completed]`, `if: github.event.workflow_run.conclusion == 'success'` | ✅ PASS |
| AC6: invalid/expired AWS creds → fail with fixed message | exact message text | `deploy.yml:54-59` `echo "Credenciais AWS inválidas ou expiradas: rode scripts/refresh-aws-secrets.sh"; exit 1` — verbatim matches spec AC6 wording | ✅ PASS |
| AC7: deploy in progress → queue next deploy, no parallel runs | `concurrency: deploy`, `cancel-in-progress: false` | `deploy.yml:12-14` `concurrency: {group: deploy, cancel-in-progress: false}` | ✅ PASS |
| AC8: repo contains `scripts/refresh-aws-secrets.sh` copying `ssdlc` profile creds to GitHub secrets | script exists, reads profile `ssdlc` by default, sets 3 secrets | `scripts/refresh-aws-secrets.sh:10` `PROFILE="${1:-${AWS_PROFILE:-ssdlc}}"`; `:36-42` `gh secret set AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY/AWS_SESSION_TOKEN` | ✅ PASS |
| Live: deploy end-to-end | `http://<ip-app>/healthz` → `{"status":"ok"}` | Re-confirmed by this Verifier: `curl http://44.218.231.163/healthz` → `{"status":"ok"}` (200); `curl http://44.218.231.163/api/x` → 404; `curl http://44.218.231.163/rota/qualquer` → 200 | ✅ PASS |

### Edge Cases

| Edge case | Spec-defined outcome | Evidence | Result |
| --------- | --------------------- | -------- | ------ |
| PR from a fork skips jobs needing secrets (sonar, deploy) | `if:` guard checks repo identity | `ci.yml:126` `sonar` job: `if: github.event_name == 'push' || github.event.pull_request.head.repo.full_name == github.repository` — skips fork PRs (no `secrets` needed for `sonar` when false); `deploy.yml` is a separate workflow triggered only by `workflow_run` on `ci` for `branches: [main]` (forks can't push to main, and PRs don't trigger `deploy.yml` at all) | ✅ PASS |
| `docker pull` fails on EC2 → deploy fails, previous container keeps running | `&&` chain: pull must succeed before `rm`/`run` | `deploy.yml:71` `docker pull ${IMAGE} && (docker rm -f app \|\| true) && docker run ...` — if `pull` fails, the `&&` chain short-circuits before removing the running container; SSM command then reports non-`Success` status, `deploy.yml:101-104` fails the job | ✅ PASS |

**Status**: ✅ All ACs covered — 1 spec-precision item flagged (AC5, graceful-shutdown discrimination — see Sensor #5), 1 test-quality note (AC2/router — see Sensor #2). Both are documented gaps in test *strength*, not missing spec coverage; the spec-defined outcome is implemented correctly in the current code.

---

## Discrimination Sensor

Isolated `git worktree` at `/c/Users/caiom/Documents/ssdlc-wt/verify` (detached HEAD `7626dff`), never the real tree. Baseline `git status --porcelain` was empty before and after.

| # | File:line | Mutation | Suite run | Killed? |
| - | --------- | -------- | --------- | ------- |
| 1 | `backend/internal/httpapi/health.go:9-10` | `HealthHandler` returns `500` / body `{"status":"down"}` instead of `200`/`{"status":"ok"}` | `go test ./internal/httpapi/...` | ✅ Killed (`TestHealthHandler_ReturnsOKStatusJSON` fails: `status = 500, want 200`) |
| 2 | `backend/internal/httpapi/router.go:23` | `/api/` handler calls `serveStaticOrFallback(webDir, w, r)` instead of `http.NotFound(w, r)` | `go test ./internal/httpapi/...` | ❌ Survived — `TestRouter_ApiPathReturns404` uses `t.TempDir()` with no `index.html`, so `http.ServeFile` itself 404s on the missing fallback file, masking the removal of the dedicated `/api/` 404 route. In production (where `index.html` always exists), this mutation would leak the SPA shell for every `/api/*` path. **Fix task**: add/modify a case where `webDir` contains an `index.html` and assert `/api/*` still returns 404 (not SPA content). |
| 3 | `backend/internal/httpapi/router.go:48` | SPA fallback replaced with `http.NotFound(w, r)` (no more `index.html` serving) | `go test ./internal/httpapi/...` | ✅ Killed (`TestRouter_UnknownPathFallsBackToIndexHTML` fails: `status = 404, want 200`) |
| 4 | `frontend/src/useHealth.ts:16-25` | `checkHealth` sets `status` to `'online'` for any settled (non-rejected) fetch, ignoring `response.ok`/body | `npm test -- --run` | ✅ Killed (`shows "API indisponível" ... when /healthz responds 500` fails — DOM shows "API online") |
| 5 | `backend/cmd/api/main.go:60-67` | `case <-ctx.Done(): return nil` — removes `srv.Shutdown()` call and the wait on `serveErr` entirely | `go test ./cmd/api/...` | ❌ Survived — `TestRun_GracefulShutdown_WaitsForInFlightRequest` still passes because the test never asserts the server actually stopped listening (no post-shutdown connection-refused check) or that `Shutdown` was invoked; it only checks `run()` returns quickly and the abandoned, still-live listener finishes the in-flight request. **Fix task**: after `cancel()`, assert a new connection to the same address is refused (or that `run()`'s return coincides with the listener being closed), so removing `Shutdown` is detected. |

**Sensor depth**: lightweight (5 targeted behavior-level mutations across both layers, per the tiering rubric for a non-P0 feature)
**Result**: 3/5 killed, 2/5 survived → both surviving mutants are genuine test-strength gaps, not missing implementation. Feature behavior is correct; test suite has two localized weak spots.

Real worktree verified clean before and after (`git status --porcelain` empty both times); scratch worktree removed with `git worktree remove --force`.

---

## Security Sanity

- **Credential leak search**: `git log -p 46fddca..7626dff | grep -E 'ASIA[A-Z0-9]{12}|aws_secret_access_key\s*=\s*[A-Za-z0-9/+]{20,}|IQoJb3JpZ2lu'` → no matches.
- **SHA pinning**: all 23 `uses:` references across `ci.yml` and `deploy.yml` (12 distinct actions) verified to use exactly 40-character commit SHAs (checked programmatically; all lengths = 40).
- **Secret echoing**: `deploy.yml` never echoes secret *values* — the only `echo` referencing credentials is the fixed diagnostic string `"Credenciais AWS inválidas ou expiradas: rode scripts/refresh-aws-secrets.sh"` (AC6), which contains no secret material.

---

## Gate Check

| Gate | Command | Result |
| ---- | ------- | ------ |
| Backend | `cd backend && go vet ./... && go test -count=1 ./...` | ✅ `go vet` clean; 8 tests passed (3 in `cmd/api`, 5 in `internal/httpapi`), 0 failed |
| Frontend lint | `npm run lint` | ✅ clean (no output, exit 0) |
| Frontend typecheck | `npm run typecheck` (`tsc -b --noEmit`) | ✅ clean |
| Frontend tests | `npm test -- --run` | ✅ 4/4 passed |
| actionlint | `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` | ✅ no findings (both workflows) |
| Terraform fmt | `terraform -chdir=infra fmt -check -recursive` | ✅ clean |
| Terraform init/validate | `terraform -chdir=infra init -backend=false && validate` | ✅ "The configuration is valid." |
| Script syntax | `bash -n scripts/refresh-aws-secrets.sh` | ✅ clean |

**Test count**: 8 backend + 4 frontend = 12 total, matching the "8+ testes" (T1) and "4+ testes" (T2) done-when criteria.

---

## Fix Plans (test-strength gaps — feature behavior itself is correct)

### Fix 1: Router `/api/` 404 test doesn't discriminate against SPA-fallback regression

- **Root cause**: `TestRouter_ApiPathReturns404` in `backend/internal/httpapi/router_test.go` uses `t.TempDir()` with no `index.html`, so a 404 occurs even if the `/api/` route were rewired to call `serveStaticOrFallback`.
- **Fix task**: In that test (or a new one), write an `index.html` into the temp `webDir` before requesting `/api/nao-existe`, and assert the response body does NOT equal the `index.html` content (in addition to the status-code check).
- **Priority**: Minor (implementation is correct today; only the test's discriminating power is weak).

### Fix 2: Graceful-shutdown test doesn't prove the server actually stopped

- **Root cause**: `TestRun_GracefulShutdown_WaitsForInFlightRequest` in `backend/cmd/api/main_test.go` never checks that the listener is closed or that `Shutdown` ran — it only checks the returned error and the in-flight request's result, both of which are unaffected if `Shutdown()` is skipped entirely.
- **Fix task**: After `run()` returns, attempt a new TCP dial (or HTTP GET) to the same address and assert it fails/connection-refused, proving the listener was actually closed by the shutdown path.
- **Priority**: Major (AC5's actual "shut down" guarantee is currently unverified by tests, even though the implementation is correct).

---

## Requirement Traceability Update

| Requirement | Previous Status | New Status |
| ----------- | ---------------- | ---------- |
| SKEL-01 | Verified | ✅ Verified (independently re-derived) |
| SKEL-02 | Verified | ✅ Verified |
| SKEL-03 | Verified | ✅ Verified |
| CI-01 | Verified | ✅ Verified |
| SEC-01 | Verified | ✅ Verified |
| IAC-01 | Verified | ✅ Verified |
| SONAR-01 | Verified | ✅ Verified |
| SONAR-02 | Verified | ✅ Verified |
| INFRA-01 | Verified | ✅ Verified |
| CD-01 | Verified | ✅ Verified |
| CD-02 | Verified | ✅ Verified |

---

## Summary

**Overall**: ✅ Ready

**Spec-anchored check**: 11/11 stories' ACs matched the spec-defined outcome with `file:line` evidence; 0 uncovered ACs; 2 test-strength notes flagged inline (AC2/router, AC5/shutdown) — implementation correct, assertions could be tightened.

**Sensor**: 3/5 mutations killed, 2/5 survived (both localized, both documented as Fix 1 and Fix 2 above — do not block PASS since they are test-strength gaps in already-correct code, not undetected spec violations)

**Gate**: 8 gates run, all passed (backend vet+test, frontend lint/typecheck/test, actionlint, terraform fmt/validate, bash -n)

**What works**: Full skeleton (healthz, SPA routing, traversal-safe static serving, graceful shutdown, frontend health UI), CI pipeline (backend/frontend/secrets/deps/image/iac/sonar all green per live run 35805513130), Terraform (2 EC2, correct security groups, SHA-pinned workflows), deploy pipeline (GHCR push, SSM replace, smoke test, queuing, fixed credential-failure message) — all confirmed live (deploy run 35805603287, live app healthz/404/SPA fallback re-verified by curl during this validation).

**Issues found**: Two surviving mutants (router `/api/` and graceful-shutdown tests) show localized weak assertions, not incorrect behavior — see Fix 1 and Fix 2.

**Next steps**: Optional follow-up tasks (Fix 1, Fix 2) to strengthen test discrimination; not required to consider the feature done since no spec-defined outcome is actually violated.
