# Architectural Specification: Docker Packaging, GitHub Action Interface, and AWS ECR Distribution Architecture

## 1. Overview & Context

This specification defines the formal architectural design, container encapsulation model, GitHub Action interface, and AWS Elastic Container Registry (ECR) distribution strategy for **`ajp-security-analyzer`**.

`ajp-security-analyzer` is a cloud-native security analysis engine combining deterministic Static Application Security Testing (SAST) via Semgrep with autonomous, multi-turn LLM security audits powered by the Model Context Protocol (MCP). Packaging and distributing this system as a containerized GitHub Action eliminates runner configuration overhead, guarantees deterministic toolchains (`go 1.25`, `python 3.11`, `semgrep 1.175.0`), isolates runtime execution environments, and standardizes security reporting across heterogeneous CI/CD pipelines.

### Primary Goals
- **Zero-Dependency Runner Execution**: Run seamlessly on standard GitHub-hosted (`ubuntu-latest`) and self-hosted runners without pre-installed Go, Python, or Semgrep runtimes.
- **Dual-Mode System Topology**: Support both high-throughput deterministic SAST gates (`scan` mode) and deep contextual AI audits (`analyze` mode) through a single unified interface.
- **Strict Container Isolation & Containment**: Enforce file-system boundaries, path traversal protections, and safe workspace access under GitHub Actions runner UID/GID mappings.
- **Phased Regional ECR Distribution**: Provide zero-auth anonymous pulls via AWS Public ECR (`us-east-1`) in Phase 1, and migrate to authenticated enterprise distribution via AWS Private ECR in `us-west-2` with AWS IAM OpenID Connect (OIDC) federation in Phase 2.
- **Automated CI/CD Release Publishing**: Orchestrate multi-architecture Buildx image builds, layer caching, semantic version tagging, and automated ECR publishing via GitHub Actions OIDC workflows.

---

## 2. System Topology & Dual-Mode Execution Model

The system operates under a dual-mode topology designed to serve two distinct stages of the software development lifecycle: pull-request blocking SAST scans and asynchronous deep AI security audits.

```mermaid
flowchart TB
    subgraph CI_Runner ["GitHub Actions Runner Environment"]
        Trigger["GitHub Event (PR / Push / Schedule / Dispatch)"]
        GHA_Action["ajp-security-analyzer Action (action.yml)"]
        Workspace["Mounted Workspace (/github/workspace)"]
        SummaryFile["$GITHUB_STEP_SUMMARY"]
        OutputFile["$GITHUB_OUTPUT"]
    end

    subgraph Container ["Docker Container Runtime (ajp-security-analyzer)"]
        Entrypoint["entrypoint.sh (Lifecycle Orchestrator)"]
        GoBinary["security-analyzer Binary (/usr/local/bin/security-analyzer)"]
        
        subgraph Mode_Scan ["Mode: scan (Fast Deterministic SAST)"]
            SemgrepDirect["Semgrep SAST CLI (Subprocess)"]
            ReportGen["pkg/report (Markdown & Step Summary)"]
        end

        subgraph Mode_Analyze ["Mode: analyze (Multi-Turn LLM Audit)"]
            Analyzer["pkg/analyzer (Agentic Orchestrator)"]
            MCP_Client["pkg/mcp Client"]
            MCP_Server["security-analyzer mcp (Subprocess Server)"]
            LLM_Factory["pkg/llm Factory (OpenAI / Anthropic / Gemini)"]
            SemgrepTool["semgrep_scan MCP Tool"]
        end
    end

    subgraph External_APIs ["External LLM Providers"]
        OpenAI["OpenAI API"]
        Anthropic["Anthropic API"]
        Gemini["Google Gemini API"]
    end

    subgraph Output_Artifacts ["Generated Artifacts"]
        LocalReport["report.md"]
        LLMReport["llm-reports/<scan_id>.md"]
    end

    Trigger --> GHA_Action
    GHA_Action -->|Mounts Workspace & Envs| Entrypoint
    Entrypoint --> GoBinary

    GoBinary -->|INPUT_MODE=scan| SemgrepDirect
    SemgrepDirect -->|Raw JSON Results| ReportGen
    ReportGen --> SummaryFile
    ReportGen --> LocalReport

    GoBinary -->|INPUT_MODE=analyze| Analyzer
    Analyzer --> MCP_Client
    MCP_Client <-->|Stdio JSON-RPC| MCP_Server
    MCP_Server --> SemgrepTool
    SemgrepTool --> SemgrepDirect
    Analyzer <-->|GenerateResponse| LLM_Factory
    LLM_Factory <--> OpenAI & Anthropic & Gemini
    Analyzer --> LLMReport
    Analyzer --> SummaryFile

    Entrypoint --> OutputFile
```

