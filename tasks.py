from invoke import task, Collection
from botocore.client import Config
from dotenv import load_dotenv
from typing import Optional
from pathlib import Path
import shutil
import boto3
import sys
import os
import datetime
from minio import Minio


# Basic project settings, similar to Makefile variables
GO_ENV = {
    "GOBUILD": "go build",
    "GOCLEAN": "go clean",
    "GOTEST": "go test",
    "GOGET": "go get",
    "BINARY_NAME": "go-overlay",
    "BINARY_UNIX": "go-overlay_unix",
    "DOCKER_IMAGE": "go-overlay",
    "DOCKER_TAG": "latest",
}
RELEASE_FILENAME = "go-overlay-linux-amd64"

def _run(c, cmd: str, pty: bool = True, env=None):
    """Helper to run commands."""
    return c.run(cmd, pty=pty, env=env)

def getenv(name: str, default: Optional[str] = None, required: bool = False) -> str:
    """Helper to get environment variables, exiting if required and not found."""
    value = os.getenv(name, default)
    if required and not value:
        print(f"ERROR: Missing required environment variable: {name}")
        sys.exit(1)
    return value or ""

# =============================================================================
# Go Tasks
# =============================================================================

@task
def build(c):
    """Build the binary for the local OS."""
    print(f"Building {GO_ENV['BINARY_NAME']}...")
    _run(c, f"{GO_ENV['GOBUILD']} -o {GO_ENV['BINARY_NAME']} -v .")

@task
def build_linux(c):
    """Build the binary for Linux (for releases and Docker)."""
    print("Building for Linux...")
    env = os.environ.copy()
    env.update({"CGO_ENABLED": "0", "GOOS": "linux"})
    _run(c, f"{GO_ENV['GOBUILD']} -o {GO_ENV['BINARY_UNIX']} -v .", env=env)

@task
def clean(c):
    """Clean build artifacts."""
    print("Cleaning...")
    _run(c, GO_ENV['GOCLEAN'])
    Path(GO_ENV['BINARY_NAME']).unlink(missing_ok=True)
    Path(GO_ENV['BINARY_UNIX']).unlink(missing_ok=True)
    print("Clean complete.")

@task
def test(c):
    """Run tests."""
    print("Running tests...")
    _run(c, f"{GO_ENV['GOTEST']} -v ./...")

@task
def deps(c):
    """Install Go dependencies."""
    print("Installing dependencies...")
    _run(c, f"{GO_ENV['GOGET']} -v ./...")

# =============================================================================
# Installation Tasks
# =============================================================================

@task(pre=[build])
def install(c):
    """Install the binary in the system."""
    print("Installing go-overlay...")
    c.run(f"sudo cp {GO_ENV['BINARY_NAME']} /go-overlay", pty=True)
    c.run(f"sudo chmod +x /go-overlay", pty=True)
    print("✓ go-overlay installed successfully!")
    print("Run 'go-overlay --help' to see commands.")

@task
def uninstall(c):
    """Uninstall the binary from the system."""
    print("Uninstalling go-overlay...")
    c.run("sudo rm -f /go-overlay", pty=True)
    c.run("sudo rm -f /usr/local/bin/entrypoint", pty=True, warn=True)
    c.run("sudo rm -rf /etc/go-overlay", pty=True, warn=True)
    print("✓ go-overlay uninstalled")

# =============================================================================
# Docker Tasks
# =============================================================================

@task(pre=[build_linux])
def docker_build(c):
    """Build the Docker image using the local binary."""
    print("Building Docker image...")
    _run(c, f"docker build -t {GO_ENV['DOCKER_IMAGE']}:{GO_ENV['DOCKER_TAG']} .")

@task
def docker_release(c, version="latest"):
    """Build a release Docker image from GitHub."""
    tag = f"release-{version}" if version != "latest" else "release-latest"
    print(f"Building Docker image from GitHub release ({version})...")
    build_arg = f"--build-arg VERSION={version}" if version != "latest" else ""
    _run(c, f"docker build -f Dockerfile.release {build_arg} -t {GO_ENV['DOCKER_IMAGE']}:{tag} .")

@task(help={'daemon': "Run in daemon mode (background)", 'port': "Port to expose"})
def docker_run(c, daemon=False, port=80):
    """Run the Docker container."""
    container_name = "go-overlay-test"
    # Stop if already running
    c.run(f"docker rm -f {container_name} || true", pty=True, warn=True, hide=True)
    
    run_options = "-d" if daemon else "--rm"
    mode = "in background" if daemon else "in foreground"
    print(f"Running Docker container {mode}...")
    _run(c, f"docker run {run_options} -p {port}:{port} --name {container_name} {GO_ENV['DOCKER_IMAGE']}:{GO_ENV['DOCKER_TAG']}")

@task
def docker_stop(c):
    """Stop and remove the test Docker container."""
    container_name = "go-overlay-test"
    print("Stopping Docker container...")
    c.run(f"docker stop {container_name} || true", pty=True, warn=True)
    c.run(f"docker rm {container_name} || true", pty=True, warn=True)

