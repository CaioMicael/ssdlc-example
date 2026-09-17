# Delivery Pipeline Specification

## Problem Statement

Trabalho acadêmico de SSDLC: a aplicação (API Go + frontend React/TS) precisa chegar à AWS de forma **simples e básica**, com um pipeline no GitHub Actions que testa, analisa (SonarQube) e faz o deploy. Infra mínima: **uma EC2 para a aplicação e uma EC2 para o SonarQube**, descritas em Terraform. A conta é um AWS Academy Learner Lab.

**Ordem de entrega (pedido do usuário):** 1) esqueleto + pipeline de CI → 2) Terraform (2 EC2) → 3) deploy.

## Goals

- [ ] Todo pull request para `main` recebe verdict automático (testes, scans, SonarQube, Terraform) em menos de 15 minutos.
- [ ] Merge em `main` atualiza a aplicação na EC2 sem passo manual, desde que as credenciais do lab estejam válidas.
- [ ] Nenhuma credencial AWS é versionada no repositório.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| ------- | ------ |
| Funcionalidade de tarefas/horas | Feature `task-time-tracking`, depois do deploy |
| Banco de dados | Esqueleto não persiste nada |
| Lambda, ECS, ECR, CloudFront, load balancer | Usuário pediu só EC2, "extremamente simples" |
| HTTPS / domínio | Simplicidade de trabalho acadêmico; aplicação e Sonar em HTTP (risco aceito) |
| Terraform apply pelo pipeline / estado remoto | Infra muda raramente; apply manual local basta e dispensa bucket de estado |
| Múltiplos ambientes, rollback automático, monitoramento | Não pedido |
| OIDC / roles IAM próprias | Learner Lab nega criação de roles (verificado em 2026-09-16) |

---

## Assumptions & Open Questions

Every ambiguity is resolved or recorded here - nothing is left silently unclear.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | -------------- | --------- | ---------- |
| Infra | 2 EC2: `app` (t3.micro) e `sonarqube` (t3.medium), VPC default, Elastic IP em cada | Pedido do usuário; EIP mantém o IP quando o lab para e religa as instâncias | y |
| Pipeline | CI + scans de segurança + SonarQube + deploy na `main` | Escolha do usuário | y |
| Região / repositório | `us-east-1`; GitHub público `ssdlc-example` | Escolha do usuário | y |
| Protocolo | HTTP: app na porta 80, SonarQube na porta 9000 | Simplicidade pedida; sem domínio próprio | n |
| Onde fica a imagem | GitHub Container Registry público (`ghcr.io/<owner>/ssdlc-example`) | Sem ECR; o push usa o `GITHUB_TOKEN`, sem credencial extra | n |
| Como o deploy chega na EC2 | `aws ssm send-command` roda `docker pull` + `docker run` na instância | Sem SSH nem chave; a `LabRole` tem `AmazonSSMManagedInstanceCore` (verificado) | n |
| Credenciais AWS no pipeline | Credenciais temporárias do lab em secrets, renovadas por script local | Learner Lab não tem OIDC; expiram em ~4h | n |
| Terraform | Um stack em `infra/`, estado local (gitignored), apply manual pelo usuário/agente | Menos peças; o CI só valida e faz scan | n |
| SonarQube | Container `sonarqube:community` com banco embutido (H2) | "Básico" pedido; H2 é só para avaliação, aceitável no trabalho | n |
| SonarQube fora do ar (lab parado) | Check do Sonar falha; o usuário inicia o lab e roda de novo | Learner Lab para as EC2 no fim da sessão | n |
| PR de fork | Sonar e deploy não rodam (sem secrets) | Repo público; secrets não vão para forks | n |
| Limiar dos scans | Falha em High/Critical; imagem só quando há correção disponível | Equilíbrio segurança/ruído | n |
| Versões | Go 1.26.8 (toolchain), Node 22 LTS | Go 1.24 é EOL e a stdlib tinha 19 CVEs High/Critical no scan Trivy | n |

**Open questions:** none - all resolved or logged above (required before the spec is confirmed).

---

## User Stories

### P1: Esqueleto da aplicação ⭐ MVP

**User Story**: Como desenvolvedor, quero uma API Go e um frontend React/TS mínimos numa única imagem Docker para ter algo real para testar e implantar.

**Why P1**: Sem artefato não há pipeline.

**Acceptance Criteria**:
1. WHEN a API recebe `GET /healthz` THEN a API SHALL responder 200 com `Content-Type: application/json` e corpo `{"status":"ok"}`.
2. WHEN a API recebe requisição para um caminho sob `/api/` que não existe THEN a API SHALL responder 404.
3. WHEN a API recebe `GET` para um caminho fora de `/api/` e `/healthz` que não é arquivo estático THEN a API SHALL responder 200 com o `index.html` do frontend.
4. The API SHALL ler a porta da variável `PORT`, usando `8080` quando ausente.
5. WHEN a API recebe SIGTERM THEN a API SHALL concluir as requisições em andamento e encerrar em no máximo 10 segundos.
6. WHEN o frontend é aberto e `GET /healthz` retorna `{"status":"ok"}` THEN o frontend SHALL exibir o título "SSDLC Example" e o texto "API online".
7. IF a chamada a `/healthz` falhar ou não retornar 200 THEN o frontend SHALL exibir o texto "API indisponível".
8. The imagem Docker SHALL conter API e build do frontend e executar como usuário não-root.

**Independent Test**: `docker run -p 8080:8080` da imagem; `http://localhost:8080/` mostra "API online".

---

### P1: CI e segurança em pull requests ⭐ MVP