### Execution Mode Comparison

| Attribute | `scan` Mode (Deterministic SAST) | `analyze` Mode (Agentic AI Audit) |
| :--- | :--- | :--- |
| **Primary Use Case** | Fast PR merge blocking, branch protection gates, pre-commit checks | Scheduled security reviews, major release audits, deep triage |
| **Execution Latency** | Sub-minute (typically 5–30 seconds) | 30–120 seconds (multi-turn model inference) |
| **External Dependencies** | None (100% local execution) | External LLM Provider API (`OPENAI_API_KEY`, etc.) |
| **Subprocess Architecture** | Single subprocess: `semgrep scan --json` | Dual subprocess: MCP Stdio Server + Semgrep CLI |
| **Output Artifacts** | `report.md`, `$GITHUB_STEP_SUMMARY` | `llm-reports/<scan_id>.md`, `$GITHUB_STEP_SUMMARY` |
| **Failure Semantics** | Evaluates findings against `INPUT_FAIL_ON` threshold (`ERROR`, `WARNING`, `INFO`) | Returns non-zero on unhandled analyzer errors or threshold violations |
| **Resource Profile** | Low CPU/Memory footprint (< 512MB RAM) | Moderate CPU/Memory footprint (1–2GB RAM during tool chaining) |

---

## 3. Multi-Stage Container Architecture

To satisfy strict performance and security constraints, the container is built using a multi-stage Docker build pipeline:

1. **Stage 1 (`builder`)**: Compiles a static Go binary using `golang:1.25-alpine` with all debug symbols stripped and CGO disabled.
2. **Stage 2 (`runtime`)**: Employs `python:3.11-slim-bookworm` containing runtime toolchains (Semgrep CLI, Git, CA Certificates, JQ, Curl) while discarding Go build tools and intermediate source code.

```mermaid
flowchart LR
    subgraph Stage1 ["Stage 1: Static Builder (golang:1.25-alpine)"]
        Source["Go Source (*.go, go.mod, go.sum)"]
        ModCache["BuildKit Cache: /go/pkg/mod"]
        BuildCache["BuildKit Cache: /root/.cache/go-build"]
        GoCompiler["CGO_ENABLED=0 go build -trimpath -ldflags='-s -w'"]
        StaticBin["/out/security-analyzer (Statically Linked Binary)"]

        Source --> GoCompiler
        ModCache --> GoCompiler
        BuildCache --> GoCompiler
        GoCompiler --> StaticBin
    end

    subgraph Stage2 ["Stage 2: Runtime Image (python:3.11-slim-bookworm)"]
        DebianBase["Base OS: Debian Bookworm Slim"]
        SysDeps["Runtime Packages: git, ca-certificates, curl, jq, bash"]
        PipSemgrep["pip install semgrep==1.175.0"]
        GitSafe["git config --global --add safe.directory '*'"]
        EntrypointScript["entrypoint.sh"]

        DebianBase --> SysDeps
        SysDeps --> PipSemgrep
        PipSemgrep --> GitSafe
    end

    StaticBin -->|COPY --from=builder| Stage2
    EntrypointScript -->|COPY| Stage2
```

### Complete Dockerfile Specification ([Dockerfile](file:///Users/aponte/personal_workspace/repos/security-analyzer/Dockerfile))

```dockerfile
# ==========================================
# Stage 1: Build Static Go Binary
# ==========================================
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Install build dependencies for static linking and git metadata
RUN apk add --no-cache git ca-certificates tzdata

# Cache Go module dependencies
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download -x

# Copy source tree
COPY . .

# Build statically compiled binary (CGO disabled, stripped debug symbols)
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w -extldflags '-static'" -o /out/security-analyzer main.go

# ==========================================
# Stage 2: Runtime Image (Python 3.11 + Semgrep)
# ==========================================
FROM python:3.11-slim-bookworm AS runtime

LABEL org.opencontainers.image.title="ajp-security-analyzer" \
      org.opencontainers.image.description="LLM-based security scanner and Semgrep SAST orchestrator for GitHub Actions" \
      org.opencontainers.image.source="https://github.com/ajponte/security-analyzer" \
      org.opencontainers.image.licenses="MIT"

# Set environment variables for non-interactive execution and Python optimization
ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PATH="/usr/local/bin:/usr/bin:/bin:${PATH}" \
    SEMGREP_ENABLE_VERSION_CHECK=0

# Install runtime dependencies: Git (required for Semgrep repo discovery), CA certs, Curl, JQ, Bash
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    git \
    curl \
    jq \
    bash \
    && rm -rf /var/lib/apt/lists/*

# Install Semgrep CLI via pip into global Python environment
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install --no-cache-dir --upgrade pip setuptools && \
    pip install --no-cache-dir semgrep==1.175.0

# Configure global git safe directory to avoid permissions mismatch in GHA workspace mounts
RUN git config --global --add safe.directory '*'

# Copy statically linked binary from builder stage
COPY --from=builder /out/security-analyzer /usr/local/bin/security-analyzer
RUN chmod +x /usr/local/bin/security-analyzer

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Default working directory mapped by GitHub Actions
WORKDIR /github/workspace

ENTRYPOINT ["/entrypoint.sh"]
```

