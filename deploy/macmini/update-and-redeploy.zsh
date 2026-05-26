#!/usr/bin/env zsh
set -euo pipefail

REPO_DIR="${REPO_DIR:-/Users/siky/code/MistyPass}"
REMOTE="${REMOTE:-github}"
BRANCH="${BRANCH:-main}"
ENV_FILE="${ENV_FILE:-${REPO_DIR}/.env.staging}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:18080/healthz}"
FORCE_REDEPLOY="${FORCE_REDEPLOY:-false}"

cd "${REPO_DIR}"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "missing env file: ${ENV_FILE}" >&2
  exit 2
fi

if [[ -z "${COMPOSE_FILES:-}" ]]; then
  COMPOSE_FILES="docker-compose.yml"
  if [[ -f "docker-compose.local.yml" ]]; then
    COMPOSE_FILES="${COMPOSE_FILES},docker-compose.local.yml"
  fi
fi

typeset -a compose_args
compose_args=(--env-file "${ENV_FILE}")
for compose_file in ${(s:,:)COMPOSE_FILES}; do
  if [[ -z "${compose_file}" ]]; then
    continue
  fi
  if [[ ! -f "${compose_file}" ]]; then
    echo "missing compose file: ${compose_file}" >&2
    exit 2
  fi
  compose_args+=(-f "${compose_file}")
done

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "tracked working tree changes are present; refusing automatic redeploy" >&2
  exit 3
fi

current_branch="$(git branch --show-current)"
if [[ "${current_branch}" != "${BRANCH}" ]]; then
  echo "== switch ${current_branch:-detached} -> ${BRANCH} =="
  git switch "${BRANCH}"
fi

echo "== fetch ${REMOTE}/${BRANCH} =="
git fetch "${REMOTE}" "${BRANCH}"

before="$(git rev-parse HEAD)"
target="$(git rev-parse "${REMOTE}/${BRANCH}")"

if [[ "${before}" != "${target}" ]]; then
  echo "== fast-forward ${BRANCH}: ${before} -> ${target} =="
  git pull --ff-only "${REMOTE}" "${BRANCH}"
else
  echo "== already up to date: ${before} =="
fi

if [[ "${before}" != "${target}" || "${FORCE_REDEPLOY}" == "true" ]]; then
  echo "== docker compose up: ${COMPOSE_FILES} =="
  docker compose "${compose_args[@]}" up -d --build
else
  echo "== skip compose: no code change =="
fi

echo "== health check: ${HEALTH_URL} =="
for attempt in {1..30}; do
  if curl -fsS "${HEALTH_URL}" >/dev/null; then
    echo "healthz ok"
    exit 0
  fi
  sleep 2
done

echo "healthz failed after retries" >&2
docker compose "${compose_args[@]}" ps
exit 1
