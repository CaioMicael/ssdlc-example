# Task Time Tracking Specification

## Problem Statement

O usuário precisa registrar suas tarefas e saber quanto tempo gastou em cada uma. Hoje não existe sistema; o objetivo é uma API Go + SPA React/TypeScript onde ele cadastra tarefas, acompanha o status e aponta horas com um cronômetro start/stop, sem precisar calcular tempo manualmente.

## Goals

- [ ] Usuário cria uma tarefa e inicia o cronômetro nela em no máximo 3 cliques a partir da tela inicial.
- [ ] O total de horas de cada tarefa é sempre a soma exata dos apontamentos (zero divergência entre lista e detalhe).
- [ ] Nunca existem dois cronômetros ativos ao mesmo tempo, inclusive com requisições concorrentes.

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
| ------- | ------ |
| Login, usuários e permissões | Decisão AD-001: sistema de usuário único, sem autenticação |
| Lançamento manual de horas (criar apontamento sem cronômetro) | Decisão do usuário: horas entram só pelo cronômetro; correção é feita editando um apontamento existente |
| Exclusão definitiva de tarefas | Tarefas são arquivadas para preservar histórico de horas (AD-003) |
| Projetos, tags, clientes, prioridades, prazos | Não pedido; MVP é tarefa + status + horas |
| Relatórios, exportação (CSV/PDF), gráficos | Não pedido; candidato a feature futura |
| Notificações / lembrete de cronômetro esquecido | Não pedido; ver assumption sobre cronômetro esquecido |
| Offline / PWA, app mobile nativo | Não pedido |
| Internacionalização | UI somente em pt-BR |

---

## Assumptions & Open Questions

Every ambiguity is resolved or recorded here - nothing is left silently unclear.

| Assumption / decision | Chosen default | Rationale | Confirmed? |
| --------------------- | -------------- | --------- | ---------- |
| Autenticação | Nenhuma; API e frontend públicos | Escolha explícita do usuário; risco aceito e registrado em AD-001 | y |
| Proteção mínima de API pública | Rate limit de 100 req/min por IP de origem (HTTP 429) | API aberta sem limite gera risco de abuso e custo na AWS; não substitui autenticação | n |
| Modelo de apontamento | Somente cronômetro start/stop | Escolha do usuário | y |
| Cronômetros simultâneos | No máximo 1 ativo no sistema; iniciar em outra tarefa para e salva o atual | Escolha do usuário | y |
| Fonte da verdade do tempo | Relógio do servidor define início e fim; o frontend só exibe | Evita manipulação/deriva do relógio do navegador e mantém consistência entre abas | n |
| Cronômetro esquecido ligado | Sem parada automática; usuário corrige editando o apontamento | Parada automática exigiria regra de negócio não pedida | n |
| Iniciar cronômetro altera status da tarefa | Não; status só muda por ação explícita | Evita efeito colateral surpresa | n |
| Concluir ou arquivar tarefa com cronômetro ativo | Cronômetro é parado e salvo automaticamente | Tarefas concluídas/arquivadas não aceitam cronômetro; manter um ativo violaria a regra | n |
| Status da tarefa | `todo` (A fazer), `in_progress` (Em andamento), `done` (Concluída); transição livre entre eles | Escolha do usuário; sem fluxo obrigatório pedido | y |
| Arquivamento | Flag separada do status; reversível (restaurar); tarefa arquivada é somente leitura | Escolha do usuário (arquivar em vez de excluir); restaurar evita perda por clique errado | n |
| Sobreposição de apontamentos ao editar | Rejeitada (HTTP 422) | Mantém a invariante de 1 cronômetro por vez também nos dados editados | n |
| Exclusão de apontamento | Permitida (exclusão definitiva) para apontamentos finalizados | Única forma de remover um registro incorreto, já que não há lançamento manual | n |
| Limites de campos | Título 1–200 caracteres (após trim); descrição 0–2000 caracteres | Limites usuais que evitam payload abusivo | n |
| Fuso horário | API armazena e retorna UTC (RFC 3339); frontend exibe no fuso do navegador | Padrão sem ambiguidade | n |
| Paginação da lista de tarefas | 50 por página, parâmetro `page` | Lista de usuário único é pequena; limite evita resposta ilimitada | n |
| Banco de dados na AWS | Esta feature provisiona o banco: sem acesso público, criptografado, backup automático de 7 dias, credencial via gerenciador de segredos | Movido do delivery-pipeline, que entrega só o esqueleto sem persistência | n |
| Idioma | UI em pt-BR; códigos de erro da API em inglês (`TASK_NOT_FOUND`) | Códigos estáveis para o código; textos para o usuário | n |