### Container Hardening & Optimization Invariants
1. **Static Binary Compilation**: `CGO_ENABLED=0` coupled with `-extldflags '-static'` guarantees zero dynamic linking dependencies (`libc`, `musl`, `glibc`), allowing the binary to execute reliably in any Linux runtime environment.
2. **BuildKit Layer Caching**: Utilizes `--mount=type=cache,target=/go/pkg/mod` and `/root/.cache/go-build` to enable sub-second re-compilations during local development and CI runs.
3. **Deterministic Toolchain Pinning**: `semgrep` is pinned to `1.175.0` to guard against breaking CLI flag modifications or AST rule evaluation changes across releases.
4. **Git Safe Directory Containment**: Running inside container runners often maps `/github/workspace` with ownership belonging to host UID `1001` while the container runs as `root`. Executing `git config --global --add safe.directory '*'` neutralizes `fatal: detected dubious ownership in repository` errors during Semgrep differential analysis.
5. **No Telemetry / Offline Execution**: `SEMGREP_ENABLE_VERSION_CHECK=0` prevents outbound version polling overhead and preserves runner network privacy.

---

## 4. GitHub Action Interface & Execution Contract

### 4.1 Action Metadata Contract ([action.yml](file:///Users/aponte/personal_workspace/repos/security-analyzer/action.yml))

The GitHub Action definition defines the inputs, outputs, branding, and container runtime bindings:

```yaml
name: 'Security Analyzer'
description: 'AI-driven SAST scanner combining Semgrep analysis and LLM security audits'
author: 'ajponte'
branding:
  icon: 'shield'
  color: 'blue'

inputs:
  mode:
    description: 'Execution mode: "scan" (direct Semgrep SAST) or "analyze" (agentic LLM audit)'
    required: false
    default: 'scan'
  scan_path:
    description: 'Target repository directory or file path to scan'
    required: false
    default: '.'
  provider:
    description: 'LLM Provider for analyze mode: "openai", "anthropic", or "gemini"'
    required: false
    default: 'openai'
  model:
    description: 'Model identifier override (e.g., "gpt-4o-mini", "claude-3-5-sonnet-latest", "gemini-2.5-flash")'
    required: false
    default: ''
  openai_api_key:
    description: 'API key for OpenAI (required if provider is openai)'
    required: false
    default: ''
  anthropic_api_key:
    description: 'API key for Anthropic (required if provider is anthropic)'
    required: false
    default: ''
  gemini_api_key:
    description: 'API key for Google Gemini (required if provider is gemini)'
    required: false
    default: ''
  rules:
    description: 'Semgrep ruleset configuration (e.g., "auto", "p/golang", "p/security-audit")'
    required: false
    default: 'auto'
  fail_on:
    description: 'Severity failure threshold: "ERROR", "WARNING", or "INFO"'
    required: false
    default: 'ERROR'
  timeout:
    description: 'Maximum scan timeout duration (e.g., "5m", "10m")'
    required: false
    default: '10m'
  semgrep_app_token:
    description: 'Optional Semgrep Cloud Platform App Token'
    required: false
    default: ''

outputs:
  scan_id:
    description: 'Unique identifier for the scan execution'
  report_path:
    description: 'Path to generated Markdown report file'
  status:
    description: 'Execution exit status ("success" or "failure")'

runs:
  using: 'docker'
  image: 'Dockerfile'
  env:
    INPUT_MODE: ${{ inputs.mode }}
    INPUT_SCAN_PATH: ${{ inputs.scan_path }}
    INPUT_PROVIDER: ${{ inputs.provider }}
    INPUT_MODEL: ${{ inputs.model }}
    INPUT_OPENAI_API_KEY: ${{ inputs.openai_api_key }}
    INPUT_ANTHROPIC_API_KEY: ${{ inputs.anthropic_api_key }}
    INPUT_GEMINI_API_KEY: ${{ inputs.gemini_api_key }}
    INPUT_RULES: ${{ inputs.rules }}
    INPUT_FAIL_ON: ${{ inputs.fail_on }}
    INPUT_TIMEOUT: ${{ inputs.timeout }}
    INPUT_SEMGREP_APP_TOKEN: ${{ inputs.semgrep_app_token }}
```