@task
def docker_logs(c):
    """Show logs from the test Docker container."""
    container_name = "go-overlay-test"
    print("Showing Docker container logs...")
    _run(c, f"docker logs -f {container_name}")


# =============================================================================
# Release & Test Tasks
# =============================================================================

@task(pre=[clean, test, build_linux])
def release(c):
    """Create a release package."""
    print("Creating release package...")
    release_dir = Path("release")
    release_dir.mkdir(parents=True, exist_ok=True)
    shutil.copy(GO_ENV['BINARY_UNIX'], release_dir / RELEASE_FILENAME)
    shutil.copy("services.toml", release_dir)
    if Path("tests/install-enhanced.sh").exists():
        shutil.copy("tests/install-enhanced.sh", release_dir)
    shutil.copy("README.md", release_dir)
    print(f"✓ Release created in ./{release_dir}/")

@task(pre=[release])
def upload(c):
    """Upload the release artifact to Backblaze B2."""
    print("Uploading artifact to Backblaze B2...")

    # Load .env file if present
    load_dotenv(dotenv_path=os.getenv("DOTENV_PATH", ".env"))

    endpoint = getenv("BACKBLAZE_ENDPOINT", "s3.us-east-005.backblazeb2.com")
    access_key = getenv("BACKBLAZE_ACCESS_KEY_ID", required=True)
    secret_key = getenv("BACKBLAZE_SECRET_ACCESS_KEY", required=True)
    bucket_name = getenv("BACKBLAZE_BUCKET", required=True)

    file_path = Path("release") / RELEASE_FILENAME
    if not file_path.is_file():
        print(f"ERROR: File not found: {file_path}")
        sys.exit(1)

    object_name = getenv("OBJECT_NAME", file_path.name)
    expiration_str = getenv("EXPIRATION", "3600")
    try:
        expiration_seconds = int(expiration_str)
    except ValueError:
        print(f"ERROR: Invalid EXPIRATION value: {expiration_str}")
        sys.exit(1)

    # Initialize MinIO client
    client = Minio(
        endpoint,
        access_key=access_key,
        secret_key=secret_key,
        secure=True  # Backblaze uses HTTPS
    )

    try:
        # Check if bucket exists
        found = client.bucket_exists(bucket_name)
        if not found:
            print(f"ERROR: Bucket '{bucket_name}' not found.")
            sys.exit(1)

        # Upload the file
        result = client.fput_object(
            bucket_name, object_name, str(file_path),
        )
        print(f"✅ Upload complete: {result.object_name} (version: {result.version_id}) -> bucket {bucket_name}")

        # Generate presigned URL
        url = client.presigned_get_object(
            bucket_name,
            object_name,
            expires=datetime.timedelta(seconds=expiration_seconds),
        )
        print(f"🔗 Presigned URL (valid for {expiration_seconds // 60} min):\n{url}")
        
        # Friendly URL (if bucket is public)
        try:
            cluster_id = endpoint.split(".")[1].split("-")[-1]  # e.g., "005"
            friendly_domain = f"f{cluster_id}.backblazeb2.com"
            friendly_url = f"https://{friendly_domain}/file/{bucket_name}/{object_name}"
            print(f"🌐 Public URL (if bucket is public):\n{friendly_url}")
        except Exception as parse_err:
            print(f"⚠️ Could not generate public URL automatically: {parse_err}")

    except Exception as e:
        print(f"❌ Upload or URL generation error: {e}")
        sys.exit(1)

@task
def test_cli(c):
    """Run CLI tests using Docker."""
    print("Testing CLI commands...")
    docker_run(c, daemon=True)
    c.run("sleep 5", pty=True) # wait for container to start
    
    container_name = "go-overlay-test"
    print(f"\n--- Testing go-overlay commands on container {container_name} ---")
    commands = {
        "Status": "status",
        "List services": "list",
        "Restart nginx": "restart nginx",
    }
    for desc, cmd in commands.items():
        print(f"\n--> {desc}...")
        c.run(f"docker exec {container_name} go-overlay {cmd}", pty=True, warn=True)
        if "restart" in cmd:
            print("Waiting 3s after restart...")
            c.run("sleep 3", pty=True)
            print("--> Listing services again...")
            c.run(f"docker exec {container_name} go-overlay list", pty=True, warn=True)
            
    print("\n--- CLI tests finished ---")
    docker_stop(c)

# =============================================================================
# Main Namespace Configuration
# =============================================================================
# Create collections for namespacing
go_coll = Collection('go', build, build_linux, clean, test, deps)
docker_coll = Collection('docker', docker_build, docker_release, docker_run, docker_stop, docker_logs)
release_coll = Collection('release', release, upload)

# Main namespace
ns = Collection(
    go_coll,
    docker_coll,
    release_coll,
    install,
    uninstall,
    test_cli,
)
