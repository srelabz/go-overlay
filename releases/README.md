# Release notes (YAML)

Each GitHub release ships **hand-written notes** instead of an auto-generated
commit list.

## File layout

```
releases/
  v0.1.4.yaml    # notes for tag v0.1.4
  v0.1.5.yaml
  README.md
```

The publisher (`.github/scripts/release.py`) looks for:

```text
releases/<tag>.yaml
releases/<tag>.yml
```

If the file exists it becomes the GitHub release body. If it is missing, the
script falls back to `RELEASE_BODY`, then the annotated tag message, then
conventional-commit subjects since the previous SemVer tag.

> **Note:** `releseases_notes.toml` is deprecated. Prefer `releases/<tag>.yaml`.
> This directory is not the same as `release/` (singular), which holds the
> artifact bundle uploaded by the pipeline.

## Schema

```yaml
# Optional - when present it must match the release tag
tag: v0.1.5

# Heading rendered as "## ..." (default: What's new)
title: Supervisor hardening

# Optional one-paragraph intro
highlights: |
  Short summary of the release for humans.

# Bullet lists (omit any section that is empty)
features:
  - "User-facing feature in plain language"

fixes:
  - "Bug fix description"

changes:
  - "Docs, tooling, refactors worth calling out"

breaking:
  - "Incompatible change (if any)"

# Optional: previous tag for the compare link (auto-detected when omitted)
# previous: v0.1.4
```

Rules:

- Keys are optional; omit empty sections.
- List items are strings. Quote them when they contain `:` or start with a
  special YAML character.
- `highlights` accepts a literal (`|`) or folded (`>`) block.
- The `tag` field is optional; when present it must match the release tag.

## Workflow

1. Implement the release.
2. Add `releases/vX.Y.Z.yaml` describing what shipped.
3. Commit, then either push the tag `vX.Y.Z` or run the **Auto Tag** workflow.
4. CI builds the binary, publishes `go-overlay.sha256`, and uses this YAML as the
   GitHub release body.

Parsing is covered by `.github/scripts/test_release.py`:

```bash
mise exec -- python .github/scripts/test_release.py
```