### 4.2 Entrypoint Lifecycle Architecture ([entrypoint.sh](file:///Users/aponte/personal_workspace/repos/security-analyzer/entrypoint.sh))

The entrypoint acts as the orchestration layer between GitHub Actions runner input conventions (`INPUT_*`) and the Go application's configuration model ([pkg/config/config.go](file:///Users/aponte/personal_workspace/repos/security-analyzer/pkg/config/config.go)).

```mermaid
flowchart TD
    Start(["entrypoint.sh Invocation"]) --> Banner["1. print_banner()"]
    Banner --> ExpSemgrep["2. export_semgrep_config()<br/>(SEMGREP_RULES, SEMGREP_FAIL_ON, SEMGREP_TIMEOUT)"]
    ExpSemgrep --> ExpLLM["3. export_llm_config()<br/>(LLM_PROVIDER, LLM_MODEL, API_KEYS)"]
    ExpLLM --> LogParam["4. log_parameters()<br/>(Sanitized audit log to stdout)"]
    LogParam --> Exec["5. execute_analyzer()<br/>(/usr/local/bin/security-analyzer $mode $scan_path)"]
    Exec --> Resolve["6. resolve_report_artifacts()<br/>(Find latest scan_id & report path)"]
    Resolve --> ExportOut["7. export_step_outputs()<br/>(Write scan_id, report_path, status to $GITHUB_OUTPUT)"]
    ExportOut --> CheckExit{"Exit Code == 0?"}
    CheckExit -->|Yes| Success(["Exit 0 (Success Banner)"])
    CheckExit -->|No| Failure(["Exit ${exit_code} (Failure Message)"])
```

#### Functional Decomposition of `entrypoint.sh`

1. **`print_banner`**: Prints clean initialization headers for CI log visibility.
2. **`export_semgrep_config`**: Maps `INPUT_RULES`, `INPUT_FAIL_ON`, `INPUT_TIMEOUT`, and `INPUT_SEMGREP_APP_TOKEN` to system environment variables.
3. **`export_llm_config`**: Routes provider credentials (`OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`) and model identifiers into the runtime environment.
4. **`log_parameters`**: Audits execution mode, scan targets, rulesets, and provider configurations without printing sensitive API keys.
5. **`execute_analyzer`**: Executes `/usr/local/bin/security-analyzer`, capturing the exact exit code while temporarily disabling `set -e` to prevent abrupt script termination prior to output artifact discovery.
6. **`resolve_report_artifacts`**:
   - In `analyze` mode: Inspects `llm-reports/` for the latest `scan-YYYYMMDD-HHMMSS-*.md` report and extracts the `scan_id`.
   - In `scan` mode: Standardizes on `report.md`.
7. **`export_step_outputs`**: Safely writes `scan_id`, `report_path`, and `status` to `$GITHUB_OUTPUT` if the runner environment variable is defined.
8. **`main`**: Enforces error handling, invokes lifecycle functions sequentially, and propagates the binary's terminal exit code.

---

## 5. AWS ECR Phased Distribution Architecture

Distribution is structured across two phases balancing immediate friction-free public adoption with enterprise-grade private infrastructure governance.

```mermaid
flowchart TB
    subgraph Registry_Phase1 ["Phase 1: AWS Public ECR (Current)"]
        PublicAuth["Auth Endpoint (us-east-1 Required)"]
        PublicURI[("public.ecr.aws/<alias>/ajp-security-analyzer")]
        PublicConsumers["Public / OSS / External Repositories"]

        PublicAuth --> PublicURI
        PublicURI -->|"Zero-Auth Anonymous Pull (docker pull)"| PublicConsumers
    end

    subgraph Registry_Phase2 ["Phase 2: AWS Private ECR (Enterprise Migration)"]
        PrivateAuth["AWS OIDC Federation (sts:AssumeRoleWithWebIdentity)"]
        PrivateURI[("AWS Private ECR (us-west-2)<br/><account-id>.dkr.ecr.us-west-2.amazonaws.com/ajp-security-analyzer")]
        PrivateConsumers["Internal Org Repositories / Regulated VPCs"]

        PrivateAuth --> PrivateURI
        PrivateURI -->|"Authenticated IAM Role Pull"| PrivateConsumers
    end
```

### Regional Topology Rationale

