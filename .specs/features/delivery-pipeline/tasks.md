# Delivery Pipeline Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/delivery-pipeline/design.md`
**Status**: Draft

> Tarefas agrupadas a pedido do usuário (velocidade): 8 tarefas, um commit por tarefa. Repositório `CaioMicael/ssdlc-example` já criado, clonado e com commit inicial. `gh` (login) só é necessário a partir da T7 para secrets/variables — alternativa: usuário cadastra pela UI do GitHub.

---

## Test Coverage Matrix

> Generated from spec - confirm before Execute. Guidelines found: none (greenfield) - strong defaults applied.

| Code Layer | Required Test Type | Coverage Expectation | Location Pattern | Run Command |
| ---------- | ------------------ | -------------------- | ---------------- | ----------- |
| Go HTTP handler / router | unit (`httptest`) | 1:1 com SKEL-01 AC1–3: healthz, 404 `/api/`, fallback SPA, arquivo estático, path traversal | `backend/internal/**/*_test.go` | `cd backend && go test ./...` |
| Go entrypoint (`run`) | unit | PORT default/override (AC4); shutdown ≤10 s concluindo requisição em andamento (AC5) | `backend/cmd/api/*_test.go` | `cd backend && go test ./...` |
| React component / hook | unit (Vitest + Testing Library) | online; offline por erro de rede; offline por não-200 (SKEL-02) | `frontend/src/**/*.test.tsx` | `cd frontend && npm test -- --run` |
| Dockerfile, config, scripts | none | build gate | - | build gate |
| Workflows | none | actionlint + execução real no GitHub | - | workflow gate |
| Terraform | none | fmt + validate + trivy config; apply verificado por curl | - | terraform gate |

## Gate Check Commands

> Confirm before Execute.

| Gate Level | When to Use | Command |
| ---------- | ----------- | ------- |
| Quick | Backend | `cd backend && go vet ./... && go test ./...` |
| Quick-FE | Frontend | `cd frontend && npm run lint && npm run typecheck && npm test -- --run && npm run build` |
| Build | Dockerfile, config, scripts | `docker build -t ssdlc-example:local .` (se Docker Desktop ativo; senão job `image` do CI) / `bash -n <script>` |
| Workflow | `.github/workflows/*` | `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` |
| Terraform | `infra/` | `terraform -chdir=infra fmt -check && terraform -chdir=infra init -backend=false && terraform -chdir=infra validate` |
| Remote | GitHub/AWS | `gh run watch --exit-status`; `curl -fsS <url>` |

---

## Execution Plan

### Phase 1: Esqueleto local

```
T1 → T2 → T3
```

### Phase 2: CI

```
T4 → T5
```

### Phase 3: Terraform e SonarQube

```
T6 → T7
```

### Phase 4: Deploy

```
T8
```

---

## Task Breakdown

### Phase 1: Esqueleto local

#### T1: Backend Go completo

**What**: Módulo `backend` com `HealthHandler`, `NewRouter(webDir)` (healthz, 404 `/api/*`, estático, fallback SPA) e `cmd/api` com `run(ctx, getenv, ready)` (PORT default 8080, WEB_DIR, slog JSON, shutdown ≤10 s).
**Where**: `backend/` (internal/httpapi, cmd/api)
**Depends on**: None
**Reuses**: nada
**Requirement**: SKEL-01
**Tools**: Skill `golang-pro`

**Done when**:

- [x] Testes cobrem AC1 (status/header/corpo), AC2 (404), AC3 (fallback + arquivo estático + traversal), AC4 (default/override), AC5 (requisição em andamento conclui; retorno <10 s)
- [x] Gate quick passa; 8+ testes

**Tests**: unit
**Gate**: quick
**Commit**: `feat(backend): add healthz api with spa serving and graceful shutdown`

---

#### T2: Frontend completo

**What**: Scaffold Vite React TS (ESLint, Vitest, Testing Library, coverage lcov, proxy `/healthz`) + `App` com "SSDLC Example" e `useHealth` → "API online" / "API indisponível".
**Where**: `frontend/`
**Depends on**: T1
**Reuses**: template `create-vite react-ts`
**Requirement**: SKEL-02
**Tools**: Skill `react`

**Done when**:

- [x] Testes: ok → "API online" (AC6); fetch rejeitado e 500 → "API indisponível" (AC7); título
- [x] Gate quick-fe passa; 4+ testes

**Tests**: unit
**Gate**: quick-fe
**Commit**: `feat(frontend): add app showing api health status`

---

#### T3: Dockerfile e higiene do repositório

**What**: `.gitignore` (node_modules, dist, coverage, `.terraform/`, `*.tfstate*`, `.env*`); `.golangci.yml`; `Dockerfile` multi-stage node → go → distroless nonroot.
**Where**: raiz (`Dockerfile`, `.gitignore`, `.golangci.yml`)
**Depends on**: T2
**Reuses**: backend e frontend
**Requirement**: SKEL-03
**Tools**: nenhum

**Done when**:

