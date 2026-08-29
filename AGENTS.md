# AGENTS.md for security-analyzer

This guide provides instructions for building, running, testing, and developing within the `security-analyzer` repository.

## Documentation & Agent Harness

This project contains a comprehensive agent documentation harness under the [docs/](file:///Users/aponte/personal_workspace/repos/security-analyzer/docs/README.md) folder optimized for AI coding assistants and architects:
- **[Documentation Harness Index](file:///Users/aponte/personal_workspace/repos/security-analyzer/docs/README.md)**: Sitemap, LLM navigation instructions, and architectural principles.
- **[LLM Integration Specification](file:///Users/aponte/personal_workspace/repos/security-analyzer/docs/specs/llm-integration.md)**: Multi-provider abstraction (`OpenAI`, `Anthropic`, `Gemini`), MCP client subprocess model, agentic loop (`pkg/analyzer`), and report generation.
- **[Semgrep Integration Specification](file:///Users/aponte/personal_workspace/repos/security-analyzer/docs/specs/semgrep-integration.md)**: SAST scanner integration, JSON parsing, MCP tool server (`pkg/mcp`), and path traversal containment.

---

## Commands

### Build and Clean
- **Build application**: `make build` (creates executable in `out/security-analyzer`)
- **Clean build artifacts**: `make clean`
- **Tidy Go modules**: `make tidy`

### Test and Quality
- **Run unit tests**: `make test`
- **Format code**: `make fmt` (runs `goimports` and `gofumpt`)
- **Lint code**: `make lint` (runs `golangci-lint`)
- **Vet code**: `make vet` (runs `go vet`)

---

## Security & PII Guidelines

1. **No Real PII**: Do not hardcode, commit, or check in real datasets, transaction records, or any production database dumps to this repository.
2. **Log Safety**: Never include real PII (such as account details, names, sensitive description fields, or exact balances) in logs. Keep `slog` log fields generic and aggregate.
3. **Use Synthetic Data for Dev/Test**: Always use the built-in synthetic generators to produce non-sensitive mock datasets for testing and local environment setups:
4. **Environment Variables**: Never hardcode credentials. Ensure configurations are fetched from the environment:

---

## Code Guidelines & Formatting

- **Formatting**: We enforce strict formatting rules. Always run `make fmt` before committing.
- **Function Comments**: Write clear comments on exported package functions and structs.
- **Inline Comments**: Use inline comments sparsely and only when describing complex business logic or edge cases.
- **Testing**: Always run `make unit-test` after making changes to verify correctness.

