# Delivery Pipeline Tasks

## Execution Protocol (MANDATORY -- do not skip)

Implement these tasks with the `tlc-spec-driven` skill: **activate it by name and follow its Execute flow and Critical Rules.** Do not search for skill files by filesystem path. The skill is the source of truth for the full flow (per-task cycle, sub-agent delegation, adequacy review, Verifier, discrimination sensor).

**If the skill cannot be activated, STOP and tell the user - do not proceed without it.**

---

**Design**: `.specs/features/delivery-pipeline/design.md`
**Status**: Draft

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
| Quick | Tarefas de backend | `cd backend && go vet ./... && go test ./...` |
| Quick-FE | Tarefas de frontend | `cd frontend && npm run lint && npm run typecheck && npm test -- --run` |
| Build | Scaffold, Dockerfile, config, scripts | Quick + Quick-FE + `npm run build` + `docker build -t ssdlc-example:local .` (se Docker Desktop ativo) / `bash -n <script>` |
| Workflow | `.github/workflows/*` | `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` |
| Terraform | `infra/` | `terraform -chdir=infra fmt -check && terraform -chdir=infra init -backend=false && terraform -chdir=infra validate` |
| Remote | GitHub/AWS | `gh run watch --exit-status`; `curl -fsS <url>` |

---

## Execution Plan

### Phase 1: Esqueleto local

```
T1 → T2 → T3
T4 → T5
T6
```

### Phase 2: CI e segurança

```
T7 → T8 → T9 → T10 → T11
```

### Phase 3: Terraform (2 EC2) e SonarQube

```
T12 → T13 → T14 → T15
```

### Phase 4: Deploy

```
T16 → T17 → T18
```

---

## Task Breakdown

### Phase 1: Esqueleto local

#### T1: Handler de healthz

**What**: Módulo Go `backend` + `HealthHandler` → 200, `application/json`, `{"status":"ok"}`.
**Where**: `backend/internal/httpapi/health.go`
**Depends on**: None
**Reuses**: nada
**Requirement**: SKEL-01
**Tools**: Skill `golang-pro`

**Done when**:

- [ ] Teste valida status, header e corpo exato (AC1)
- [ ] Gate quick passa; 1+ testes

**Tests**: unit
**Gate**: quick
**Commit**: `feat(backend): add healthz handler`

---

#### T2: Router com 404 de API e fallback SPA

**What**: `NewRouter(webDir)`: `/healthz`, 404 em `/api/*`, arquivo estático, fallback `index.html`.
**Where**: `backend/internal/httpapi/router.go`
**Depends on**: T1
**Reuses**: `HealthHandler`
**Requirement**: SKEL-01
**Tools**: Skill `golang-pro`

**Done when**:

- [ ] Testes: `/api/x` → 404 (AC2); `/rota/qualquer` → `index.html` (AC3); arquivo existente servido; `/healthz` via router; traversal não sai do `webDir`
- [ ] Gate quick passa; 6+ testes

**Tests**: unit
**Gate**: quick
**Commit**: `feat(backend): add router with api 404 and spa fallback`

---

#### T3: Entrypoint com PORT e shutdown gracioso

**What**: `main.go` + `run(ctx, getenv, ready)`; `PORT` default 8080; `WEB_DIR`; logs `slog` JSON; shutdown ≤10 s.
**Where**: `backend/cmd/api/main.go`
**Depends on**: T2
**Reuses**: `NewRouter`
**Requirement**: SKEL-01
**Tools**: Skill `golang-pro`

**Done when**:

- [ ] Testes: default 8080 e override (AC4); requisição lenta conclui 200 após cancelar ctx e `run` retorna <10 s (AC5)
- [ ] Gate quick passa; 8+ testes

**Tests**: unit
**Gate**: quick
**Commit**: `feat(backend): add api entrypoint with graceful shutdown`

---

#### T4: Scaffold do frontend

**What**: Vite React TS + ESLint + Vitest/Testing Library + coverage lcov; scripts `lint`, `typecheck`, `test`, `build`; proxy `/healthz` → `:8080`.
**Where**: `frontend/package.json`
**Depends on**: None
**Reuses**: template `create-vite react-ts`
**Tools**: Skill `react`

**Done when**:

- [ ] `npm run lint`, `typecheck`, `build` passam; `npm test -- --run` executa

**Tests**: none
**Gate**: build
**Commit**: `chore(frontend): scaffold vite react typescript app`

---

#### T5: Tela de status da API

**What**: `App` com "SSDLC Example" e `useHealth` → "API online" / "API indisponível".
**Where**: `frontend/src/App.tsx`
**Depends on**: T4
**Reuses**: scaffold
**Requirement**: SKEL-02
**Tools**: Skill `react`

**Done when**:

- [ ] Testes: ok → "API online" (AC6); fetch rejeitado → "API indisponível"; 500 → "API indisponível" (AC7); título
- [ ] Gate quick-fe passa; 4+ testes

**Tests**: unit
**Gate**: quick-fe
**Commit**: `feat(frontend): show api health status`

---

#### T6: Dockerfile