- [ ] `git status` sem artefatos; busca por `ASIA`/`aws_secret` vazia
- [ ] Build gate: imagem sobe, `/healthz` 200, `/` com "SSDLC Example", usuário não-root (ou transferido ao job `image` na T5)

**Tests**: none
**Gate**: build
**Commit**: `build: add container image and repository hygiene`

---

### Phase 2: CI

#### T4: Workflow de CI completo

**What**: `ci.yml` com jobs `backend`, `frontend` (cobertura como artefato), `secrets` (gitleaks), `deps` (trivy fs), `image` (build + trivy image); concurrency; `permissions: contents: read`; actions por SHA.
**Where**: `.github/workflows/ci.yml`
**Depends on**: T3
**Reuses**: gates locais
**Requirement**: CI-01, SEC-01
**Tools**: nenhum

**Done when**:

- [ ] actionlint passa; todo `uses:` com SHA de 40 caracteres

**Tests**: none
**Gate**: workflow
**Commit**: `ci: add build, test and security scanning pipeline`

---

#### T5: Push e CI verde

**What**: `git push origin main` para `CaioMicael/ssdlc-example` (git e remote já configurados).
**Where**: GitHub (remoto)
**Depends on**: T4
**Reuses**: commits T1–T4
**Tools**: `gh`, `git`

**Done when**:

- [ ] Run do `ci` verde na aba Actions (backend, frontend, secrets, deps, image)

**Tests**: none
**Gate**: remote
**Commit**: nenhum (push)

---

### Phase 3: Terraform e SonarQube

#### T6: Terraform das 2 EC2 + job de IaC

**What**: `infra/main.tf` (SG 80 e 9000, EC2 app t3.micro com Docker, EC2 sonar t3.medium com SonarQube, EIPs, IMDSv2, discos criptografados, `LabInstanceProfile`, outputs, `.trivyignore` justificado) + job `iac` no CI.
**Where**: `infra/main.tf`
**Depends on**: T5
**Reuses**: VPC default
**Requirement**: INFRA-01, SONAR-01, IAC-01
**Tools**: nenhum

**Done when**:

- [ ] Gate terraform passa; `trivy config infra` sem HIGH/CRITICAL não justificados; actionlint passa

**Tests**: none
**Gate**: terraform
**Commit**: `feat(infra): add app and sonarqube ec2 with iac checks`

---

#### T7: Apply, SonarQube e job de análise

**What**: `terraform apply` local; usuário troca senha admin e gera token; `gh` define `APP_INSTANCE_ID`, `APP_URL`, `SONAR_HOST_URL`, `SONAR_TOKEN`; `sonar-project.properties` + job `sonar` com quality gate; push.
**Where**: `sonar-project.properties`
**Depends on**: T6
**Reuses**: stack T6, cobertura T4
**Requirement**: SONAR-01, SONAR-02
**Tools**: `terraform`, `aws`, `gh`

**Done when**:

- [ ] `http://<ip-sonar>:9000/api/system/status` → UP; EC2 app `Online` no SSM
- [ ] Check `sonar` verde e projeto com cobertura > 0%

**Tests**: none
**Gate**: remote
**Commit**: `ci: add sonarqube analysis with quality gate`

---

### Phase 4: Deploy

#### T8: Deploy na EC2 ponta a ponta

**What**: `scripts/refresh-aws-secrets.sh` (executado) + `deploy.yml` (`workflow_run` do ci na main, push GHCR `:sha`, checagem de credencial com mensagem fixa, SSM pull→replace, smoke test 3 min, `concurrency: deploy`) + primeiro deploy.
**Where**: `.github/workflows/deploy.yml`
**Depends on**: T7
**Reuses**: variables/secrets da T7
**Requirement**: CD-01, CD-02
**Tools**: `gh`, `aws`

**Done when**:

- [ ] `bash -n` e actionlint passam
- [ ] `gh run watch --exit-status` do `deploy` verde
- [ ] `curl -fsS http://<ip-app>/healthz` → `{"status":"ok"}`; `/` mostra "SSDLC Example"

**Tests**: none
**Gate**: remote
**Commit**: `ci: add ec2 deploy workflow`

---

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| ---- | ---------------------- | ------------- | ------ |
| T1 | None | início | ✅ Match |
| T2 / T3 | T1 / T2 | T1 → T2 → T3 | ✅ Match |
| T4 | T3 (fase anterior) | início da fase 2 | ✅ Match |
| T5 | T4 | T4 → T5 | ✅ Match |
| T6 | T5 (fase anterior) | início da fase 3 | ✅ Match |
| T7 | T6 | T6 → T7 | ✅ Match |
| T8 | T7 (fase anterior) | fase 4 | ✅ Match |

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1 | Go handler/router/entrypoint | unit | unit | ✅ OK |
| T2 | React component/hook | unit | unit | ✅ OK |
| T3 | Dockerfile/config | none | none | ✅ OK |
| T4 | Workflow | none | none | ✅ OK |
| T5, T7, T8 | Remoto + config/workflow | none | none | ✅ OK |
| T6 | Terraform + workflow | none | none | ✅ OK |