**User Story**: Como desenvolvedor, quero que cada PR seja testado e varrido automaticamente para barrar código quebrado ou inseguro.

**Why P1**: Núcleo do SSDLC.

**Acceptance Criteria**:
1. WHEN um pull request é aberto ou atualizado contra `main`, ou há push na `main`, THEN o pipeline SHALL executar lint e testes do backend e lint, typecheck, testes e build do frontend.
2. IF lint, teste, typecheck ou build falhar THEN o pipeline SHALL marcar o check como falho.
3. IF o gitleaks encontrar um segredo no histórico THEN o pipeline SHALL marcar o check de segredos como falho.
4. IF o Trivy encontrar vulnerabilidade High/Critical nas dependências Go ou npm THEN o pipeline SHALL marcar o check de dependências como falho.
5. IF o Trivy encontrar vulnerabilidade High/Critical com correção disponível na imagem Docker THEN o pipeline SHALL marcar o check de imagem como falho.
6. IF `terraform fmt -check`, `terraform validate` ou o Trivy config encontrarem erro ou achado High/Critical em `infra/` THEN o pipeline SHALL marcar o check de IaC como falho.
7. The pipeline SHALL fixar actions de terceiros por SHA de commit e declarar `permissions: contents: read` por padrão.

**Independent Test**: PR com um teste quebrado fica vermelho; corrigido, fica verde.

---

### P1: SonarQube com quality gate ⭐ MVP

**User Story**: Como responsável pelo SSDLC, quero análise estática num SonarQube próprio com quality gate bloqueante.

**Why P1**: Escolha explícita do usuário.

**Acceptance Criteria**:
1. The Terraform SHALL criar uma EC2 `t3.medium` que, ao iniciar, executa o SonarQube Community em Docker na porta 9000.
2. WHEN a EC2 do SonarQube reinicia THEN o SonarQube SHALL voltar a subir automaticamente.
3. WHEN um pull request do próprio repositório ou um push na `main` é analisado THEN o pipeline SHALL rodar o scanner SonarQube com a cobertura de backend e frontend.
4. IF o quality gate for reprovado ou o SonarQube estiver inacessível THEN o pipeline SHALL marcar o check do SonarQube como falho.

**Independent Test**: `http://<ip-sonar>:9000` mostra a tela de login; um PR mostra o check `sonar`.

---

### P1: Deploy na EC2 ⭐ MVP

**User Story**: Como desenvolvedor, quero que o merge na `main` atualize a aplicação na EC2 automaticamente.

**Why P1**: Pedido explícito.

**Acceptance Criteria**:
1. The Terraform SHALL criar uma EC2 `t3.micro` com Docker instalado, porta 80 aberta e sem porta 22 aberta.
2. WHEN o CI termina com sucesso na `main` THEN o pipeline SHALL publicar a imagem no GHCR com a tag do SHA do commit.
3. WHEN a imagem é publicada THEN o pipeline SHALL, via SSM, substituir o container da aplicação na EC2 pela imagem daquele SHA, mapeando a porta 80 para 8080.
4. WHEN o comando SSM termina THEN o pipeline SHALL chamar `http://<ip-app>/healthz` e marcar o deploy como falho se não receber 200 em até 3 minutos.
5. IF o CI falhar na `main` THEN o pipeline SHALL não executar o deploy.
6. IF as credenciais AWS estiverem inválidas ou expiradas THEN o pipeline SHALL falhar com a mensagem "Credenciais AWS inválidas ou expiradas: rode scripts/refresh-aws-secrets.sh".
7. WHILE um deploy está em execução o pipeline SHALL enfileirar o próximo deploy em vez de rodar em paralelo.
8. The repositório SHALL conter `scripts/refresh-aws-secrets.sh`, que copia as credenciais do profile local `ssdlc` para os secrets do GitHub.

**Independent Test**: Merge de uma mudança de texto no frontend aparece em `http://<ip-app>/`.

---

## Edge Cases

- WHEN um pull request vem de um fork THEN o pipeline SHALL pular os jobs que exigem secrets (sonar, deploy).
- IF o `docker pull` falhar na EC2 THEN o pipeline SHALL marcar o deploy como falho e manter o container anterior rodando.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------ |
| SKEL-01 | P1: Esqueleto (AC 1–5) | Tasks | Implementing |
| SKEL-02 | P1: Esqueleto (AC 6–7) | Tasks | Implementing |
| SKEL-03 | P1: Esqueleto (AC 8) | Tasks | Implementing |
| CI-01 | P1: CI e segurança (AC 1–2) | Tasks | Implementing |
| SEC-01 | P1: CI e segurança (AC 3–5, 7) | Tasks | Implementing |
| IAC-01 | P1: CI e segurança (AC 6) | Tasks | Implementing |
| SONAR-01 | P1: SonarQube (AC 1–2) | Tasks | Implementing |
| SONAR-02 | P1: SonarQube (AC 3–4) | Tasks | Pending |
| INFRA-01 | P1: Deploy (AC 1) | Tasks | Implementing |
| CD-01 | P1: Deploy (AC 2–7), Edge Cases | Tasks | Pending |
| CD-02 | P1: Deploy (AC 8) | Tasks | Pending |

**Coverage:** 11 total, 11 mapped to tasks, 0 unmapped

---

## Success Criteria

- [ ] `http://<ip-app>/` mostra "API online" após um merge, sem passo manual.
- [ ] PR com teste quebrado ou segredo exposto fica bloqueado.
- [ ] Nenhuma chave AWS no histórico do git (gitleaks verde).
