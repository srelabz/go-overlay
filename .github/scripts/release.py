#!/usr/bin/env python3
"""Script to automatically create releases on GitHub."""

from typing import Any, Optional
from pathlib import Path
import subprocess
import requests
import hashlib
import sys
import re
import os


REPO_ROOT = Path(__file__).resolve().parents[2]
NOTES_DIR = "releases"
NOTES_YAML_KEYS = ("features", "fixes", "changes", "breaking")
NOTES_YAML_HEADINGS = {
    "features": "### Features",
    "fixes": "### Fixes",
    "changes": "### Changes",
    "breaking": "### Breaking changes",
}
TAG_PATTERN = re.compile(
    r"^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)"
    r"(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$"
)
CHANGELOG_SECTIONS = (
    ("feat", "### Features"),
    ("fix", "### Fixes"),
    ("", "### Other changes"),
)
CONVENTIONAL_PREFIX = re.compile(
    r"^(?P<type>[a-z]+)(?:\([^)]*\))?!?:\s*(?P<subject>.+)$"
)


def git_output(*args: str) -> str:
    """Runs a git command and returns its stdout."""
    result = subprocess.run(
        ["git", *args], capture_output=True, text=True, check=True, timeout=60
    )
    return result.stdout


def semver_tags() -> list:
    """Returns every SemVer tag, newest first."""
    return [
        line.strip()
        for line in git_output("tag", "--sort=-v:refname").splitlines()
        if TAG_PATTERN.fullmatch(line.strip())
    ]


def previous_semver_tag(tag: str) -> Optional[str]:
    """Returns the SemVer tag immediately before `tag`."""
    tags = semver_tags()
    if tag in tags:
        older = tags[tags.index(tag) + 1:]
        return older[0] if older else None
    return tags[0] if tags else None


def changelog_commits(tag: str) -> tuple:
    """Returns the previous SemVer tag and the commit subjects since it."""
    tags = semver_tags()
    if tag in tags:
        end = tag
        older = tags[tags.index(tag) + 1:]
        previous = older[0] if older else None
    else:
        end = "HEAD"
        previous = tags[0] if tags else None

    log_range = f"{previous}..{end}" if previous else end
    subjects = [
        line.strip()
        for line in git_output(
            "log", "--no-merges", "--pretty=format:%s", log_range
        ).splitlines()
        if line.strip()
    ]
    return previous, subjects


def notes_path_for_tag(tag: str, root: Optional[Path] = None) -> Optional[Path]:
    """Returns releases/<tag>.yaml or .yml when it exists."""
    base = root if root is not None else REPO_ROOT
    for name in (f"{tag}.yaml", f"{tag}.yml"):
        path = base / NOTES_DIR / name
        if path.is_file():
            return path
    return None


def _unquote_yaml_scalar(value: str) -> str:
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in "\"'":
        return value[1:-1]
    return value


def parse_release_notes_yaml(text: str) -> dict:
    """Parses the restricted release-notes YAML dialect using only the stdlib.

    Supported:
      key: scalar
      key: |
        multiline block
      key:
        - list item
      # comments and blank lines
    """
    result: dict = {}
    lines = text.splitlines()
    i = 0
    n = len(lines)

    while i < n:
        raw = lines[i]
        stripped = raw.strip()
        if not stripped or stripped.startswith("#"):
            i += 1
            continue

        key_match = re.match(r"^([A-Za-z_][\w]*)\s*:\s*(.*)$", raw)
        if not key_match:
            i += 1
            continue

        key = key_match.group(1)
        val = key_match.group(2).rstrip()
        i += 1

        if val in ("|", ">"):
            block: list = []
            while i < n:
                nxt = lines[i]
                if not nxt.strip():
                    block.append("")
                    i += 1
                    continue
                if not (nxt.startswith(" ") or nxt.startswith("\t")):
                    break
                block.append(nxt[2:] if nxt.startswith("  ") else nxt.lstrip())
                i += 1
            while block and block[-1] == "":
                block.pop()
            if val == "|":
                result[key] = "\n".join(block)
            else:
                result[key] = " ".join(p.strip() for p in block if p.strip())
            continue

        if val != "":
            result[key] = _unquote_yaml_scalar(val)
            continue

        items: list = []
        while i < n:
            nxt = lines[i]
            if not nxt.strip() or nxt.strip().startswith("#"):
                i += 1
                continue
            item_match = re.match(r"^\s*-\s+(.*)$", nxt)
            if not item_match:
                break
            items.append(_unquote_yaml_scalar(item_match.group(1)))
            i += 1
        result[key] = items

    return result


