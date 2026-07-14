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
