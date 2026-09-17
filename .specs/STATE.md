# Project State

## Decisions

### AD-001
- **Decision**: O sistema é de usuário único, sem autenticação, com API e frontend públicos na internet.
- **Reason**: Escolha explícita do usuário para manter o sistema básico.
- **Trade-off**: Qualquer pessoa com a URL pode ler e alterar dados; mitigado apenas por rate limiting por IP. Adicionar autenticação depois exige mudar API, frontend e modelo de dados (dono das tarefas).
- **Scope**: task-time-tracking (API, frontend), delivery-pipeline (exposição da infraestrutura)
- **Date**: 2026-09-16
- **Status**: active

### AD-002
- **Decision**: Horas só são registradas por cronômetro start/stop controlado pelo relógio do servidor, com no máximo um cronômetro ativo no sistema.
- **Reason**: Escolha do usuário; o relógio do servidor evita divergência entre abas e manipulação pelo cliente.
- **Trade-off**: Sem lançamento manual; horas esquecidas só podem ser corrigidas editando apontamentos existentes.
- **Scope**: task-time-tracking (modelo de dados, API, frontend)
- **Date**: 2026-09-16
- **Status**: active

### AD-003
- **Decision**: Tarefas nunca são excluídas definitivamente; são arquivadas (flag reversível, separada do status).
- **Reason**: Preservar o histórico de horas apontadas.
- **Trade-off**: Dados crescem indefinidamente; sem exclusão de dados pelo usuário.
- **Scope**: task-time-tracking
- **Date**: 2026-09-16
- **Status**: active

### AD-004
- **Decision**: Infraestrutura na AWS descrita em Terraform e implantada pelo GitHub Actions via OIDC, sem chaves AWS estáticas; SonarQube é quality gate bloqueante.
- **Reason**: Escolha do usuário (AWS, GitHub Actions, Terraform, SonarQube) alinhada ao objetivo SSDLC.
- **Trade-off**: Depende de um servidor SonarQube disponível e de roles IAM com trust policy OIDC corretas.
- **Scope**: delivery-pipeline, e todas as features que alteram infraestrutura
- **Date**: 2026-09-16
- **Status**: superseded by AD-005

### AD-005
- **Decision**: A AWS é um AWS Academy Learner Lab. O pipeline autentica com as credenciais temporárias do lab guardadas em GitHub secrets, e os recursos usam a role existente `LabRole`. Terraform e SonarQube (self-hosted em EC2) continuam como quality gates.
- **Reason**: O Learner Lab nega criação de roles/OIDC, CloudFront e App Runner (verificado com `iam simulate-principal-policy` em 2026-09-16).
- **Trade-off**: As credenciais expiram a cada ~4h e o deploy falha até serem renovadas; o SonarQube só fica no ar com o lab ativo. Migrar para uma conta AWS normal exige trocar por OIDC.
- **Scope**: delivery-pipeline, task-time-tracking (infra), todas as features que tocam AWS
- **Date**: 2026-09-16
- **Status**: active

### AD-006
- **Decision**: A aplicação é uma única imagem de container (API Go + build do React servido pelo próprio Go) rodando em AWS Lambda com Function URL, via Lambda Web Adapter.
- **Reason**: É a única rota HTTPS sem domínio próprio e sem CloudFront no Learner Lab; mesma origem elimina CORS; o código Go continua um servidor `net/http` comum, portável para ECS/EC2.
- **Trade-off**: Cold start; limite de 15 min por requisição; frontend e backend são sempre implantados juntos.
- **Scope**: delivery-pipeline, task-time-tracking
- **Date**: 2026-09-16
- **Status**: superseded by AD-007

### AD-007
- **Decision**: A infra é mínima: uma EC2 para a aplicação (container Docker com API Go + React, HTTP porta 80) e uma EC2 para o SonarQube, via Terraform com estado local. O deploy publica a imagem no GHCR e atualiza a EC2 via SSM.
- **Reason**: Trabalho acadêmico; o usuário pediu explicitamente "extremamente simples, só EC2".
- **Trade-off**: Sem HTTPS, sem alta disponibilidade, estado do Terraform só na máquina local, e a aplicação cai quando a sessão do lab termina.
- **Scope**: delivery-pipeline, task-time-tracking (o banco virá na mesma linha simples)
- **Date**: 2026-09-16
- **Status**: active

## Handoff

- **Feature**: delivery-pipeline (foco atual: infra primeiro). task-time-tracking fica em espera, com spec escrita; o banco de dados (privado, criptografado, backup de 7 dias, credencial em gerenciador de segredos) entra nela.
- **Phase**: Tasks — spec, design e tasks (18 tarefas, 2 EC2 simples) validados; aguardando aprovação
- **Next step**: Usuário aprova as tarefas e roda `gh auth login`; executar T1
- **Environment**: AWS CLI, Terraform e gh instalados via winget; profile AWS local `ssdlc` (credenciais do Learner Lab, expiram); Docker Desktop estava parado
- **Blockers**: nenhum
- **Branch**: n/a (repositório git ainda não inicializado)