**What**: Multi-stage node → go → distroless nonroot com `/web`.
**Where**: `Dockerfile`
**Depends on**: None
**Reuses**: backend e frontend
**Requirement**: SKEL-03
**Tools**: nenhum

**Done when**:

- [ ] `docker build` + `docker run -p 8080:8080` → `/healthz` 200 e `/` com "SSDLC Example"; usuário não-root (se Docker Desktop indisponível, gate transferido ao job `image` da T10)

**Tests**: none
**Gate**: build
**Commit**: `build: add single container image`

---

### Phase 2: CI e segurança

#### T7: Git e higiene do repositório

**What**: `git init -b main`, `.gitignore` (node_modules, dist, coverage, `.terraform/`, `*.tfstate*`, `.env*`), `.golangci.yml`.
**Where**: `.gitignore`
**Depends on**: None
**Reuses**: nada
**Tools**: nenhum

**Done when**:

- [ ] `git status` sem artefatos/estado; busca por `ASIA`/`aws_secret` no repo vazia

**Tests**: none
**Gate**: build
**Commit**: `chore: initialize repository hygiene`

---

#### T8: CI de backend e frontend

**What**: `ci.yml` com jobs `backend` e `frontend`, cobertura como artefato, concurrency, `permissions: contents: read`, actions por SHA.
**Where**: `.github/workflows/ci.yml`
**Depends on**: T7
**Reuses**: gates locais
**Requirement**: CI-01
**Tools**: nenhum

**Done when**:

- [ ] actionlint passa; todo `uses:` com SHA de 40 caracteres

**Tests**: none
**Gate**: workflow
**Commit**: `ci: add backend and frontend checks`

---

#### T9: Jobs de segredos e dependências

**What**: Jobs `secrets` (gitleaks, histórico completo) e `deps` (trivy fs HIGH,CRITICAL).
**Where**: `.github/workflows/ci.yml`
**Depends on**: T8
**Reuses**: estrutura do T8
**Requirement**: SEC-01
**Tools**: nenhum

**Done when**:

- [ ] actionlint passa

**Tests**: none
**Gate**: workflow
**Commit**: `ci: add secret and dependency scanning`

---

#### T10: Job de imagem

**What**: Job `image`: docker build + trivy image `--ignore-unfixed` HIGH,CRITICAL.
**Where**: `.github/workflows/ci.yml`
**Depends on**: T9
**Reuses**: `Dockerfile`
**Requirement**: SEC-01
**Tools**: nenhum

**Done when**:

- [ ] actionlint passa

**Tests**: none
**Gate**: workflow
**Commit**: `ci: add container image scan`

---

#### T11: Publicar no GitHub com CI verde

**What**: `gh repo create ssdlc-example --public --source . --push`. **Pré-requisito do usuário:** `! gh auth login`.
**Where**: GitHub (remoto)
**Depends on**: T10
**Reuses**: commits anteriores
**Tools**: `gh`

**Done when**:

- [ ] Repo público criado, `main` enviada, `gh run watch --exit-status` do `ci` verde

**Tests**: none
**Gate**: remote
**Commit**: nenhum (push)

---

### Phase 3: Terraform (2 EC2) e SonarQube

#### T12: Terraform das duas EC2

**What**: Stack único: SG app (80), SG sonar (9000), EC2 app t3.micro com Docker, EC2 sonar t3.medium com SonarQube em Docker, EIPs, IMDSv2, discos criptografados, `LabInstanceProfile`, outputs, `.trivyignore` justificado.
**Where**: `infra/main.tf`
**Depends on**: None
**Reuses**: VPC default, `LabInstanceProfile`
**Requirement**: INFRA-01, SONAR-01
**Tools**: nenhum

**Done when**:

- [ ] Gate terraform passa; `trivy config infra` sem HIGH/CRITICAL não justificados

**Tests**: none
**Gate**: terraform
**Commit**: `feat(infra): add app and sonarqube ec2 instances`

---

#### T13: Job de IaC no CI

**What**: Job `iac`: fmt -check, init -backend=false, validate, trivy config.
**Where**: `.github/workflows/ci.yml`
**Depends on**: T12
**Reuses**: gate terraform
**Requirement**: IAC-01
**Tools**: nenhum

**Done when**:

- [ ] actionlint passa

**Tests**: none
**Gate**: workflow
**Commit**: `ci: add terraform validation and iac scan`

---

#### T14: Aplicar infra e preparar SonarQube

**What**: `terraform apply` local (profile `ssdlc`); verificar EC2 no SSM; SonarQube UP; usuário troca senha admin e gera token; definir variables `APP_INSTANCE_ID`, `APP_URL`, `SONAR_HOST_URL` e secret `SONAR_TOKEN` via `gh`.
**Where**: AWS + GitHub (remoto)
**Depends on**: T13
**Reuses**: stack T12
**Requirement**: INFRA-01, SONAR-01
**Tools**: `terraform`, `aws`, `gh`

**Done when**:

