# security-analyzer

LLM-based security scans.

## Development

This project uses a Makefile to manage build, run, lint, and formatting tasks.

### Available Make Commands

Below are the primary `make` commands available for local development:

- **`make build`**: Compiles the Go application and generates an executable under `./out/security-analyzer`.
- **`make run`**: Compiles and executes the Go application.
- **`make lint`**: Runs the [golangci-lint](https://golangci-lint.run/) linter against the codebase to check for style violations and potential issues.
- **`make clean`**: Removes build artifacts and temporary files (such as the `./out` directory and coverage reports).

For a complete list of all make targets, run:
```bash
make help
```

## Continuous Integration (CI)

A GitHub Actions CI workflow is configured in `.github/workflows/ci.yml`.

### Workflow Triggers

The CI pipeline runs on:
- All **pushed commits** to any branch.
- All **pull requests** opened or updated.

### Workflow Jobs

To ensure fast feedback, the workflow splits tasks into two parallel jobs:

1. **Lint (`Lint` job)**:
   - Configures the Go environment (version `1.25.x`).
   - Runs `golangci-lint` to check code quality and style conventions.

2. **Build (`Build` job)**:
   - Configures the Go environment (version `1.25.x`).
   - Executes `make build` to verify the codebase compiles successfully.