**Open questions:** none - all resolved or logged above (required before the spec is confirmed).

---

## User Stories

### P1: Cadastrar e gerenciar tarefas ⭐ MVP

**User Story**: Como usuário, quero criar, listar, editar e mudar o status das minhas tarefas para organizar meu trabalho.

**Why P1**: Sem tarefa não há onde apontar horas.

**Acceptance Criteria**:
1. WHEN o usuário envia `POST /api/v1/tasks` com título válido THEN a API SHALL criar a tarefa com status `todo`, `archived=false` e responder 201 com a tarefa criada (id, title, description, status, archived, total_seconds=0, created_at, updated_at).
2. IF o título estiver vazio após trim ou tiver mais de 200 caracteres THEN a API SHALL responder 422 com código `VALIDATION_ERROR` e o campo `title` listado em `fields`.
3. IF a descrição tiver mais de 2000 caracteres THEN a API SHALL responder 422 com código `VALIDATION_ERROR` e o campo `description` listado em `fields`.
4. WHEN o usuário envia `GET /api/v1/tasks` THEN a API SHALL retornar as tarefas não arquivadas ordenadas por `updated_at` decrescente, no máximo 50 por página.
5. WHEN a requisição de listagem inclui `status=<valor>` THEN a API SHALL retornar somente tarefas com aquele status.
6. WHEN o usuário envia `PATCH /api/v1/tasks/{id}` com título, descrição ou status válidos THEN a API SHALL atualizar apenas os campos enviados, atualizar `updated_at` e responder 200.
7. IF o status enviado não for `todo`, `in_progress` ou `done` THEN a API SHALL responder 422 com código `VALIDATION_ERROR` e o campo `status` listado em `fields`.
8. IF o id da tarefa não existir THEN a API SHALL responder 404 com código `TASK_NOT_FOUND`.
9. WHEN o usuário cria ou edita uma tarefa no frontend e a API responde 422 THEN o frontend SHALL exibir a mensagem de erro junto ao campo correspondente sem perder o que foi digitado.

**Independent Test**: Criar uma tarefa pela UI, vê-la na lista com status "A fazer", mudar para "Concluída" e confirmar a persistência após recarregar a página.

---

### P1: Cronômetro start/stop ⭐ MVP

**User Story**: Como usuário, quero iniciar e parar um cronômetro em uma tarefa para registrar o tempo gasto sem calcular manualmente.

**Why P1**: É o único meio de apontar horas, o núcleo do sistema.

**Acceptance Criteria**:
1. WHEN o usuário envia `POST /api/v1/tasks/{id}/timer/start` e não há cronômetro ativo THEN a API SHALL criar um apontamento com `started_at` igual ao horário do servidor, `ended_at` nulo, e responder 201.
2. WHEN o usuário inicia o cronômetro na tarefa B enquanto há cronômetro ativo na tarefa A THEN a API SHALL, na mesma transação, finalizar o apontamento de A com `ended_at` igual ao horário do servidor e criar o apontamento ativo de B, respondendo 201.
3. WHEN o usuário inicia o cronômetro em uma tarefa que já tem o cronômetro ativo THEN a API SHALL responder 200 com o apontamento ativo existente sem criar outro.
4. IF a tarefa estiver com status `done` ou arquivada THEN a API SHALL recusar o início do cronômetro com 409 e código `TASK_NOT_TRACKABLE`.
5. WHEN o usuário envia `POST /api/v1/timer/stop` e há cronômetro ativo THEN a API SHALL definir `ended_at` com o horário do servidor e responder 200 com o apontamento finalizado.
6. IF não houver cronômetro ativo ao parar THEN a API SHALL responder 409 com código `NO_ACTIVE_TIMER`.
7. The API SHALL garantir no máximo um apontamento com `ended_at` nulo em qualquer momento, inclusive quando duas requisições de início chegam simultaneamente.
8. WHEN o usuário envia `GET /api/v1/timer` THEN a API SHALL retornar 200 com o apontamento ativo e a tarefa associada, ou 200 com `null` quando não houver cronômetro ativo.
9. WHILE há cronômetro ativo o frontend SHALL exibir, em todas as telas, o título da tarefa e o tempo decorrido no formato `HH:MM:SS`, atualizado a cada 1 segundo e calculado a partir de `started_at`.
10. WHEN o frontend é carregado ou recarregado THEN o frontend SHALL obter o cronômetro ativo em `GET /api/v1/timer` e restaurar a exibição.
11. WHEN o status de uma tarefa com cronômetro ativo muda para `done` THEN a API SHALL finalizar o apontamento ativo com `ended_at` igual ao horário do servidor na mesma transação.