def load_release_notes_file(path: Path) -> dict:
    """Loads release notes from a YAML file, preferring PyYAML when available."""
    text = path.read_text(encoding="utf-8")
    try:
        import yaml

        data = yaml.safe_load(text)
        if isinstance(data, dict):
            return data
    except Exception:
        pass
    return parse_release_notes_yaml(text)


def build_release_body_from_notes(
    data: dict, previous: Optional[str], tag: str, repository: str
) -> str:
    """Builds the Markdown release body from a releases/<tag>.yaml document."""
    if data.get("tag") not in (None, "", tag):
        raise ValueError(
            f"release notes tag {data.get('tag')!r} does not match release tag {tag!r}"
        )

    title = str(data.get("title") or "What's new").strip() or "What's new"
    lines = [f"## {title}"]

    highlights = data.get("highlights")
    if isinstance(highlights, str) and highlights.strip():
        lines.append("")
        lines.append(highlights.strip())

    for key in NOTES_YAML_KEYS:
        items = data.get(key)
        if not items:
            continue
        if isinstance(items, str):
            items = [items]
        if not isinstance(items, list):
            continue
        cleaned = [str(item).strip() for item in items if str(item).strip()]
        if not cleaned:
            continue
        lines.append("")
        lines.append(NOTES_YAML_HEADINGS[key])
        lines.extend(f"- {item}" for item in cleaned)

    declared_previous = data.get("previous")
    if isinstance(declared_previous, str) and declared_previous.strip():
        previous = declared_previous.strip()

    if previous:
        lines.append("")
        lines.append(
            f"**Full changelog**: "
            f"https://github.com/{repository}/compare/{previous}...{tag}"
        )

    body = "\n".join(lines).rstrip() + "\n"
    if body.strip() == f"## {title}":
        raise ValueError(f"release notes file for {tag} has no content sections")
    return body


def build_release_body(
    subjects: list, previous: Optional[str], tag: str, repository: str
) -> Optional[str]:
    """Builds the Markdown release body from conventional commit subjects."""
    if not subjects:
        return None

    grouped: dict = {prefix: [] for prefix, _ in CHANGELOG_SECTIONS}
    for subject in subjects:
        matched = CONVENTIONAL_PREFIX.match(subject)
        commit_type = matched.group("type") if matched else ""
        text = matched.group("subject") if matched else subject
        bucket = commit_type if commit_type in dict(CHANGELOG_SECTIONS) else ""
        grouped[bucket].append(text)

    lines = ["## What's new"]
    for prefix, heading in CHANGELOG_SECTIONS:
        if not grouped[prefix]:
            continue
        lines.append("")
        lines.append(heading)
        lines.extend(f"- {text}" for text in grouped[prefix])

    if previous:
        lines.append("")
        lines.append(
            f"**Full changelog**: "
            f"https://github.com/{repository}/compare/{previous}...{tag}"
        )
    return "\n".join(lines) + "\n"


def release_notes_from_file(
    tag: str, repository: str, root: Optional[Path] = None
) -> Optional[str]:
    """Builds the release body from releases/<tag>.yaml when the file exists."""
    notes_file = notes_path_for_tag(tag, root=root)
    if notes_file is None:
        return None

    try:
        data = load_release_notes_file(notes_file)
        try:
            previous = previous_semver_tag(tag)
        except (OSError, subprocess.SubprocessError):
            previous = None
        body = build_release_body_from_notes(data, previous, tag, repository)
    except (OSError, ValueError, TypeError) as error:
        print(f"WARNING: could not load release notes from {notes_file}: {error}")
        return None

    try:
        display = notes_file.relative_to(root or REPO_ROOT)
    except ValueError:
        display = notes_file
    print(f"Using release notes from {display}")
    return body