```
+-----------------------------------------------------------------------------------+
|                              REGIONAL TOPOLOGY                                    |
+-----------------------------------------------------------------------------------+
|  AWS Region: us-east-1 (N. Virginia)                                              |
|  - AWS Public ECR API Control Plane & Authentication Token Endpoint               |
|  - Required for: `aws-actions/amazon-ecr-login` with `registry-type: public`      |
|  - Target Registry: `public.ecr.aws/<alias>/ajp-security-analyzer`                |
+-----------------------------------------------------------------------------------+
|  AWS Region: us-west-2 (Oregon)                                                   |
|  - Primary Enterprise Workload, VPC Endpoints & Private ECR Storage               |
|  - Required for: Enterprise IAM Role-to-Assume, KMS CMK Encryption, Inspector     |
|  - Target Registry: `<account-id>.dkr.ecr.us-west-2.amazonaws.com/ajp-security-analyzer` |
+-----------------------------------------------------------------------------------+
```

### 5.1 Phase 1: AWS Public ECR Distribution (Current)
- **Registry URI**: `public.ecr.aws/<alias>/ajp-security-analyzer`
- **Authentication**: **Zero-Auth Anonymous Pulls** for all consuming GitHub Actions workflows. No AWS credentials or secrets required in target repositories.
- **Publisher Authentication**: The publishing CI pipeline authenticates against the AWS Public ECR API endpoint, which is strictly located in **`us-east-1`**.
- **Marketplace Compatibility**: Consuming repositories invoke the action directly via standard GitHub Action syntax (`uses: ajponte/security-analyzer@v1`).

### 5.2 Phase 2: AWS Private ECR Migration Architecture
- **Registry URI**: `<account-id>.dkr.ecr.us-west-2.amazonaws.com/ajp-security-analyzer`
- **Region**: Primary internal enterprise region **`us-west-2`**.
- **Authentication Model**: Passwordless **AWS OIDC Federation** (`sts:AssumeRoleWithWebIdentity`) using GitHub Actions OIDC provider.
- **Security & Compliance Features**:
  - **AWS KMS CMK Encryption**: Encrypt image layers at rest using dedicated AWS KMS customer-managed keys.
  - **Enhanced Vulnerability Scanning**: Continuous CVE detection powered by AWS Inspector and Clair scanning.
  - **VPC Endpoint Isolation**: Support air-gapped pulls from self-hosted runners via AWS PrivateLink (`com.amazonaws.us-west-2.ecr.dkr`).

### Strategic Migration Triggers & Comparison

| Characteristic | Phase 1: AWS Public ECR | Phase 2: AWS Private ECR |
| :--- | :--- | :--- |
| **Target Audience** | Open-source, public, and multi-organization repositories | Internal enterprise repositories, regulated VPCs |
| **Authentication** | Anonymous (No credentials needed) | AWS IAM OIDC Role (`aws-actions/configure-aws-credentials`) |
| **Registry Location** | Global Public Gallery (Auth in `us-east-1`) | Private Regional ECR in `us-west-2` |
| **Vulnerability Scanning** | Basic ECR scan on push | AWS Inspector continuous runtime CVE scanning |
| **Network Egress** | Public internet access required | Supports private VPC Endpoints (`PrivateLink`) |
| **Data Transfer Quotas** | Free tier public quota | Governed by enterprise AWS data transfer policies |

---

## 6. CI/CD Publishing Pipeline

