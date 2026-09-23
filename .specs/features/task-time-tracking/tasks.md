# Task Time Tracking Tasks — Fatia 1 (CRUD de tarefas)

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/task-time-tracking/design.md`
**Status**: Draft

> Fatia 1: CRUD de tarefas + arquivar/restaurar, persistindo em SQLite. Cronômetro fica para a fatia 2.
> A `main` está protegida: o trabalho vai na branch `feat/tasks-crud` e entra por pull request com os 7 checks verdes.

---

## Test Coverage Matrix

> Generated from spec + existing repo tests. Guidelines found: none explicit; floor = testes existentes (`backend/internal/httpapi/*_test.go`, `frontend/src/App.test.tsx`), teto = spec.

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Store SQLite | unit (banco temporário real) | Todos os caminhos: criar, listar com filtro, editar, arquivar/restaurar, não encontrado, validação, tarefa arquivada | `backend/internal/store/*_test.go` | `cd backend && go test ./...` |
| Handlers HTTP | unit (`httptest`) | Toda rota: happy path + cada erro listado no design (422, 404, 409, 400) | `backend/internal/httpapi/*_test.go` | `cd backend && go test ./...` |
| Entrypoint / wiring | unit | `DB_PATH` respeitado; `/healthz` reflete o banco | `backend/cmd/api/*_test.go` | `cd backend && go test ./...` |
| React componentes/hooks | unit (Vitest + Testing Library) | Listar, criar, erro de validação 422, mudar status, arquivar | `frontend/src/**/*.test.tsx` | `cd frontend && npm test -- --run` |
| Dockerfile / workflow | none | build gate + deploy real | - | build gate |

## Gate Check Commands

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | Tarefas de backend | `cd backend && go vet ./... && go test -count=1 ./...` |
| Quick-FE | Tarefas de frontend | `cd frontend && npm run lint && npm run typecheck && npm test -- --run && npm run build` |
| Build | Dockerfile / imagem | `docker build -t ssdlc-example:local .` + `docker run` com volume e escrita real no banco |
| Workflow | `.github/workflows/*` | `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` |
| Remote | PR no GitHub | 7 checks verdes no PR |

---

## Execution Plan

### Phase 1: Persistência e API

```
T1 → T2 → T3
```

### Phase 2: Interface

```
T4
```

---

## Task Breakdown

### Phase 1: Persistência e API

#### T1: Store SQLite de tarefas

**What**: Pacote `store` com `Open` (WAL, busy_timeout, migração) e operações Create/List/Get/Update/SetArchived, com erros tipados.
**Where**: `backend/internal/store/store.go`
**Depends on**: None
**Reuses**: estilo de teste do backend existente
**Requirement**: TASK-01, TASK-02, TASK-03, ARCH-01
**Tools**: Skill `golang-pro`

**Done when**:

- [x] Driver Go puro (`modernc.org/sqlite`); build com `CGO_ENABLED=0` funciona
- [x] Testes: criar com defaults (`todo`, `archived=false`); título vazio/>200 e descrição >2000 → `ErrValidation`; listar ordenado por `updated_at` desc; filtro por status e por arquivadas; `Get` inexistente → `ErrNotFound`; `Update` parcial altera só o enviado e mexe em `updated_at`; status inválido → `ErrValidation`; editar arquivada → `ErrArchived`; arquivar e restaurar preservam os campos
- [x] Gate quick passa; 12+ testes

**Tests**: unit
**Gate**: quick
**Commit**: `feat(backend): add sqlite task store`

---

#### T2: Rotas HTTP de tarefas

**What**: Handlers para `POST/GET /api/v1/tasks`, `PATCH /api/v1/tasks/{id}`, `POST .../archive` e `.../restore`, com o formato de erro da spec, e `NewRouter` recebendo o store.
**Where**: `backend/internal/httpapi/tasks.go`
**Depends on**: T1
**Reuses**: `NewRouter`, padrão de handler e testes existentes
**Requirement**: TASK-01, TASK-02, TASK-03, ARCH-01, ARCH-02, API-01
**Tools**: Skill `golang-pro`

**Done when**:

- [x] Testes por rota: 201 com corpo da tarefa; 200 na listagem com filtros; 200 no patch; 200 em archive/restore; 422 com `fields` em título inválido e status inválido; 404 `TASK_NOT_FOUND`; 409 `TASK_ARCHIVED`; 400 `INVALID_JSON`; 413 acima de 1 MB
- [x] Rotas antigas intactas: `/healthz`, 404 em `/api/` desconhecido, fallback SPA
- [x] Gate quick passa; 14+ testes novos
- [x] Nenhum teste existente enfraquecido ou removido

**Tests**: unit
**Gate**: quick
**Commit**: `feat(backend): add task crud endpoints`

---

#### T3: Wiring, volume e deploy

**What**: `run` lê `DB_PATH` (default `./app.db`), abre o store, injeta no router e fecha no shutdown; `/healthz` passa a checar o banco (503 se falhar); `Dockerfile` cria `/data` com dono nonroot e define `DB_PATH`; `deploy.yml` roda com `-v ssdlc_data:/data`.
**Where**: `backend/cmd/api/main.go`
**Depends on**: T2
**Reuses**: `run`, Dockerfile e deploy.yml existentes
**Requirement**: API-03, INFRA-01
**Tools**: Skill `golang-pro`

**Done when**:

- [x] Testes: `DB_PATH` custom é usado; `/healthz` retorna 200 com banco ok e 503 com banco inacessível
- [x] Gate quick e workflow passam
- [x] Build gate: imagem sobe com volume, cria tarefa, container é removido e recriado, e a tarefa continua lá
- [x] Gate quick passa; 4+ testes novos

**Tests**: unit
**Gate**: build
**Commit**: `feat(backend): persist tasks in a docker volume`

---

### Phase 2: Interface

#### T4: Tela de tarefas

**What**: `api/tasks.ts`, `useTasks`, `TaskForm` e `TaskList` na página inicial: criar, listar, mudar status, arquivar/restaurar e alternar exibição de arquivadas.
**Where**: `frontend/src/TaskList.tsx`
**Depends on**: T3
**Reuses**: padrão de `useHealth`, estilo dos testes existentes
**Requirement**: TASK-04, ARCH-01
**Tools**: Skill `react`

**Done when**:

- [ ] Testes com `fetch` mockado: lista renderiza tarefas; criar adiciona à lista; 422 mostra a mensagem no campo e preserva o que foi digitado; mudar status chama o PATCH e reflete na tela; arquivar remove da lista padrão; estado vazio tem mensagem própria
- [ ] Gate quick-fe passa; 6+ testes novos
- [ ] "API online" e o título continuam funcionando (testes existentes intactos)

**Tests**: unit
**Gate**: quick-fe
**Commit**: `feat(frontend): add task list and form`

---

## Phase Execution Map

```
Phase 1 → Phase 2
```

Phase 1: T1 → T2 → T3 · Phase 2: T4

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| ---- | ---------------------- | ------------- | ------ |
| T1 | None | início | ✅ Match |
| T2 | T1 | T1 → T2 | ✅ Match |
| T3 | T2 | T2 → T3 | ✅ Match |
| T4 | T3 (fase anterior) | fase 2 | ✅ Match |

## Test Co-location Validation

| Task | Code Layer | Matrix Requires | Task Says | Status |
| ---- | ---------- | --------------- | --------- | ------ |
| T1 | Store SQLite | unit | unit | ✅ OK |
| T2 | Handlers HTTP | unit | unit | ✅ OK |
| T3 | Wiring + Dockerfile/workflow | unit (wiring) | unit | ✅ OK |
| T4 | React | unit | unit | ✅ OK |
