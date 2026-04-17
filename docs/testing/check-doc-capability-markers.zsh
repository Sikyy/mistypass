#!/usr/bin/env zsh
set -euo pipefail

DOC_ROOT="${DOC_ROOT:-docs}"
DOC_MAX_DEPTH="${DOC_MAX_DEPTH:-0}"
DOC_IGNORE_FILE="${DOC_IGNORE_FILE:-docs/testing/doc-capability-marker-ignore.txt}"

if [[ ! -d "${DOC_ROOT}" ]]; then
  echo "FAIL docs marker check: DOC_ROOT does not exist -> ${DOC_ROOT}"
  exit 1
fi

typeset -a files
typeset -a missing
typeset -a ignored
typeset -a ignore_patterns

if [[ -f "${DOC_IGNORE_FILE}" ]]; then
  while IFS= read -r raw_pattern; do
    pattern="${raw_pattern##[[:space:]]#}"
    pattern="${pattern%%[[:space:]]#}"
    if [[ -z "${pattern}" || "${pattern}" == \#* ]]; then
      continue
    fi
    ignore_patterns+=("${pattern}")
  done < "${DOC_IGNORE_FILE}"
fi

should_ignore_doc_file() {
  local doc_file="$1"
  local pattern
  for pattern in "${ignore_patterns[@]}"; do
    if [[ "${doc_file}" == ${~pattern} ]]; then
      return 0
    fi
  done
  return 1
}

if (( DOC_MAX_DEPTH > 0 )); then
  find_cmd=(find "${DOC_ROOT}" -maxdepth "${DOC_MAX_DEPTH}" -type f -name '*.md')
else
  find_cmd=(find "${DOC_ROOT}" -type f -name '*.md')
fi

while IFS= read -r doc_file; do
  if should_ignore_doc_file "${doc_file}"; then
    ignored+=("${doc_file}")
    continue
  fi
  files+=("${doc_file}")
  if ! rg -q "PROD_READY|CONTRACT_READY|SKELETON_ONLY|BLOCKED_EXTERNAL" "${doc_file}"; then
    missing+=("${doc_file}")
  fi
done < <("${find_cmd[@]}" | sort)

if [[ "${#files[@]}" -eq 0 ]]; then
  echo "FAIL docs marker check: no markdown files found under ${DOC_ROOT} (maxdepth=${DOC_MAX_DEPTH}, ignored=${#ignored[@]})"
  exit 1
fi

if [[ "${#missing[@]}" -gt 0 ]]; then
  echo "FAIL docs marker check: missing capability status markers"
  for doc_file in "${missing[@]}"; do
    echo " - ${doc_file}"
  done
  exit 1
fi

echo "PASS docs marker check: files=${#files[@]} missing=0 ignored=${#ignored[@]} maxdepth=${DOC_MAX_DEPTH}"