The container publishing pipeline is automated via [.github/workflows/publish-ecr.yml](file:///Users/aponte/personal_workspace/repos/security-analyzer/.github/workflows/publish-ecr.yml).

```mermaid
sequenceDiagram
    autonumber
    participant Git as GitHub (Push to main / Tag v*.*.*)
    participant GHA as GitHub Actions Runner
    participant STS as AWS Security Token Service (STS us-east-1)
    participant IAM as AWS IAM Role (GitHubActions-ECR-Publisher)
    participant ECR as AWS Public ECR (public.ecr.aws)

    Git->>GHA: Trigger Workflow (publish-ecr.yml)
    GHA->>GHA: Checkout Code & Setup Docker Buildx
    GHA->>GHA: Build Test Image & Run Smoke Tests
    
    rect rgb(240, 248, 255)
        Note over GHA,STS: OIDC Passwordless Authentication
        GHA->>STS: Request Token (AssumeRoleWithWebIdentity)
        STS->>IAM: Validate JWT Claims (iss, aud, sub: repo:ajponte/security-analyzer:*)
        IAM-->>STS: Generate Scoped Temporary Credentials (1 Hour)
        STS-->>GHA: Return AWS_ACCESS_KEY_ID, SECRET, SESSION_TOKEN
    end

    GHA->>ECR: Login (aws-actions/amazon-ecr-login --registry-type public)
    GHA->>GHA: Calculate Multi-Tags (latest, sha-xxxxxxx, v1.2.3, v1.2, v1)
    GHA->>ECR: Build & Push Multi-Arch Image Layers
    ECR-->>GHA: Confirm Digest & Manifest Published
```

### Complete Publishing Workflow ([.github/workflows/publish-ecr.yml](file:///Users/aponte/personal_workspace/repos/security-analyzer/.github/workflows/publish-ecr.yml))

```yaml
name: Build & Publish Container to AWS ECR

on:
  push:
    branches:
      - main
    tags:
      - 'v*.*.*'
  pull_request:
    branches:
      - main
  workflow_dispatch:

permissions:
  id-token: write # Required for requesting AWS OIDC Web Identity Token
  contents: read  # Required for actions/checkout

env:
  AWS_REGION: us-east-1 # AWS Public ECR API endpoint requires us-east-1
  PUBLIC_ECR_REGISTRY: public.ecr.aws/a1b2c3d4
  IMAGE_NAME: ajp-security-analyzer
  ROLE_TO_ASSUME: arn:aws:iam::123456789012:role/GitHubActions-ECR-Publisher

jobs:
  build-and-test:
    name: Build & Validate Image
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Source Code
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Build Test Image
        uses: docker/build-push-action@v5
        with:
          context: .
          file: ./Dockerfile
          push: false
          tags: ${{ env.IMAGE_NAME }}:test
          load: true

      - name: Smoke Test Image Execution
        run: |
          docker run --rm ${{ env.IMAGE_NAME }}:test --help || true

  publish-public-ecr:
    name: Publish to AWS Public ECR
    needs: build-and-test
    if: github.event_name != 'pull_request'
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Source Code
        uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Configure AWS Credentials via OIDC
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ env.ROLE_TO_ASSUME }}
          aws-region: ${{ env.AWS_REGION }}
          audience: sts.amazonaws.com

      - name: Login to AWS Public ECR
        uses: aws-actions/amazon-ecr-login@v2
        with:
          registry-type: public

      - name: Extract Docker Metadata (Tags & Labels)
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.PUBLIC_ECR_REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=raw,value=latest,enable=${{ github.ref == 'refs/heads/main' }}
            type=sha,prefix=sha-,format=short
            type=semver,pattern={{version}}
            type=semver,pattern={{major}}.{{minor}}
            type=semver,pattern={{major}}

      - name: Build & Push Image to Public ECR
        uses: docker/build-push-action@v5
        with:
          context: .
          file: ./Dockerfile
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

### Multi-Tagging Strategy

```
Repository Release Event: git tag v1.4.2 && git push origin v1.4.2
  ├── public.ecr.aws/<alias>/ajp-security-analyzer:v1.4.2    (Exact Immutable Release)
  ├── public.ecr.aws/<alias>/ajp-security-analyzer:v1.4      (Minor Floating Release)
  ├── public.ecr.aws/<alias>/ajp-security-analyzer:v1        (Major Floating Release)
  ├── public.ecr.aws/<alias>/ajp-security-analyzer:sha-a1b2c3d (Immutable Commit SHA)
  └── public.ecr.aws/<alias>/ajp-security-analyzer:latest    (Latest Mainline Branch Build)
```

---

## 7. Consumer Workflow Patterns & Integration Blueprints

### 7.1 Pattern A: Zero-Auth Fast SAST PR Gate (Phase 1 Public ECR)

Consuming repositories invoke the action directly without needing AWS credentials:

```yaml
name: Security Scan

on:
  pull_request:
    branches: [ main ]
  push:
    branches: [ main ]

jobs:
  sast:
    name: Fast SAST Gate
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Source Code
        uses: actions/checkout@v4

      - name: Run Security Analyzer
        id: security_scan
        uses: ajponte/security-analyzer@v1
        with:
          mode: 'scan'
          scan_path: '.'
          rules: 'auto'
          fail_on: 'ERROR'
          timeout: '5m'

      - name: Archive SAST Report Artifact
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: sast-report
          path: ${{ steps.security_scan.outputs.report_path }}
          retention-days: 14
```

### 7.2 Pattern B: Deep AI Security Audit with Dynamic Artifact Persistence

```yaml
name: AI Security Audit

on:
  schedule:
    - cron: '0 4 * * 1' # Weekly on Monday at 04:00 UTC
  workflow_dispatch:

jobs:
  audit:
    name: Autonomous LLM Security Audit
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Source Code
        uses: actions/checkout@v4

      - name: Run Deep AI Security Analysis
        id: ai_audit
        uses: ajponte/security-analyzer@v1
        with:
          mode: 'analyze'
          scan_path: '.'
          provider: 'anthropic'
          model: 'claude-3-5-sonnet-latest'
          anthropic_api_key: ${{ secrets.ANTHROPIC_API_KEY }}
          fail_on: 'ERROR'

      - name: Archive Unique AI Audit Report
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: ai-audit-${{ steps.ai_audit.outputs.scan_id }}
          path: ${{ steps.ai_audit.outputs.report_path }}
          retention-days: 30

      - name: Post Audit Findings to PR
        if: github.event_name == 'pull_request' && always()
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const reportPath = '${{ steps.ai_audit.outputs.report_path }}';
            if (fs.existsSync(reportPath)) {
              const content = fs.readFileSync(reportPath, 'utf8');
              github.rest.issues.createComment({
                issue_number: context.issue.number,
                owner: context.repo.owner,
                repo: context.repo.repo,
                body: `## 🛡️ AI Security Audit Report (${{ steps.ai_audit.outputs.scan_id }})\n\n${content}`
              });
            }
```

### 7.3 Pattern C: Enterprise Private ECR Consumer Workflow (Phase 2)

For enterprise repositories pulling from a private registry in `us-west-2`:

```yaml
name: Enterprise Private SAST

on:
  pull_request:
    branches: [ main ]

permissions:
  id-token: write # Required for AWS OIDC
  contents: read

jobs:
  scan:
    name: Private Container Scan
    runs-on: ubuntu-latest
    steps:
      - name: Checkout Code
        uses: actions/checkout@v4

      - name: Authenticate to AWS via OIDC
        uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: arn:aws:iam::123456789012:role/GithubConsumer-ECR-Pull
          aws-region: us-west-2
          audience: sts.amazonaws.com

      - name: Log in to Private ECR
        id: login-ecr
        uses: aws-actions/amazon-ecr-login@v2

      - name: Execute Security Analyzer Container
        env:
          ECR_REGISTRY: ${{ steps.login-ecr.outputs.registry }}
          IMAGE_TAG: latest
        run: |
          docker run --rm \
            -v "${{ github.workspace }}:/github/workspace" \
            -e GITHUB_STEP_SUMMARY="/github/workspace/summary.md" \
            -e INPUT_MODE="scan" \
            -e INPUT_SCAN_PATH="." \
            -e INPUT_FAIL_ON="ERROR" \
            "${ECR_REGISTRY}/ajp-security-analyzer:${IMAGE_TAG}"

      - name: Append Summary to GitHub Summary
        if: always()
        run: |
          if [[ -f "${{ github.workspace }}/summary.md" ]]; then
            cat "${{ github.workspace }}/summary.md" >> $GITHUB_STEP_SUMMARY
          fi
```

---

## 8. IAM Trust Boundaries, Security Containment & Threat Modeling

```mermaid
flowchart LR
    subgraph GitHub_Trust_Domain ["GitHub Actions OIDC Trust Domain"]
        WorkflowJWT["GitHub OIDC JWT Token<br/>(sub: repo:ajponte/security-analyzer:*)"]
    end

    subgraph AWS_IAM_Boundary ["AWS IAM Security Boundary"]
        OIDCProvider["OIDC Provider: token.actions.githubusercontent.com"]
        PublisherRole["IAM Role: GitHubActions-ECR-Publisher"]
        ConsumerRole["IAM Role: GithubConsumer-ECR-Pull"]
    end

    subgraph ECR_Storage ["AWS Container Registries"]
        PublicRepo[("AWS Public ECR (us-east-1)<br/>public.ecr.aws/<alias>/ajp-security-analyzer")]
        PrivateRepo[("AWS Private ECR (us-west-2)<br/>123456789012.dkr.ecr.us-west-2.amazonaws.com/ajp-security-analyzer")]
    end

    WorkflowJWT -->|sts:AssumeRoleWithWebIdentity| OIDCProvider
    OIDCProvider -->|Evaluates Subject & Audience| PublisherRole
    OIDCProvider -->|Evaluates Enterprise Org Sub| ConsumerRole

    PublisherRole -->|ecr-public:UploadLayerPart, PutImage| PublicRepo
    ConsumerRole -->|ecr:BatchGetImage, GetDownloadUrlForLayer| PrivateRepo
```

### 8.1 AWS IAM OIDC Trust Policy (Publisher)

To eliminate hardcoded long-lived `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` secrets, the publishing role utilizes Web Identity Federation:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "GitHubOIDCAuthentication",
      "Effect": "Allow",
      "Principal": {
        "Federated": "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"
      },
      "Action": "sts:AssumeRoleWithWebIdentity",
      "Condition": {
        "StringEquals": {
          "token.actions.githubusercontent.com:aud": "sts.amazonaws.com"
        },
        "StringLike": {
          "token.actions.githubusercontent.com:sub": "repo:ajponte/security-analyzer:*"
        }
      }
    }
  ]
}
```

### 8.2 IAM Least-Privilege Publishing Policy (Public ECR)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "PublicECRAuth",
      "Effect": "Allow",
      "Action": [
        "ecr-public:GetAuthorizationToken",
        "sts:GetServiceBearerToken"
      ],
      "Resource": "*"
    },
    {
      "Sid": "PublicECRPush",
      "Effect": "Allow",
      "Action": [
        "ecr-public:BatchCheckLayerAvailability",
        "ecr-public:CompleteLayerUpload",
        "ecr-public:InitiateLayerUpload",
        "ecr-public:PutImage",
        "ecr-public:UploadLayerPart"
      ],
      "Resource": "arn:aws:ecr-public::123456789012:repository/ajp-security-analyzer"
    }
  ]
}
```

### 8.3 IAM Least-Privilege Consumer Policy (Private ECR `us-west-2`)

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "PrivateECRAuth",
      "Effect": "Allow",
      "Action": [
        "ecr:GetAuthorizationToken"
      ],
      "Resource": "*"
    },
    {
      "Sid": "PrivateECRPull",
      "Effect": "Allow",
      "Action": [
        "ecr:BatchCheckLayerAvailability",
        "ecr:GetDownloadUrlForLayer",
        "ecr:BatchGetImage"
      ],
      "Resource": "arn:aws:ecr:us-west-2:123456789012:repository/ajp-security-analyzer"
    }
  ]
}
```

