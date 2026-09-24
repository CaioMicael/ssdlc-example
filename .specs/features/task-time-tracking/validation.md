# Task Time Tracking (Fatia 1) Validation

**Verdict: PASS**

**Date**: 2026-09-23
**Spec**: `.specs/features/task-time-tracking/spec.md`
**Scope**: Fatia 1 only — "P1: Cadastrar e gerenciar tarefas" (AC1–9) and "P2: Arquivar e restaurar tarefas" (AC1, 3–6). AC2 of P2 (timer-related) and all TIME-*/ENTRY-* stories, rate limit (429), and CORS are **not in this slice** (fatia 2 / delivery-pipeline scope) — listed for completeness, not counted as gaps.
**Diff range**: `336d391..e400944` (squashed PR #2, commit `e400944`)
**Verifier**: independent sub-agent (author ≠ verifier), re-derived coverage from source, evidence-or-zero.

---

## Spec-Anchored Acceptance Criteria

### P1: Cadastrar e gerenciar tarefas

| # | Criterion | Spec-defined outcome | `file:line` + assertion | Result |
|---|-----------|----------------------|--------------------------|--------|
| 1 | POST cria tarefa com título válido | 201, `status=todo`, `archived=false`, `total_seconds=0`, id/title/description/created_at/updated_at present | `backend/internal/httpapi/tasks_test.go:100-127` `TestCreateTask_ValidTitle_Returns201WithDefaults` — asserts `rec.Code != 201`, `got.Status != "todo"`, `got.Archived != false`, `got.TotalSeconds != 0` | ✅ PASS |
| 2 | Título vazio/>200 → 422 | `422`, `code=VALIDATION_ERROR`, `fields.title` present | `tasks_test.go:131-149` (empty) and `:151-169` (>200) — asserts `rec.Code != 422`, `got.Error.Code != "VALIDATION_ERROR"`, `fields["title"]` present | ✅ PASS |
| 3 | Descrição >2000 → 422 | `422`, `VALIDATION_ERROR`, `fields.description` | `tasks_test.go:171-186` — asserts 422 and `fields["description"]` present | ✅ PASS |
| 4 | GET lista não arquivadas, ordenada por `updated_at` desc, máx 50/página | 200, only non-archived, ordering, cap 50 | `tasks_test.go:221-238` (`TestListTasks_ReturnsCreatedTasks`) covers happy path; `store_test.go:107-141` (`TestList_OrdersByUpdatedAtDescending`) covers ordering; `store_test.go:404-422` (`TestList_DefaultLimitIsFifty`) covers the default-50 code path | ⚠️ Spec-precision gap — see note below (cap value never exercised with >50 rows) |
| 5 | `status=<valor>` filtra | only matching status returned | `tasks_test.go:240-268` `TestListTasks_FilteredByStatus_ReturnsOnlyMatching`; `store_test.go:142-174` `TestList_FiltersByStatus` | ✅ PASS |
| 6 | PATCH atualiza só campos enviados, bump `updated_at`, 200 | 200, partial update, unsent fields unchanged | `tasks_test.go:319-350` `TestUpdateTask_TitleAndStatus_Returns200AndPersists` — asserts title/status changed, `Description` unchanged, re-fetches via `s.Get` to confirm persistence; `store_test.go:221-253` `TestUpdate_PartialUpdateChangesOnlySentFields_BumpsUpdatedAt` | ✅ PASS |
| 7 | Status inválido no PATCH → 422 | `422`, `VALIDATION_ERROR`, `fields.status` | `tasks_test.go:354-374` `TestUpdateTask_InvalidStatus_Returns422WithFieldsStatus` | ✅ PASS |
| 8 | id inexistente → 404 | `404`, `TASK_NOT_FOUND` | `tasks_test.go:376-390` `TestUpdateTask_UnknownID_Returns404` | ✅ PASS |
| 9 | Frontend 422 exibe erro por campo sem perder o digitado | field-level error shown, input preserved | `frontend/src/Tasks.test.tsx:121-146` `'shows the field message on 422 and keeps the typed text'`; implementation `frontend/src/TaskForm.tsx:22-35` only clears `title`/`description` on success | ✅ PASS |

### P2: Arquivar e restaurar tarefas (AC1, 3–6; AC2 out of scope — timer)

| # | Criterion | Spec-defined outcome | `file:line` + assertion | Result |
|---|-----------|-----------------------|--------------------------|--------|
| 1 | POST `.../archive` → `archived=true`, mantém apontamentos, 200 | 200, `archived=true` | `tasks_test.go:413-439` `TestArchiveTask_Returns200AndDisappearsFromDefaultList` — asserts `rec.Code != 200`, `!archived.Archived`, and disappearance from default list | ✅ PASS |
| 3 | POST `.../restore` → `archived=false`, mantém status anterior, 200 | 200, `archived=false`, status preserved | `tasks_test.go:441-471` `TestRestoreTask_Returns200AndReturnsToDefaultList`; `store_test.go:298-339` `TestSetArchived_ArchiveThenRestore_PreservesFields` explicitly checks other fields (title/description/status) untouched | ✅ PASS |
| 4 | `archived=true` na listagem → só arquivadas | only archived tasks | `tasks_test.go:289-317` `TestListTasks_ArchivedTrue_ReturnsOnlyArchived`; `store_test.go:175-208` `TestList_FiltersByArchived` | ✅ PASS |
| 5 | PATCH em tarefa arquivada → 409 `TASK_ARCHIVED` | 409, `TASK_ARCHIVED` | `tasks_test.go:392-411` `TestUpdateTask_ArchivedTask_Returns409` | ✅ PASS |
| 6 | Nenhum endpoint de exclusão definitiva | no DELETE task route | `backend/internal/httpapi/tasks.go:73-79` `registerTaskRoutes` registers only POST/GET/PATCH/archive/restore — no DELETE for `/api/v1/tasks*`; confirmed by reading the full route table | ✅ PASS (negative-space check, no route exists) |

**Status**: ⚠️ 1 spec-precision gap flagged (pagination cap boundary), all other in-scope ACs (14/15) match the spec-defined outcome precisely.

---

## Edge Cases (in-scope for fatia 1)

| Edge case | Spec-defined outcome | Evidence | Result |
|-----------|------------------------|----------|--------|
| JSON inválido → 400 `INVALID_JSON` | 400, code | `tasks_test.go:187-201` `TestCreateTask_MalformedJSON_Returns400` | ✅ PASS |
| Corpo >1MB → 413 `PAYLOAD_TOO_LARGE` | 413, code | `tasks_test.go:203-219` `TestCreateTask_OversizedBody_Returns413`; enforced via `http.MaxBytesReader` at `backend/internal/httpapi/tasks.go:212` | ✅ PASS |
| Banco indisponível → 503 `SERVICE_UNAVAILABLE` | 503, code, no internal detail leaked | `backend/cmd/api/main_test.go:306-330` `TestHealthz_StoreUnavailable_Returns503` asserts `resp.StatusCode == http.StatusServiceUnavailable`; handler at `backend/internal/httpapi/health.go:9-19` | ✅ PASS |
| `GET /healthz` 200 quando OK, 503 caso contrário | exact codes both branches | `main_test.go:278-300` (200 case) and `:306-330` (503 case) | ✅ PASS |
| Formato de erro `{"error":{"code","message","fields"?}}` | exact shape | `tasks.go:245-257` `errorResponse`/`errorDetail` types match; every error test above unmarshals into this exact shape and checks `.Error.Code` / `.Error.Fields` | ✅ PASS |
| Erro inesperado → 500 `INTERNAL_ERROR`, detalhe só no log | 500, generic message, no leak | `tasks_test.go:502-517` `TestCreateTask_UnexpectedStoreError_Returns500WithGenericMessage` explicitly asserts the response body does NOT contain the driver's internal error text | ✅ PASS |
| id não-UUID → 404 do recurso | 404 with resource's own not-found code | `tasks_test.go:376-390` uses a non-UUID id (`"nao-existe"`) and asserts 404 `TASK_NOT_FOUND` (store does no UUID-format validation; a non-existent id of any shape falls through to `ErrNotFound`) | ✅ PASS |
| RFC 3339 UTC | timestamps stored/returned as RFC3339 UTC | Code: `backend/internal/store/store.go:176,296,319` use `time.Now().UTC().Format(time.RFC3339Nano)` — verified by reading source | ⚠️ Spec-precision gap — no test parses/asserts the timestamp format; only non-emptiness is checked (`tasks_test.go:126`) |
| Paginação 50 | max 50 per page enforced against real overflow | `pageSize = 50` constant at `backend/internal/httpapi/tasks.go:20`; `store.go:27` `defaultListLimit = 50` — both confirmed by reading source, but no test creates >50 tasks to prove the cap actually truncates a real result set | ⚠️ Spec-precision gap (same as AC4 above) |

**Not in this slice** (correctly out of scope, not gaps): `RATE_LIMITED`/429/`Retry-After`, CORS restriction to configured origin — spec lists these as Edge Cases but design.md explicitly defers them; no code for either exists yet in this diff, which is consistent with "not implemented in fatia 1," not a defect.

---

## Discrimination Sensor

Ran in isolated scratch worktree `C:/Users/caiom/Documents/ssdlc-wt/verify2` (detached at `e400944`), never touching the real tree. Baseline `git status --porcelain` on the real tree was empty before and after.

| # | Mutation | File:line | Killed? |
|---|----------|-----------|---------|
| 1 | Store: `validateTitle` — disabled the >200-char check (`if false && len(trimmed) > titleMaxLen`) | `backend/internal/store/store.go:125` | ✅ Killed — `TestCreate_TitleOver200Chars_ValidationError` and `TestCreateTask_TitleTooLong_Returns422WithFieldsTitle` failed |
| 2 | Store: `Update` also mutates a field that was not sent (`task.Status = StatusDone` unconditionally) | `backend/internal/store/store.go:276-283` | ✅ Killed — `TestUpdate_PartialUpdateChangesOnlySentFields_BumpsUpdatedAt` failed |
| 3 | Store: `List` drops the `archived` filter (`WHERE 1 = 1` instead of `WHERE archived = ?`) | `backend/internal/store/store.go:207-209` | ✅ Killed — `TestList_FiltersByArchived`, `TestListTasks_ArchivedTrue_ReturnsOnlyArchived`, `TestArchiveTask_Returns200AndDisappearsFromDefaultList` all failed |
| 4 | Handler: `writeStoreError` maps `ErrNotFound` to 500 instead of 404 | `backend/internal/httpapi/tasks.go:235-236` | ✅ Killed — `TestUpdateTask_UnknownID_Returns404` failed |
| 5 | Handler: `handleCreateTask` returns 200 instead of 201 | `backend/internal/httpapi/tasks.go:94` | ✅ Killed — `TestCreateTask_ValidTitle_Returns201WithDefaults` failed |
| 6 | Frontend: `useTasks.create` no longer prepends the created task to the list | `frontend/src/useTasks.ts:35-42` | ✅ Killed — `'creates a task: sends the POST body and shows the new row'` (`frontend/src/Tasks.test.tsx:85-119`) failed (1 test file failed, 11/12 passed) |

**Sensor depth**: lightweight (6 mutations, standard feature tier — exceeds the 1–3 minimum), spanning store, handlers, and frontend.
**Result**: 6/6 killed — PASS. No surviving mutants; no fix tasks required from the sensor.

Isolation verified: real-tree `git status --porcelain` was empty before sensor work and empty after `git worktree remove --force`.

---

## Gate Check

| Gate | Command | Result |
|------|---------|--------|
| Backend vet+test | `cd backend && go vet ./... && go test -count=1 ./...` | `go vet` clean; **45 tests passed**, 0 failed, 0 skipped, across `cmd/api`, `internal/httpapi`, `internal/store` |
| Backend lint (CI-equivalent) | `golangci-lint v2.13.2 run --config ../.golangci.yml ./...` | **0 issues** |
| Frontend lint | `npm run lint` (eslint) | clean, no output/errors |
| Frontend typecheck | `npm run typecheck` (tsc -b --noEmit) | clean |
| Frontend tests | `npm test -- --run` (vitest) | **12/12 passed**, 2 test files |
| Frontend build | `npm run build` | succeeded, `dist/` produced |

No test count regression (no baseline pre-feature test count exists in this repo history for this feature — 45 backend / 12 frontend tests are all net-new for this diff).

---

## Live Deployment Check (`http://44.218.231.163`)

**Could not reach the host from this environment.** `curl` to port 80 and 8080 both timed out (`Connection timed out after 15007 milliseconds`), while general internet egress from the same shell worked (`curl http://example.com` → `200`). `WebFetch` also failed (`connect ECONNREFUSED :443` — it auto-upgrades to HTTPS, which this API does not serve). This looks like a network path/firewall restriction specific to this verifier's environment reaching that IP/port, not something the diff under review can explain — flagged as an **environment limitation**, not a code defect. No `GET /healthz`, `GET /api/v1/tasks`, or `POST` create/archive could be exercised against the live instance in this session.

---

## Security Sanity

| Check | Result |
|-------|--------|
| No credentials in `git log -p 336d391..e400944` | Checked via grep for password/secret/api_key/token/AKIA/private-key patterns — no matches beyond legitimate error codes (`TASK_NOT_FOUND`, `VALIDATION_ERROR`) |
| `DB_PATH` not hardcoded, not world-writable | `Dockerfile:20` sets `ENV ... DB_PATH=/data/app.db`; `Dockerfile:14,19` creates `/data` in the build stage and copies it into the final stage with `--chown=nonroot:nonroot`, so the directory is owned by `nonroot`, not world-writable by default `COPY` permissions |
| Container runs as nonroot | `Dockerfile:16` base image `gcr.io/distroless/static-debian12:nonroot`; `Dockerfile:22` explicit `USER nonroot:nonroot` before `ENTRYPOINT`. (Docker was available locally but the image was not built/inspected at runtime; verified via Dockerfile source, which is sufficient per the task's fallback instruction.) |

---

## Fix Plans

None required — 0 surviving mutants, 0 failed ACs. The two spec-precision gaps (pagination cap boundary test, RFC3339 format assertion) do not indicate incorrect behavior — source code was read directly and both `pageSize`/`defaultListLimit = 50` and `time.Now().UTC().Format(time.RFC3339Nano)` are correct — they indicate the test suite doesn't pin these exact values against a large/format-sensitive input. Recommended (optional, non-blocking) follow-up: add one store test creating 51 tasks and asserting `len(result) == 50`, and one test asserting `time.Parse(time.RFC3339, task.CreatedAt)` succeeds and the parsed time's `Location()` is UTC.

---

## Summary

**Overall**: ✅ Ready (fatia 1)

**Spec-anchored check**: 14/15 in-scope ACs matched the spec-defined outcome exactly; 1 AC (P1#4, list pagination) plus the general "paginação 50" and "RFC 3339 UTC" edge cases carry a spec-precision gap in test strength, not in implementation.
**Sensor**: 6/6 mutations killed (store, handler, and frontend layers all discriminating).
**Gate**: 45 backend tests + 12 frontend tests passed, 0 failed; vet, lint, typecheck, and build all clean.

**What works**: Full CRUD (create/list/patch), archive/restore, validation (title/description/status), error taxonomy (400/404/409/422/413/500/503) with exact codes and the spec's error envelope, frontend field-level error handling without data loss, SQLite persistence via `DB_PATH` with WAL/busy_timeout, nonroot Docker image.

**Issues found**: None functional. Two test-strength gaps noted above (non-blocking).

**Next steps**: None required to ship fatia 1. Optional: strengthen the two flagged tests before fatia 2 builds on this store. Live E2E check against `http://44.218.231.163` should be re-run from an environment with network access to that host/port, since this session could not reach it.
