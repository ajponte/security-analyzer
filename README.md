# security-analyzer

AI-driven SAST scanner combining Semgrep static analysis and LLM-based security audits for local workflows and GitHub Actions.

---

## GitHub Action Usage

`security-analyzer` is available as a containerized GitHub Action that executes Semgrep SAST scans and agentic LLM audits directly inside your CI/CD pipeline without needing to manually install Go, Python, or Semgrep dependencies.

### Example 1: Fast SAST Scan on Pull Requests

Run static application security testing on code changes and upload the scan report:

```yaml
name: Security Scan

on:
  pull_request:
    branches: [ main ]
  push:
    branches: [ main ]

jobs:
  sast:
    name: Fast SAST Scan
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Repository
        uses: actions/checkout@v4

      - name: Run AJP Tech Security Analyzer
        id: security_scan
        uses: ajponte/security-analyzer@v1
        with:
          mode: 'scan'
          scan_path: '.'
          rules: 'auto'
          fail_on: 'ERROR'
          timeout: '5m'

      - name: Upload Scan Report
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: semgrep-report
          path: report.md
          retention-days: 14
```

### Example 2: Agentic LLM Security Audit

Run an AI-driven security audit synthesizing Semgrep findings with multi-turn LLM reasoning:

```yaml
name: AI Security Audit

on:
  schedule:
    - cron: '0 3 * * 1' # Weekly on Monday at 3:00 AM UTC
  workflow_dispatch:

jobs:
  ai-audit:
    name: Deep LLM Security Analysis
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Repository
        uses: actions/checkout@v4

      - name: Run AJP Tech Security Analyzer (AI Audit)
        id: ai_scan
        uses: ajponte/security-analyzer@v1
        with:
          mode: 'analyze'
          scan_path: '.'
          provider: 'anthropic'
          model: 'claude-3-5-sonnet-latest'
          anthropic_api_key: ${{ secrets.ANTHROPIC_API_KEY }}
          fail_on: 'ERROR'

      - name: Upload AI Analysis Reports
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: llm-security-reports
          path: llm-reports/
          retention-days: 30
```

### Action Inputs

| Input | Description | Required | Default |
| --- | --- | --- | --- |
| `mode` | Execution mode: `scan` (direct Semgrep SAST) or `analyze` (agentic LLM audit). | No | `scan` |
| `scan_path` | Target repository directory or file path to scan. | No | `.` |
| `provider` | LLM provider for analyze mode: `openai`, `anthropic`, or `gemini`. | No | `openai` |
| `model` | Model identifier override (e.g., `gpt-4o-mini`, `claude-3-5-sonnet-latest`, `gemini-2.5-flash`). | No | `""` |
| `openai_api_key` | Secret API key for OpenAI (required if provider is `openai`). | No | `""` |
| `anthropic_api_key` | Secret API key for Anthropic (required if provider is `anthropic`). | No | `""` |
| `gemini_api_key` | Secret API key for Google Gemini (required if provider is `gemini`). | No | `""` |
| `rules` | Semgrep ruleset configuration (e.g., `auto`, `p/golang`, `p/security-audit`). | No | `auto` |
| `fail_on` | Severity failure threshold: `ERROR`, `WARNING`, or `INFO`. | No | `ERROR` |
| `timeout` | Maximum scan timeout duration (e.g., `5m`, `10m`). | No | `10m` |
| `semgrep_app_token` | Optional Semgrep Cloud Platform App Token. | No | `""` |

### Action Outputs

| Output | Description |
| --- | --- |
| `scan_id` | Unique identifier for the scan execution (e.g., `scan-20260828-190000-abcdef12`). |
| `report_path` | File path to the generated Markdown report (`report.md` or `llm-reports/<scan_id>.md`). |
| `status` | Execution exit status (`success` or `failure`). |

---

## AWS Private ECR Distribution & Execution

`security-analyzer` is published to **AWS Private ECR** in region **`us-west-2`** for enterprise CI/CD integration:

- **Repository URI**: `615471835001.dkr.ecr.us-west-2.amazonaws.com/ajp/security-analyzer`
- **AWS Region**: `us-west-2`
- **Repository ARN**: `arn:aws:ecr:us-west-2:615471835001:repository/ajp/security-analyzer`
- **IAM User**: `arn:aws:iam::615471835001:user/aiengineer` (`AmazonEC2ContainerRegistryPowerUser`)

### Enterprise Consumer Workflow: Fast SAST Scan

Consuming repositories authenticate using AWS OIDC (or IAM access keys) and run the private container directly:

```yaml
name: Security SAST Gate (Private ECR)

on:
  pull_request:
    branches: [ main ]
  push:
    branches: [ main ]

permissions:
  id-token: write # Required for AWS OIDC authentication
  contents: read

jobs:
  sast:
    name: Fast SAST Gate
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Source Code
        uses: actions/checkout@v4

      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: arn:aws:iam::615471835001:role/GitHubConsumer-SecurityAnalyzer-Pull
          aws-region: us-west-2
          audience: sts.amazonaws.com

      - name: Log in to AWS Private ECR
        id: login-ecr
        uses: aws-actions/amazon-ecr-login@v2

      - name: Run Security Analyzer Container
        run: |
          docker run --rm \
            -v "${{ github.workspace }}:/github/workspace" \
            -e GITHUB_STEP_SUMMARY="/github/workspace/step-summary.md" \
            -e INPUT_MODE="scan" \
            -e INPUT_SCAN_PATH="." \
            -e INPUT_RULES="auto" \
            -e INPUT_FAIL_ON="ERROR" \
            -e INPUT_TIMEOUT="5m" \
            615471835001.dkr.ecr.us-west-2.amazonaws.com/ajp/security-analyzer:latest

      - name: Append Job Summary
        if: always()
        run: |
          if [[ -f "${{ github.workspace }}/step-summary.md" ]]; then
            cat "${{ github.workspace }}/step-summary.md" >> $GITHUB_STEP_SUMMARY
            rm -f "${{ github.workspace }}/step-summary.md"
          fi

      - name: Archive SAST Report Artifact
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: sast-report
          path: report.md
          retention-days: 14
```

### Enterprise Consumer Workflow: Deep AI Security Audit

```yaml
name: AI Security Audit (Private ECR)

on:
  schedule:
    - cron: '0 3 * * 1' # Every Monday at 03:00 UTC
  workflow_dispatch:

permissions:
  id-token: write
  contents: read

jobs:
  audit:
    name: Deep LLM Security Analysis
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Source Code
        uses: actions/checkout@v4

      - name: Configure AWS Credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: arn:aws:iam::615471835001:role/GitHubConsumer-SecurityAnalyzer-Pull
          aws-region: us-west-2
          audience: sts.amazonaws.com

      - name: Log in to AWS Private ECR
        uses: aws-actions/amazon-ecr-login@v2

      - name: Run AI Security Analysis
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
        run: |
          docker run --rm \
            -v "${{ github.workspace }}:/github/workspace" \
            -e GITHUB_STEP_SUMMARY="/github/workspace/step-summary.md" \
            -e INPUT_MODE="analyze" \
            -e INPUT_SCAN_PATH="." \
            -e INPUT_PROVIDER="anthropic" \
            -e INPUT_MODEL="claude-3-5-sonnet-latest" \
            -e INPUT_ANTHROPIC_API_KEY="${ANTHROPIC_API_KEY}" \
            -e INPUT_FAIL_ON="ERROR" \
            615471835001.dkr.ecr.us-west-2.amazonaws.com/ajp/security-analyzer:latest

      - name: Append Job Summary
        if: always()
        run: |
          if [[ -f "${{ github.workspace }}/step-summary.md" ]]; then
            cat "${{ github.workspace }}/step-summary.md" >> $GITHUB_STEP_SUMMARY
            rm -f "${{ github.workspace }}/step-summary.md"
          fi

      - name: Archive AI Security Reports
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: ai-security-reports
          path: llm-reports/
          retention-days: 30
```

---

## Command-Line Usage