### 8.4 Container & Runtime Security Boundaries

1. **Non-Privileged Execution**: The container requires no privileged flags (`--privileged`) or Linux capabilities (`CAP_SYS_ADMIN`).
2. **Directory Traversal Protection**: When running in `analyze` mode, tool calls requesting file scanning must pass `isSafePath` containment checks ([pkg/mcp/tools.go](file:///Users/aponte/personal_workspace/repos/security-analyzer/pkg/mcp/tools.go#L21)), preventing the LLM or malicious prompts from accessing host files outside `/github/workspace`.
3. **Secret Masking & Zero-Logging**: Sensitive API keys (`OPENAI_API_KEY`, etc.) passed through Action inputs are strictly routed as process environment variables and never logged to stdout or embedded in artifact reports.
4. **Read-Only Rootfs Compatibility**: All generated reports (`report.md`, `llm-reports/*.md`) are written exclusively to the mounted workspace directory (`/github/workspace`), allowing container engines to run with read-only root filesystems (`--read-only`) if configured.

---

## 9. Verification, Testing & Operational Runbook

### Local Container Build & Verification
```bash
# 1. Build local container image
docker build -t ajp-security-analyzer:local .

# 2. Test fast SAST scan mode
docker run --rm -v "$(pwd):/github/workspace" ajp-security-analyzer:local scan .

# 3. Test LLM analysis mode (requires API key)
docker run --rm -v "$(pwd):/github/workspace" \
  -e OPENAI_API_KEY="${OPENAI_API_KEY}" \
  ajp-security-analyzer:local analyze .
```

### Troubleshooting & Operational Matrix

| Symptom / Error | Root Cause | Remediation |
| :--- | :--- | :--- |
| `fatal: detected dubious ownership in repository` | Container UID mismatch with host runner mount | Ensure `git config --global --add safe.directory '*'` is present in Dockerfile and executed before git commands. |
| `OIDC error: Could not assume role with web identity` | Missing `permissions: id-token: write` or mismatched `sub` claim in IAM Trust Policy | Verify workflow YAML defines `id-token: write` and ensure IAM trust condition matches `repo:<owner>/<repo>:*`. |
| `Public ECR login error: unauthorized` | AWS Public ECR requested in region other than `us-east-1` | AWS Public ECR auth control plane is strictly located in `us-east-1`. Set `aws-region: us-east-1` in the credentials action. |
| `Private ECR pull denied in us-west-2` | IAM role missing repository pull permissions | Ensure consumer role has `ecr:BatchGetImage` on `arn:aws:ecr:us-west-2:<account>:repository/ajp-security-analyzer`. |
| `Semgrep scan timeout exceeded` | Massive mono-repo or unindexed vendor directories | Supply `.semgrepignore` or increase timeout input via `with: timeout: '15m'`. |