- [ ] `curl -fsS http://<ip-sonar>:9000/api/system/status` → `"status":"UP"`
- [ ] `aws ssm describe-instance-information` lista a EC2 app como `Online`
- [ ] `curl http://<ip-app>:22` não conecta (porta fechada)

**Tests**: none
**Gate**: remote
**Commit**: nenhum (estado local, gitignored)

---

#### T15: Job SonarQube

**What**: `sonar-project.properties` (sources backend+frontend, exclusões de teste, `sonar.go.coverage.reportPaths`, `sonar.javascript.lcov.reportPaths`) + job `sonar` com quality gate.
**Where**: `sonar-project.properties`
**Depends on**: T14
**Reuses**: artefatos de cobertura (T8)
**Requirement**: SONAR-02
**Tools**: nenhum

**Done when**:

- [ ] actionlint passa; push mostra check `sonar` verde e projeto no SonarQube com cobertura > 0%

**Tests**: none
**Gate**: remote
**Commit**: `ci: add sonarqube analysis with quality gate`

---

### Phase 4: Deploy

#### T16: Script de refresh das credenciais

**What**: Lê o profile `ssdlc` e roda `gh secret set` para `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`, sem imprimir valores.
**Where**: `scripts/refresh-aws-secrets.sh`
**Depends on**: None
**Reuses**: profile `ssdlc`
**Requirement**: CD-02
**Tools**: `gh`, `aws`

**Done when**:

- [ ] `bash -n` passa; executado e `gh secret list` mostra os 3

**Tests**: none
**Gate**: build
**Commit**: `feat(scripts): add aws lab credentials refresh script`

---

#### T17: Workflow de deploy

**What**: `deploy.yml`: `workflow_run` do `ci` com sucesso na `main`, push GHCR `:sha`, checagem de credencial com mensagem fixa, SSM send-command (pull antes de remover), wait, smoke test 3 min, `concurrency: deploy`.
**Where**: `.github/workflows/deploy.yml`
**Depends on**: T16
**Reuses**: variables da T14
**Requirement**: CD-01
**Tools**: nenhum

**Done when**:

- [ ] actionlint passa; `permissions` só `contents: read` + `packages: write`

**Tests**: none
**Gate**: workflow
**Commit**: `ci: add ec2 deploy workflow`

---

#### T18: Primeiro deploy verificado

**What**: Push na `main`, CI e deploy verdes, pacote GHCR público, app no ar.
**Where**: GitHub + AWS (remoto)
**Depends on**: T17
**Reuses**: tudo acima
**Tools**: `gh`, navegador

**Done when**:

- [ ] `gh run watch --exit-status` do `deploy` verde
- [ ] `curl -fsS http://<ip-app>/healthz` → `{"status":"ok"}` e `/` mostra "SSDLC Example"

**Tests**: none
**Gate**: remote
**Commit**: nenhum (verificação)

---

## Phase Execution Map

```
Phase 1 → Phase 2 → Phase 3 → Phase 4
```

Phase 1: T1→T2→T3, T4→T5, T6 · Phase 2: T7→…→T11 · Phase 3: T12→…→T15 · Phase 4: T16→T17→T18

---

## Task Granularity Check

| Task | Scope | Status |
| ---- | ----- | ------ |
| T1, T2, T3, T5 | 1 handler/router/entrypoint/componente | ✅ Granular |
| T4 | Scaffold gerado por ferramenta | ⚠️ Coeso |
| T6, T7, T16 | 1 arquivo | ✅ Granular |
| T8–T10, T13, T17 | 1–2 jobs de workflow | ✅ Granular |
| T12 | 1 stack Terraform (2 EC2) | ⚠️ Coeso, pedido "simples" |
| T15 | 1 properties + 1 job | ⚠️ Inseparáveis |
| T11, T14, T18 | 1 operação remota | ✅ Granular |

## Diagram-Definition Cross-Check

| Task | Depends On (task body) | Diagram Shows | Status |
| ---- | ---------------------- | ------------- | ------ |
| T1, T4, T6, T7, T12, T16 | None | início de cadeia | ✅ Match |
| T2 / T3 | T1 / T2 | T1 → T2 → T3 | ✅ Match |
| T5 | T4 | T4 → T5 | ✅ Match |
| T8–T11 | anterior | T7 → … → T11 | ✅ Match |
| T13–T15 | anterior | T12 → … → T15 | ✅ Match |
| T17 / T18 | T16 / T17 | T16 → T17 → T18 | ✅ Match |

## Test Co-location Validation

| Task | Code Layer Created/Modified | Matrix Requires | Task Says | Status |
| ---- | --------------------------- | --------------- | --------- | ------ |
| T1, T2 | Go handler/router | unit | unit | ✅ OK |
| T3 | Go entrypoint | unit | unit | ✅ OK |
| T5 | React component/hook | unit | unit | ✅ OK |
| T4, T6, T7, T16 | Config/Dockerfile/script | none | none | ✅ OK |
| T8–T10, T13, T15, T17 | Workflows | none | none | ✅ OK |
| T12 | Terraform | none | none | ✅ OK |
| T11, T14, T18 | Verificação remota | none | none | ✅ OK |
