# 🚀 CI/CD Pipeline - go-overlay

This document describes the complete CI/CD pipeline for the go-overlay project, with automated tests, security scans, and automated release process.

## 📋 Overview

The pipeline is divided into two main parts:

1. **CI (Continuous Integration)** - Tests, Security & Build
2. **CD (Continuous Deployment)** - Release & Deploy

All logic is centralized in:
- `tasks.py` - Python tasks with Invoke
- `mise.toml` - Simplified mise commands
- `.github/workflows/` - GitHub Actions orchestration

## 🔄 CI Pipeline

### CI Stages

```
┌──────────┐    ┌──────────┐    ┌──────────┐
│  Tests   │ -> │ Security │ -> │  Build   │
└──────────┘    └──────────┘    └──────────┘
```

#### 1️⃣ Tests
- Install Go dependencies
- Run unit tests
- Validate code

#### 2️⃣ Security
- **gosec**: Static analysis of Go code
- **govulncheck**: Vulnerability check in dependencies

#### 3️⃣ Build
- Compile binary for Linux
- Validate compilation

### Running Locally

#### Using mise (recommended):

```bash
# Full CI pipeline
mise run ci

# Quick pipeline (skip security scans)
mise run ci:quick

# Tests only
mise run ci:test

# Security only
mise run ci:security

# Build only
mise run ci:build
```

#### Using invoke directly:

```bash
# Full CI pipeline
invoke ci.full

# Quick pipeline
invoke ci.quick

# Individual stages
invoke ci.test
invoke ci.security
invoke ci.build
```

### When CI Runs

CI runs automatically on GitHub Actions when:
- ✅ Pull Requests to `main`
- ✅ Pushes to `main`
- ✅ Manual execution (workflow_dispatch)

## 🚢 CD Pipeline

### CD Stages

```
┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
│  Tests   │ -> │ Security │ -> │ Release  │ -> │  Upload  │
└──────────┘    └──────────┘    └──────────┘    └──────────┘
```

#### Release Process

1. **Tag Creation**: Workflow detects `v*` tags or pushes to `main`
2. **Versioning**: Python script auto-increments version
3. **Compilation**: Build binary with embedded version
4. **Packaging**: Creates `release/` directory with:
   - `go-overlay-linux-amd64` - Compiled binary
   - `services.toml` - Example configuration
   - `README.md` - Documentation
5. **GitHub Upload**: Creates release on GitHub with binary
6. **B2 Upload** (optional): Uploads to Backblaze B2 if configured

### Running Locally

#### Using mise:

```bash
# Full release (with tests)
mise run cd

# Release without tests (faster)
mise run cd:skip-tests
```

#### Using invoke:

```bash
# Full release (with tests)
invoke cd.release

# Release without tests
invoke cd.release --skip-tests
```

### When CD Runs

CD runs automatically on GitHub Actions when:
- ✅ Pushing `v*` tags (e.g., `v0.1.2`)
- ✅ Pushes to `main` (creates tag automatically)
- ✅ Manual execution (workflow_dispatch)

## 🛡️ Security Scans

### Tools Used

| Tool | Description | Command |
|------|-------------|---------|
| **gosec** | Static analysis of Go code | `mise run security:gosec` |
| **govulncheck** | Vulnerability checks in dependencies | `mise run security:vuln` |
| **bandit** | Python code analysis (optional) | `invoke security.scan` |
| **gitleaks** | Secret detection in code | `invoke security.scan` |

### Run All Scans

```bash
# Via mise
mise run security:full

# Via invoke
invoke security.go
```

### Reports

Security reports are saved in `dist/security/`:
- `gosec.json` - gosec report
- `govulncheck.json` - Vulnerability report
- `bandit.json` - bandit report (if executed)
- `gitleaks.json` - Secrets report (if executed)

## 📊 Artifacts and Outputs

### CI Pipeline

**Generated artifacts:**
- `security-reports/` - Security reports (retention: 30 days)
- `go-overlay-binary` - Compiled binary (retention: 7 days)

### CD Pipeline

**Generated artifacts:**
- `go-overlay-release/` - Complete release package (retention: 90 days)

**GitHub Releases:**
- Tag created automatically
- Binary attached to release
- Release notes generated

## 🔧 Configuration

### Environment Variables (optional)

For Backblaze B2 upload, configure:

```bash
export BACKBLAZE_ENDPOINT="s3.us-east-005.backblazeb2.com"
export BACKBLAZE_ACCESS_KEY_ID="your-key-id"
export BACKBLAZE_SECRET_ACCESS_KEY="your-secret"
export BACKBLAZE_BUCKET="your-bucket"
```

Or create a `.env` file in the project root.

### GitHub Secrets

Configure in the GitHub repository:

- `RELEASE_TOKEN` (optional) - Token with release permissions
- If not configured, uses default `GITHUB_TOKEN`

## 📝 Quick Commands

### Local Development

```bash
# Install dependencies
mise run dev

# Local build
mise run build

# Clean artifacts
mise run clean

# Install locally
mise run install
```

### Docker

```bash
# Build image
mise run docker:build

# Run container
mise run docker:run

# Stop container
mise run docker:stop

# Test CLI
mise run docker:test
```

## 🎯 Best Practices

1. **Before creating PR**:
   ```bash
   mise run ci
   ```

2. **Before making release**:
   ```bash
   mise run cd
   ```

3. **For rapid development**:
   ```bash
   mise run ci:quick
   ```

4. **To check security**:
   ```bash
   mise run security:full
   ```

## 🔄 Recommended Workflow

### For Features/Bugfixes

```bash
# 1. Create branch
git checkout -b feature/my-feature

# 2. Develop and test
mise run ci:quick

# 3. Before committing
mise run ci

# 4. Push and create PR
git push origin feature/my-feature
```

### For Release

```bash
# 1. Verify everything is ok
mise run ci

# 2. Create tag
git tag v0.1.2
git push origin v0.1.2

# 3. GitHub Actions does the rest automatically!
```

## 📈 Metrics and Monitoring

### Average Execution Time

- **CI Pipeline**: ~3-5 minutes
- **CD Pipeline**: ~5-8 minutes

### Success Rate

Monitor in GitHub Actions:
- CI should pass in 100% of PRs
- CD should pass in 100% of releases

## 🆘 Troubleshooting

### CI is Failing

1. Run locally to replicate:
   ```bash
   mise run ci
   ```

2. Check logs for each stage:
   ```bash
   mise run ci:test      # Tests
   mise run ci:security  # Security
   mise run ci:build     # Build
   ```

### Security Scans Failing

1. Check the report:
   ```bash
   cat dist/security/gosec.json
   cat dist/security/govulncheck.json
   ```

2. Review and fix the issues found

### Release Not Creating

1. Verify tag follows `v*` pattern
2. Check GitHub Token permissions
3. Test locally:
   ```bash
   mise run cd
   ```

## 📚 References

- [Python Tasks (Invoke)](../tasks.py)
- [mise Configuration](../mise.toml)
- [CI Workflow](../.github/workflows/ci.yml)
- [CD Workflow](../.github/workflows/release.yml)
- [Release Script](../.github/scripts/release.py)

---

**Last updated**: 2025-10-11
