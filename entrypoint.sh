#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# security-analyzer GitHub Action Entrypoint
# ==============================================================================

print_banner() {
  echo "========================================================"
  echo "  🛡️ Running AJP Tech Security Analyzer GitHub Action"
  echo "========================================================"
}

export_semgrep_config() {
  export SEMGREP_RULES="${INPUT_RULES:-auto}"
  export SEMGREP_FAIL_ON="${INPUT_FAIL_ON:-ERROR}"
  export SEMGREP_TIMEOUT="${INPUT_TIMEOUT:-10m}"

  if [[ -n "${INPUT_SEMGREP_APP_TOKEN:-}" ]]; then
    export SEMGREP_APP_TOKEN="${INPUT_SEMGREP_APP_TOKEN}"
  fi
}

export_llm_config() {
  export LLM_PROVIDER="${INPUT_PROVIDER:-openai}"

  if [[ -n "${INPUT_MODEL:-}" ]]; then
    export LLM_MODEL="${INPUT_MODEL}"
  fi

  if [[ -n "${INPUT_OPENAI_API_KEY:-}" ]]; then
    export OPENAI_API_KEY="${INPUT_OPENAI_API_KEY}"
  fi

  if [[ -n "${INPUT_ANTHROPIC_API_KEY:-}" ]]; then
    export ANTHROPIC_API_KEY="${INPUT_ANTHROPIC_API_KEY}"
  fi

  if [[ -n "${INPUT_GEMINI_API_KEY:-}" ]]; then
    export GEMINI_API_KEY="${INPUT_GEMINI_API_KEY}"
  fi
}

log_parameters() {
  local mode="$1"
  local scan_path="$2"

  echo "Execution Mode  : ${mode}"
  echo "Target Scan Path: ${scan_path}"
  echo "Semgrep Rules   : ${SEMGREP_RULES}"
  echo "Fail Policy     : ${SEMGREP_FAIL_ON}"

  if [[ "${mode}" == "analyze" ]]; then
    echo "LLM Provider    : ${LLM_PROVIDER}"
    echo "LLM Model       : ${LLM_MODEL:-default}"
  fi
  echo "--------------------------------------------------------"
}

execute_analyzer() {
  local mode="$1"
  local scan_path="$2"
  local exit_code=0

  set +e
  /usr/local/bin/security-analyzer "${mode}" "${scan_path}"
  exit_code=$?
  set -e

  return "${exit_code}"
}

resolve_report_artifacts() {
  local mode="$1"
  local report_file="report.md"
  local scan_id=""

  if [[ "${mode}" == "analyze" && -d "llm-reports" ]]; then
    local latest_report
    latest_report=$(find llm-reports -name "*.md" -type f -exec ls -t {} + 2>/dev/null | head -n 1 || true)
    if [[ -n "${latest_report}" ]]; then
      report_file="${latest_report}"
      local filename
      filename=$(basename "${report_file}")
      scan_id="${filename%.md}"
    fi
  fi

  echo "${scan_id}|${report_file}"
}

export_step_outputs() {
  local scan_id="$1"
  local report_path="$2"
  local exit_code="$3"

  if [[ -z "${GITHUB_OUTPUT:-}" ]]; then
    return 0
  fi

  echo "scan_id=${scan_id}" >> "${GITHUB_OUTPUT}"
  echo "report_path=${report_path}" >> "${GITHUB_OUTPUT}"

  if [[ ${exit_code} -eq 0 ]]; then
    echo "status=success" >> "${GITHUB_OUTPUT}"
  else
    echo "status=failure" >> "${GITHUB_OUTPUT}"
  fi
}

main() {
  local mode="${INPUT_MODE:-scan}"
  local scan_path="${INPUT_SCAN_PATH:-.}"

  print_banner
  export_semgrep_config
  export_llm_config
  log_parameters "${mode}" "${scan_path}"

  local exit_code=0
  execute_analyzer "${mode}" "${scan_path}" || exit_code=$?

  local artifact_meta
  artifact_meta=$(resolve_report_artifacts "${mode}")
  local scan_id="${artifact_meta%%|*}"
  local report_path="${artifact_meta##*|}"

  export_step_outputs "${scan_id}" "${report_path}" "${exit_code}"

  if [[ ${exit_code} -ne 0 ]]; then
    echo "❌ Security Analyzer terminated with exit code ${exit_code}"
    exit "${exit_code}"
  fi

  echo "✅ Security Analyzer completed successfully"
  exit 0
}

main "$@"
