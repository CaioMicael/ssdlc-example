#!/usr/bin/env bash
# Copies temporary AWS Academy Learner Lab credentials from a local AWS CLI
# profile into GitHub Actions secrets, so deploy.yml can authenticate.
#
# Usage: scripts/refresh-aws-secrets.sh [profile]
#   profile defaults to $AWS_PROFILE, falling back to "ssdlc".
set -euo pipefail

REPO="CaioMicael/ssdlc-example"
PROFILE="${1:-${AWS_PROFILE:-ssdlc}}"

fail() {
  echo "Erro: $1" >&2
  exit 1
}

command -v aws >/dev/null 2>&1 || fail "aws CLI não encontrado no PATH"
command -v gh >/dev/null 2>&1 || fail "gh CLI não encontrado no PATH"

gh auth status >/dev/null 2>&1 || fail "gh não está autenticado; rode 'gh auth login'"

aws configure get region --profile "$PROFILE" >/dev/null 2>&1 \
  || fail "profile AWS '$PROFILE' não encontrado; rode 'aws configure --profile $PROFILE'"

AWS_ACCESS_KEY_ID="$(aws configure get aws_access_key_id --profile "$PROFILE" 2>/dev/null || true)"
AWS_SECRET_ACCESS_KEY="$(aws configure get aws_secret_access_key --profile "$PROFILE" 2>/dev/null || true)"
AWS_SESSION_TOKEN="$(aws configure get aws_session_token --profile "$PROFILE" 2>/dev/null || true)"

[ -n "$AWS_ACCESS_KEY_ID" ] || fail "aws_access_key_id ausente no profile '$PROFILE'"
[ -n "$AWS_SECRET_ACCESS_KEY" ] || fail "aws_secret_access_key ausente no profile '$PROFILE'"
[ -n "$AWS_SESSION_TOKEN" ] || fail "aws_session_token ausente no profile '$PROFILE' (Learner Lab exige token de sessão)"

aws sts get-caller-identity --profile "$PROFILE" >/dev/null 2>&1 \
  || fail "Credenciais expiradas: reinicie o Learner Lab"

gh secret set AWS_ACCESS_KEY_ID --repo "$REPO" --body "$AWS_ACCESS_KEY_ID" >/dev/null
echo "Secret definido: AWS_ACCESS_KEY_ID"

gh secret set AWS_SECRET_ACCESS_KEY --repo "$REPO" --body "$AWS_SECRET_ACCESS_KEY" >/dev/null
echo "Secret definido: AWS_SECRET_ACCESS_KEY"

gh secret set AWS_SESSION_TOKEN --repo "$REPO" --body "$AWS_SESSION_TOKEN" >/dev/null
echo "Secret definido: AWS_SESSION_TOKEN"

echo "Credenciais copiadas do profile '$PROFILE' para os secrets de $REPO."
echo "Lembrete: credenciais do Learner Lab expiram em ~4h; rode este script de novo quando o deploy falhar por credenciais expiradas."
