# security-analyzer

LLM-based security scans.

## Command-Line Usage

Once the application is built (e.g., via `make build`), it can be executed in three modes via [main.go](file:///Users/aponte/personal_workspace/security-analyzer/main.go):

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
   Triggers an LLM-driven security audit. It spawns the MCP server subprocess internally, runs Semgrep scan tools, analyzes the findings using the configured LLM provider, and prints/saves an AI-synthesized audit report to `llm-report.md`.

---

## Documentation & Agent Harness

This project contains a comprehensive documentation harness under [docs/](file:///Users/aponte/personal_workspace/repos/security-analyzer/docs/README.md) optimized for developers, AI assistants, and architects:
- **[docs/README.md](file:///Users/aponte/personal_workspace/repos/security-analyzer/docs/README.md)**: Harness index, LLM navigation guide, and architectural design principles.
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

## Continuous Integration (CI)

A GitHub Actions CI workflow is configured in [.github/workflows/ci.yml](file:///Users/aponte/personal_workspace/security-analyzer/.github/workflows/ci.yml).

### Workflow Triggers

The CI pipeline runs on:
- **Pushes** to the `main` branch.
- **Pull requests** targeting the `main` branch.

### Workflow Pipeline

The workflow runs a single sequential **`build`** job on an Ubuntu runner:

1. **Checkout**: Checks out the repository code.
2. **Go Setup**: Configures the Go environment (version `1.25`).
3. **Lint**: Runs `golangci-lint` using `golangci/golangci-lint-action@v9` (configured with version `v2.9.0`) to inspect code quality and conventions.
4. **Build**: Executes `make build` to verify the application compiles successfully and outputs the binary.
5. **Test**: Executes `make test` to run all unit tests and verify correctness.