**Independent Test**: Iniciar o cronômetro na tarefa A, esperar 5 s, iniciar na tarefa B, parar; a tarefa A deve ter 1 apontamento de ~5 s e a B 1 apontamento finalizado, sem nenhum cronômetro ativo.

---

### P1: Consultar e corrigir apontamentos ⭐ MVP

**User Story**: Como usuário, quero ver os apontamentos e o total de horas de cada tarefa e corrigir registros errados, já que não existe lançamento manual.

**Why P1**: Sem correção, um cronômetro esquecido deixa dados errados para sempre.

**Acceptance Criteria**:
1. WHEN o usuário envia `GET /api/v1/tasks/{id}/time-entries` THEN a API SHALL retornar os apontamentos da tarefa ordenados por `started_at` decrescente, cada um com id, started_at, ended_at e duration_seconds.
2. The API SHALL calcular `total_seconds` da tarefa como a soma de `duration_seconds` dos apontamentos finalizados daquela tarefa.
3. WHEN o usuário envia `PATCH /api/v1/time-entries/{id}` com `started_at` e/ou `ended_at` válidos THEN a API SHALL atualizar o apontamento e responder 200 com a duração recalculada.
4. IF `ended_at` for menor ou igual a `started_at` THEN a API SHALL responder 422 com código `VALIDATION_ERROR`.
5. IF `ended_at` for posterior ao horário do servidor THEN a API SHALL responder 422 com código `VALIDATION_ERROR`.
6. IF o intervalo editado se sobrepuser a outro apontamento de qualquer tarefa THEN a API SHALL responder 422 com código `TIME_ENTRY_OVERLAP`.
7. IF o apontamento a editar ou excluir estiver ativo THEN a API SHALL responder 409 com código `TIME_ENTRY_ACTIVE`.
8. WHEN o usuário envia `DELETE /api/v1/time-entries/{id}` de um apontamento finalizado THEN a API SHALL remover o apontamento definitivamente e responder 204.
9. IF o id do apontamento não existir THEN a API SHALL responder 404 com código `TIME_ENTRY_NOT_FOUND`.
10. WHEN o usuário abre o detalhe de uma tarefa THEN o frontend SHALL exibir o total de horas no formato `HH:MM` e a lista de apontamentos com data, início, fim e duração no fuso do navegador.
11. WHEN o usuário solicita a exclusão de um apontamento THEN o frontend SHALL pedir confirmação antes de enviar a requisição.

**Independent Test**: Gerar um apontamento, editar o fim para 30 min após o início, verificar total `00:30`; tentar colocar o fim antes do início e ver a mensagem de erro.

---

### P2: Arquivar e restaurar tarefas

**User Story**: Como usuário, quero arquivar tarefas antigas para limpar a lista sem perder o histórico de horas.

**Why P2**: A lista funciona sem isso no início; vira necessário com o uso.

**Acceptance Criteria**:
1. WHEN o usuário envia `POST /api/v1/tasks/{id}/archive` THEN a API SHALL marcar `archived=true`, manter os apontamentos e responder 200.
2. WHEN uma tarefa com cronômetro ativo é arquivada THEN a API SHALL finalizar o apontamento ativo com `ended_at` igual ao horário do servidor na mesma transação.
3. WHEN o usuário envia `POST /api/v1/tasks/{id}/restore` THEN a API SHALL marcar `archived=false`, mantendo o status anterior, e responder 200.
4. WHEN a listagem inclui `archived=true` THEN a API SHALL retornar somente tarefas arquivadas.
5. IF o usuário tentar editar uma tarefa arquivada via `PATCH` THEN a API SHALL responder 409 com código `TASK_ARCHIVED`.
6. The API SHALL não expor nenhum endpoint de exclusão definitiva de tarefa.

