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

sh(f'CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-X main.version={tag}" -o service-manager .')

if server != "https://github.com":
    print("Gitea detected; skipping release upload")
    sys.exit(0)

api = f"https://api.github.com/repos/{repo}/releases"
headers = {"Authorization": f"Bearer {tok}", "Accept": "application/vnd.github+json"}
data = {"tag_name": tag, "name": f"Release {tag}", "draft": False, "prerelease": False}
r = requests.post(api, headers=headers, json=data)
if r.status_code == 422:
    r = requests.get(f"{api}/tags/{tag}", headers=headers)
release_id = r.json()["id"]

upload = f"{api}/{release_id}/assets?name=service-manager"
with open("service-manager", "rb") as f:
    h = headers | {"Content-Type": "application/octet-stream"}
    requests.post(upload, headers=h, data=f.read())
print("Release created:", tag)