def release_notes_from_commits(tag: str, repository: str) -> Optional[str]:
    """Builds the release body from conventional commits since the previous tag."""
    try:
        previous, subjects = changelog_commits(tag)
    except (OSError, subprocess.SubprocessError) as error:
        print(f"WARNING: could not build changelog from commits: {error}")
        return None
    return build_release_body(subjects, previous, tag, repository)


def release_notes(
    tag: str, repository: str, root: Optional[Path] = None
) -> Optional[str]:
    """Builds release notes: prefers releases/<tag>.yaml, else commit subjects."""
    body = release_notes_from_file(tag, repository, root=root)
    if body:
        return body
    return release_notes_from_commits(tag, repository)



class GitHubReleaser:
    """Manages GitHub release creation."""

    def __init__(self):
        self.server = os.getenv("GITHUB_SERVER_URL", "")
        self.repo = os.getenv("GITHUB_REPOSITORY", "")
        self.ref_type = os.getenv("GITHUB_REF_TYPE", "")
        self.ref = os.getenv("GITHUB_REF", "")
        self.token = os.getenv("GITHUB_TOKEN", "")
        self.binary_name = "go-overlay"
        self.update_latest = os.getenv(
            "UPDATE_LATEST_RELEASE", ""
        ).lower() in ("1", "true", "yes")

    def run_command(
        self,cmd: str,capture_output: bool = False,
    ) -> Optional[str]:
        """Executes a shell command."""
        try:
            if capture_output:
                result = subprocess.check_output(cmd, shell=True, text=True)
                return result.strip()
            subprocess.check_call(cmd, shell=True)
            return None
        except subprocess.CalledProcessError as e:
            print(f"ERROR: Command failed: {cmd}")
            raise e

    def print_debug_info(self):
        """Prints debug information."""
        print(f"DEBUG: Server: {self.server}")
        print(f"DEBUG: Repo: {self.repo}")
        print(f"DEBUG: Ref type: {self.ref_type}")
        print(f"DEBUG: Ref: {self.ref}")

    def get_next_version(self) -> str:
        """Calculates the next version based on existing tags."""
        self.run_command("git fetch --prune --tags")
        last_tag = self.run_command(
            "git tag --sort=-v:refname | head -n1", capture_output=True
        )

        if not last_tag:
            last_tag = "v0.0.0"

        print(f"Last tag: {last_tag}")

        version_str = last_tag.lstrip("v")
        major, minor, patch = map(int, version_str.split("."))

        new_tag = f"v{major}.{minor}.{patch + 1}"
        print(f"Creating new tag: {new_tag}")

        return new_tag

    def get_latest_tag(self) -> str:
        """Return the latest tag (highest semver) available in the repo."""
        self.run_command("git fetch --prune --tags")
        last_tag = self.run_command(
            "git tag --sort=-v:refname | head -n1", capture_output=True
        )
        if not last_tag:
            print("ERROR: No tags found; cannot update latest release.")
            sys.exit(1)
        print(f"Latest tag: {last_tag}")
        return last_tag

    def push_tag(self, tag: str):
        """Creates and pushes a new tag to the repository."""
        self.run_command(f"git tag {tag}")

        if self.server == "https://github.com":
            push_url = (
                f"https://x-access-token:{self.token}@github.com/{self.repo}.git"
            )
            self.run_command(f"git push {push_url} {tag}")
        else:
            self.run_command(f"git push origin {tag}")

        print(f"Tag {tag} pushed successfully")

    def handle_branch_push(self) -> str:
        """Handles a branch push by creating a new tag."""
        print("Branch push detected, creating new tag...")
        tag = self.get_next_version()
        self.push_tag(tag)
        print(f"Tag {tag} pushed, exiting to let tag trigger handle the release")
        sys.exit(0)

    def extract_tag_from_ref(self) -> str:
        """Extracts the tag name from the ref."""
        return self.ref.rsplit("/", 1)[-1]

    def build_binary(self, tag: str):
        """Builds the Go binary with the specified version."""
        print(f"Building binary for tag: {tag}")

        build_cmd = (
            f"CGO_ENABLED=0 GOOS=linux go build -a -trimpath "
            f'-ldflags="-s -w -X main.version={tag}" -o {self.binary_name} ./cmd/go-overlay'
        )
        self.run_command(build_cmd)

        binary_path = Path(self.binary_name)
        if not binary_path.exists():
            print(f"ERROR: Binary {self.binary_name} not found after build")
            sys.exit(1)

        file_size = binary_path.stat().st_size
        checksum = self.write_checksum(binary_path)
        print(f"Binary built successfully: {file_size:,} bytes")
        print(f"SHA256: {checksum}")

        return file_size

    def checksum_name(self) -> str:
        """Name of the checksum asset published alongside the binary."""
        return f"{self.binary_name}.sha256"

    def write_checksum(self, binary_path: Path) -> str:
        """Writes the sha256 checksum file next to the binary."""
        digest = hashlib.sha256(binary_path.read_bytes()).hexdigest()
        Path(self.checksum_name()).write_text(
            f"{digest}  {self.binary_name}\n", encoding="utf-8"
        )
        return digest

    def update_version_file(self, tag: str):
        """Updates the VERSION file to match the release tag and pushes to main."""
        version_str = tag.lstrip("v")
        print(f"Updating VERSION file to: {version_str}")

        self.run_command("git config user.name 'github-actions'", capture_output=False)
        self.run_command(
            "git config user.email 'github-actions@github.com'",
            capture_output=False,
        )

        self.run_command("git fetch origin main", capture_output=False)
        self.run_command("git checkout -B main origin/main", capture_output=False)

        Path("VERSION").write_text(version_str + "\n", encoding="utf-8")

        self.run_command("git add VERSION", capture_output=False)
        try:
            self.run_command(
                f"git commit -m 'chore(release): set VERSION to {version_str}'",
                capture_output=False,
            )
        except subprocess.CalledProcessError:
            print("No changes to VERSION; skipping commit")
            return

        if self.server == "https://github.com":
            push_url = (
                f"https://x-access-token:{self.token}@github.com/{self.repo}.git"
            )
            self.run_command("git push %s HEAD:main" % push_url, capture_output=False)
        else:
            self.run_command("git push origin HEAD:main", capture_output=False)

    def create_or_get_release(self, tag: str) -> dict:
        """Creates or retrieves an existing release on GitHub."""
        api_url = f"https://api.github.com/repos/{self.repo}/releases"
        headers = {
            "Authorization": f"Bearer {self.token}",
            "Accept": "application/vnd.github+json",
        }

        body = release_notes_from_file(tag, self.repo) or ""

        if not body:
            body = os.getenv("RELEASE_BODY", "").strip()

        if not body:
            try:
                body = (
                    self.run_command(
                        f"git tag -l --format='%(contents)' {tag}",
                        capture_output=True,
                    )
                    or ""
                )
            except subprocess.CalledProcessError:
                body = ""

        if not body:
            body = release_notes_from_commits(tag, self.repo) or ""

        if not body:
            print(
                f"WARNING: no release notes found for {tag}; "
                f"add {NOTES_DIR}/{tag}.yaml to control the release body"
            )

        release_data = {
            "tag_name": tag,
            "name": tag,
            "body": (body or f"Automated release for {tag}"),
            "draft": False,
            "prerelease": False,
        }

        print(f"Creating release for tag: {tag}")
        response = requests.post(api_url, headers=headers, json=release_data)

        if response.status_code == 422:
            print("Release already exists, fetching existing release...")
            response = requests.get(f"{api_url}/tags/{tag}", headers=headers)
        elif response.status_code != 201:
            print(
                f"ERROR: Failed to create release. "
                f"Status: {response.status_code}, Response: {response.text}"
            )
            sys.exit(1)

        if response.status_code not in (200, 201):
            print(
                f"ERROR: Failed to get release info. "
                f"Status: {response.status_code}, Response: {response.text}"
            )
            sys.exit(1)

        return response.json()

    def delete_existing_asset(self, release_data: dict, name: str):
        """Deletes an asset with the same name from the release, if present."""
        assets_api = release_data.get("assets_url")
        if not assets_api:
            return

        headers = {
            "Authorization": f"Bearer {self.token}",
            "Accept": "application/vnd.github+json",
        }

        try:
            resp = requests.get(assets_api, headers=headers, timeout=30)
            if resp.status_code != 200:
                print(f"WARNING: Could not list assets (status {resp.status_code})")
                return
            for asset in resp.json():
                if asset.get("name") == name:
                    asset_id = asset.get("id")
                    del_url = (
                        f"https://api.github.com/repos/{self.repo}"
                        f"/releases/assets/{asset_id}"
                    )
                    print(f"Deleting existing asset '{name}' (id={asset_id})")
                    requests.delete(del_url, headers=headers, timeout=30)
        except requests.RequestException as e:
            print(f"WARNING: Could not delete existing asset: {e}")

    def upload_asset(self, release_data: dict, file_path: Path, content_type: str):
        """Uploads a single asset to the release."""
        name = file_path.name
        upload_url = release_data["upload_url"].replace(
            "{?name,label}", f"?name={name}"
        )

        self.delete_existing_asset(release_data, name)

        headers = {
            "Authorization": f"Bearer {self.token}",
            "Content-Type": content_type,
            "Accept": "application/vnd.github+json",
        }

        print(f"Uploading {name} ({file_path.stat().st_size:,} bytes)...")
        response = requests.post(
            upload_url, headers=headers, data=file_path.read_bytes(), timeout=300
        )

        if response.status_code != 201:
            print(
                f"ERROR: Failed to upload {name}. "
                f"Status: {response.status_code}\nResponse: {response.text}"
            )
            sys.exit(1)

        print(f"✓ {name} uploaded successfully!")
        print(f"Asset URL: {response.json()['browser_download_url']}")

    def upload_binary(self, release_data: dict, file_size: int):
        """Uploads the binary and its checksum to the release."""
        print(f"Release ID: {release_data['id']} ({file_size:,} bytes)")
        self.upload_asset(
            release_data, Path(self.binary_name), "application/octet-stream"
        )
        self.upload_asset(
            release_data, Path(self.checksum_name()), "text/plain"
        )

    def run(self):
        """Runs the complete release process."""
        self.print_debug_info()

        if self.ref_type == "branch":
            if self.update_latest:
                tag = self.get_latest_tag()
                print(f"Branch push: updating existing release for {tag}")
            else:
                print("Branch push detected; UPDATE_LATEST_RELEASE is not set. Skipping release.")
                sys.exit(0)
        elif self.ref_type == "tag":
            tag = self.extract_tag_from_ref()
            print(f"Tag push detected: {tag}")
            try:
                self.update_version_file(tag)
            except Exception as e:
                print(f"WARNING: Could not update VERSION on main: {e}")
        else:
            print(f"Unknown ref type: {self.ref_type}")
            sys.exit(1)

        file_size = self.build_binary(tag)

        if self.server != "https://github.com":
            print("Gitea detected; skipping release upload")
            sys.exit(0)

        print("Creating or updating GitHub release...")
        release_data = self.create_or_get_release(tag)
        self.upload_binary(release_data, file_size)

        print(f"Release created successfully: {tag}")


def main():
    """Entry point of the script."""
    releaser = GitHubReleaser()
    releaser.run()


if __name__ == "__main__":
    main()