Once the application is built (e.g., via `make build`), it can be executed in three modes via [main.go](file:///Users/aponte/personal_workspace/repos/security-analyzer/main.go):

1. **Local Scan**:
   ```bash
   ./out/security-analyzer scan <path>
   # or simply:
   ./out/security-analyzer <path>
   ```
   Runs a direct local Semgrep scan against the specified `<path>`. Writes a Markdown report to `report.md` and appends a step summary if running in a GitHub Actions runner.

2. **MCP Server**:
   ```bash
   ./out/security-analyzer mcp
   ```
   Starts the stdio-based Model Context Protocol (MCP) server. This mode allows LLM agents and IDE tools to invoke the `semgrep_scan` tool.

3. **LLM Analysis**:
   ```bash
   ./out/security-analyzer analyze <path>
   ```
   Triggers an LLM-driven security audit. It spawns the MCP server subprocess internally, runs Semgrep scan tools, analyzes the findings using the configured LLM provider, and saves an AI-synthesized audit report to `llm-reports/<scan_id>.md` (while streaming progress to stdout).

---

## Docker Usage

You can build and run the Docker container locally, or pull the private container directly:

```bash
# Build the container image locally
docker build -t ajp-security-analyzer:latest .

# Run a local SAST scan
docker run --rm -v "$(pwd):/github/workspace" ajp-security-analyzer:latest scan .

# Run an AI security audit locally
docker run --rm \
  -v "$(pwd):/github/workspace" \
  -e INPUT_MODE="analyze" \
  -e INPUT_PROVIDER="openai" \
  -e INPUT_OPENAI_API_KEY="$OPENAI_API_KEY" \
  ajp-security-analyzer:latest

# Or pull & run from AWS Private ECR (requires prior ECR login in us-west-2)
docker run --rm \
  -v "$(pwd):/github/workspace" \
  615471835001.dkr.ecr.us-west-2.amazonaws.com/ajp/security-analyzer:latest
```

---

## Documentation & Agent Harness

This project contains a comprehensive documentation harness under [docs/](file:///Users/aponte/personal_workspace/repos/security-analyzer/docs/README.md) and [agent-docs/](file:///Users/aponte/personal_workspace/repos/security-analyzer/agent-docs/PRIVATE-ECR-STRATEGY.md) optimized for developers, AI assistants, and architects:
- **[docs/README.md](file:///Users/aponte/personal_workspace/repos/security-analyzer/docs/README.md)**: Harness index, LLM navigation guide, and architectural design principles.
- **[agent-docs/PRIVATE-ECR-STRATEGY.md](file:///Users/aponte/personal_workspace/repos/security-analyzer/agent-docs/PRIVATE-ECR-STRATEGY.md)**: Production AWS Private ECR deployment architecture, IAM policies, and consumer integration recipes.
- **[agent-docs/GHA-DOCKER.md](file:///Users/aponte/personal_workspace/repos/security-analyzer/agent-docs/GHA-DOCKER.md)**: Containerization, GitHub Actions distribution, and AWS ECR publishing architecture.
- **[docs/specs/docker-gha-integration.md](file:///Users/aponte/personal_workspace/repos/security-analyzer/docs/specs/docker-gha-integration.md)**: Architectural specification for Docker packaging, GitHub Action interface, and AWS ECR distribution.
- **[docs/specs/llm-integration.md](file:///Users/aponte/personal_workspace/repos/security-analyzer/docs/specs/llm-integration.md)**: Architectural specification for LLM integration, multi-provider abstraction (`OpenAI`, `Anthropic`, `Gemini`), MCP client subprocess model, and agentic analysis engine.
- **[docs/specs/semgrep-integration.md](file:///Users/aponte/personal_workspace/repos/security-analyzer/docs/specs/semgrep-integration.md)**: Architectural specification for Semgrep SAST scanner, MCP server mode, path traversal sandboxing, and reporting.
- **[AGENTS.md](file:///Users/aponte/personal_workspace/repos/security-analyzer/AGENTS.md)**: Instructions, commands, and rules for autonomous AI coding agents.

---

## Configuration

The application is configured using environment variables (which are loaded by [pkg/config/config.go](file:///Users/aponte/personal_workspace/repos/security-analyzer/pkg/config/config.go) and processed in [main.go](file:///Users/aponte/personal_workspace/repos/security-analyzer/main.go)). You can also place these variables in a `.env` file at the root of the workspace.

### LLM Configurations (for `analyze` mode)

| Variable | Description | Default |
| --- | --- | --- |
| `LLM_PROVIDER` | The LLM API provider (`openai`, `anthropic`, `gemini`). | `openai` |
| `LLM_MODEL` | The specific model identifier (`gpt-4o-mini`, `claude-3-5-sonnet-latest`, `gemini-2.5-flash`). | Provider default |
| `OPENAI_API_KEY` | Secret API key for OpenAI. | *(Required if using OpenAI)* |
| `ANTHROPIC_API_KEY` | Secret API key for Anthropic. | *(Required if using Anthropic)* |
| `GEMINI_API_KEY` | Secret API key for Google Gemini. | *(Required if using Gemini)* |

### Semgrep Configurations

| Variable | Description | Default |
| --- | --- | --- |
| `SEMGREP_RULES` | Semgrep rules config (e.g., `p/golang`, `auto`). | `auto` |
| `SEMGREP_FAIL_ON` | Severity level at which the build fails (`ERROR`, `WARNING`, `INFO`). | `ERROR` |
| `SEMGREP_TIMEOUT` | Maximum duration allowed for the Semgrep command (e.g., `5m`, `10m`). | `10m` |
| `SEMGREP_APP_TOKEN` | Optional application token for scanning with Semgrep App. | *(Optional)* |

---

## Development

This project uses a Makefile to manage build, run, lint, and formatting tasks.

### Available Make Commands

Below are the primary `make` commands available for local development:

- **`make build`**: Compiles the Go application and generates an executable under `./out/security-analyzer`.
- **`make run`**: Compiles and executes the Go application.
- **`make test`**: Runs all unit tests for the codebase.
- **`make lint`**: Runs the [golangci-lint](https://golangci-lint.run/) linter against the codebase to check for style violations and potential issues.
- **`make clean`**: Removes build artifacts and temporary files (such as the `./out` directory and coverage reports).

For a complete list of all make targets, run:
```bash
make help
```

---

## Continuous Integration & Publishing

- **CI Pipeline ([.github/workflows/ci.yml](file:///Users/aponte/personal_workspace/repos/security-analyzer/.github/workflows/ci.yml))**: Automatically runs linting, unit tests, and Go builds on push and pull requests to `main`.
- **Publish Pipeline ([.github/workflows/publish-ecr.yml](file:///Users/aponte/personal_workspace/repos/security-analyzer/.github/workflows/publish-ecr.yml))**: Builds multi-stage container images and publishes to AWS Private ECR (`615471835001.dkr.ecr.us-west-2.amazonaws.com/ajp/security-analyzer` in `us-west-2`) with semantic version tags. Requires repository secrets `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` for IAM user `aiengineer`.
