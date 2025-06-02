#!/usr/bin/env python3
import os, subprocess, sys, requests, shlex, json, tempfile, pathlib

def sh(cmd, output=False):
    if output:
        return subprocess.check_output(cmd, shell=True).decode().strip()
    else:
        subprocess.check_call(cmd, shell=True)

server = os.getenv("GITHUB_SERVER_URL", "")
repo   = os.getenv("GITHUB_REPOSITORY", "")
ref_tp = os.getenv("GITHUB_REF_TYPE", "")
ref    = os.getenv("GITHUB_REF", "")
tok    = os.getenv("GITHUB_TOKEN", "")
tag    = ""

if ref_tp == "branch":
    sh("git fetch --prune --tags")
    last = sh("git tag --sort=-v:refname | head -n1", output=True) or "v0.0.0"
    a, b, c = map(int, last.lstrip("v").split("."))
    tag = f"v{a}.{b}.{c+1}"
    sh(f"git tag {tag}")
    if server == "https://github.com":
        sh(f"git push https://x-access-token:{tok}@github.com/{repo}.git {tag}")
    else:
        sh("git push origin " + tag)
    sys.exit(0)
else:
    tag = ref.rsplit("/", 1)[-1]

print(f"Building binary for tag: {tag}")
sh(f'CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-X main.version={tag}" -o service-manager .')

if not os.path.exists("service-manager"):
    print("ERROR: Binary service-manager not found after build")
    sys.exit(1)

file_size = os.path.getsize("service-manager")
print(f"Binary built successfully: {file_size} bytes")

if server != "https://github.com":
    print("Gitea detected; skipping release upload")
    sys.exit(0)

api = f"https://api.github.com/repos/{repo}/releases"
headers = {"Authorization": f"Bearer {tok}", "Accept": "application/vnd.github+json"}
data = {"tag_name": tag, "name": f"Release {tag}", "draft": False, "prerelease": False}

print(f"Creating release for tag: {tag}")
r = requests.post(api, headers=headers, json=data)

if r.status_code == 422:
    print("Release already exists, fetching existing release...")
    r = requests.get(f"{api}/tags/{tag}", headers=headers)
elif r.status_code != 201:
    print(f"ERROR: Failed to create release. Status: {r.status_code}, Response: {r.text}")
    sys.exit(1)

if r.status_code not in [200, 201]:
    print(f"ERROR: Failed to get release info. Status: {r.status_code}, Response: {r.text}")
    sys.exit(1)

release_data = r.json()
release_id = release_data["id"]
print(f"Release ID: {release_id}")

upload_url = f"{api}/{release_id}/assets?name=service-manager"
print(f"Uploading binary to: {upload_url}")

with open("service-manager", "rb") as f:
    upload_headers = headers.copy()
    upload_headers["Content-Type"] = "application/octet-stream"
    upload_response = requests.post(upload_url, headers=upload_headers, data=f.read())

    if upload_response.status_code == 201:
        print("✓ Binary uploaded successfully!")
        asset_info = upload_response.json()
        print(f"Asset URL: {asset_info['browser_download_url']}")
    else:
        print(f"ERROR: Failed to upload binary. Status: {upload_response.status_code}")
        print(f"Response: {upload_response.text}")
        sys.exit(1)

print("Release created:", tag)
