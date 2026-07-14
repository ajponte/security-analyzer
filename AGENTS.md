# AGENTS.md for security-analyzer

This guide provides instructions for building, running, testing, and developing within the `security-analyzer` repository.

## Documentation & Agent Harness

This project contains a comprehensive agent documentation harness under the [docs/](docs) folder. Refer to these files for deeper context.

---

## Commands

### Build and Clean
- **Build application**: `make build` (creates executable in `out/data-loader`)
- **Clean build artifacts**: `make clean`
- **Tidy Go modules**: `make tidy`
- **Vendoring dependencies**: `make vendor`


### Test and Quality
- **All Quality Checks (Lint, Format)**
- **Run unit tests**: `make unit-test`
- **Run tests with JSON output (CI)**: `make test-ci`
- **Show test coverage in HTML**: `make coverage`
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