**Independent Test**: Arquivar uma tarefa com horas, confirmar que sumiu da lista padrão, aparece no filtro de arquivadas com o mesmo total e volta ao ser restaurada.

---

## Edge Cases

- IF o corpo da requisição não for JSON válido THEN a API SHALL responder 400 com código `INVALID_JSON`.
- IF o corpo da requisição exceder 1 MB THEN a API SHALL responder 413 com código `PAYLOAD_TOO_LARGE`.
- IF um cliente exceder 100 requisições por minuto a partir do mesmo IP THEN a API SHALL responder 429 com código `RATE_LIMITED` e cabeçalho `Retry-After`.
- IF o banco de dados estiver indisponível THEN a API SHALL responder 503 com código `SERVICE_UNAVAILABLE`, sem expor detalhes internos na resposta.
- IF ocorrer erro inesperado THEN a API SHALL responder 500 com código `INTERNAL_ERROR` e mensagem genérica, registrando o detalhe apenas no log.
- IF a API retornar erro de rede ou 5xx THEN o frontend SHALL exibir uma mensagem de falha com opção de tentar novamente, sem apagar dados digitados.
- WHEN o id na URL não for um UUID válido THEN a API SHALL responder 404 com o código de não encontrado do recurso.
- The API SHALL retornar todos os erros no formato `{"error": {"code": string, "message": string, "fields"?: {campo: mensagem}}}`.
- The API SHALL expor `GET /healthz` que responde 200 quando o processo e o banco estão acessíveis e 503 caso contrário.
- The API SHALL registrar cada requisição em log JSON estruturado com request_id, método, rota, status e duração, sem registrar corpos de requisição.
- The API SHALL aceitar CORS somente da origem configurada do frontend.

---

## Requirement Traceability

| Requirement ID | Story | Phase | Status |
| -------------- | ----- | ----- | ------ |
| TASK-01 | P1: Cadastrar e gerenciar tarefas (AC 1–3) | Design | Pending |
| TASK-02 | P1: Cadastrar e gerenciar tarefas (AC 4–5) | Design | Pending |
| TASK-03 | P1: Cadastrar e gerenciar tarefas (AC 6–8) | Design | Pending |
| TASK-04 | P1: Cadastrar e gerenciar tarefas (AC 9) | Design | Pending |
| TIME-01 | P1: Cronômetro (AC 1–4) | Design | Pending |
| TIME-02 | P1: Cronômetro (AC 5–6) | Design | Pending |
| TIME-03 | P1: Cronômetro (AC 7) | Design | Pending |
| TIME-04 | P1: Cronômetro (AC 8, 11) | Design | Pending |
| TIME-05 | P1: Cronômetro (AC 9–10) | Design | Pending |
| ENTRY-01 | P1: Apontamentos (AC 1–2) | Design | Pending |
| ENTRY-02 | P1: Apontamentos (AC 3–7, 9) | Design | Pending |
| ENTRY-03 | P1: Apontamentos (AC 8) | Design | Pending |
| ENTRY-04 | P1: Apontamentos (AC 10–11) | Design | Pending |
| ARCH-01 | P2: Arquivar e restaurar (AC 1–4) | Design | Pending |
| ARCH-02 | P2: Arquivar e restaurar (AC 5–6) | Design | Pending |
| API-01 | Edge Cases: formato de erro, JSON inválido, payload, UUID | Design | Pending |
| API-02 | Edge Cases: rate limit, CORS | Design | Pending |
| API-03 | Edge Cases: 503/500, healthz, logs | Design | Pending |
| API-04 | Edge Cases: erro de rede no frontend | Design | Pending |

**Coverage:** 19 total, 0 mapped to tasks, 19 unmapped ⚠️ (esperado antes de Tasks)

---

## Success Criteria

- [ ] Fluxo criar tarefa → iniciar → parar → ver total funciona ponta a ponta no ambiente AWS.
- [ ] Teste de concorrência com 20 inícios simultâneos termina com exatamente 1 apontamento ativo.
- [ ] Todos os códigos de erro listados nesta spec têm pelo menos um teste automatizado.
