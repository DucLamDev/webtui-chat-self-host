#!/usr/bin/env sh
set -eu

DOMAIN=""
EMAIL=""
INSTANCE_NAME="VPSTTT Chat"
PORTAL_ORIGIN="https://chat.vpsttt.com"
EXTERNAL_IP=""
REPO_URL=""
INSTALL_DIR=""
SKIP_DNS_CHECK=0
FORCE=0

usage() {
  echo "Usage: $0 --domain chat.example.com --email admin@example.com [--name 'Example Chat'] [--repo-url https://github.com/org/repo.git] [--install-dir /opt/vpsttt-chat] [--portal-origin https://chat.vpsttt.com] [--external-ip 203.0.113.10] [--skip-dns-check] [--force]"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --domain) DOMAIN=${2:-}; shift 2 ;;
    --email) EMAIL=${2:-}; shift 2 ;;
    --name) INSTANCE_NAME=${2:-}; shift 2 ;;
    --repo-url) REPO_URL=${2:-}; shift 2 ;;
    --install-dir) INSTALL_DIR=${2:-}; shift 2 ;;
    --portal-origin) PORTAL_ORIGIN=${2:-}; shift 2 ;;
    --external-ip) EXTERNAL_IP=${2:-}; shift 2 ;;
    --skip-dns-check) SKIP_DNS_CHECK=1; shift ;;
    --force) FORCE=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

if [ -z "$DOMAIN" ] || [ -z "$EMAIL" ]; then
  usage
  exit 1
fi

if [ -z "$INSTALL_DIR" ]; then
  if [ -f "$0" ]; then
    SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
    SOURCE_DIR=$(CDPATH= cd -- "$SCRIPT_DIR/../.." && pwd)
    if [ -f "$SOURCE_DIR/deploy/self-hosted/install.sh" ]; then
      INSTALL_DIR="$SOURCE_DIR"
    fi
  fi
fi
if [ -z "$INSTALL_DIR" ]; then
  INSTALL_DIR="/opt/vpsttt-chat"
fi

if [ "$(id -u)" -ne 0 ]; then
  echo "Run this script as root, for example with sudo." >&2
  exit 1
fi

if [ -r /etc/os-release ]; then
  . /etc/os-release
else
  echo "This bootstrap supports Ubuntu only." >&2
  exit 1
fi

if [ "${ID:-}" != "ubuntu" ]; then
  echo "This bootstrap supports Ubuntu only. Detected: ${ID:-unknown}" >&2
  exit 1
fi

echo "Updating apt package index..."
apt-get update

echo "Installing base packages..."
DEBIAN_FRONTEND=noninteractive apt-get install -y \
  ca-certificates \
  curl \
  git \
  gnupg \
  openssl \
  ufw

if ! command -v docker >/dev/null 2>&1 || ! docker compose version >/dev/null 2>&1; then
  echo "Installing Docker Engine and Docker Compose v2..."
  install -m 0755 -d /etc/apt/keyrings
  if [ ! -f /etc/apt/keyrings/docker.gpg ]; then
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg
  fi

  ARCHITECTURE=$(dpkg --print-architecture)
  CODENAME=${VERSION_CODENAME:-}
  if [ -z "$CODENAME" ]; then
    echo "Unable to detect Ubuntu codename." >&2
    exit 1
  fi

  echo "deb [arch=$ARCHITECTURE signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $CODENAME stable" > /etc/apt/sources.list.d/docker.list
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y \
    docker-ce \
    docker-ce-cli \
    containerd.io \
    docker-buildx-plugin \
    docker-compose-plugin
else
  echo "Docker Engine and Docker Compose v2 are already installed."
fi

echo "Enabling Docker service..."
systemctl enable --now docker

echo "Configuring UFW firewall..."
ufw allow OpenSSH
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 3478/tcp
ufw allow 3478/udp
ufw allow 443/udp
ufw allow 49160:49200/udp
ufw --force enable

if [ ! -f "$INSTALL_DIR/deploy/self-hosted/install.sh" ]; then
  if [ -z "$REPO_URL" ]; then
    echo "$INSTALL_DIR does not contain VPSTTT Chat source." >&2
    echo "Pass --repo-url, or clone the repository there before running bootstrap." >&2
    exit 1
  fi

  if [ -e "$INSTALL_DIR" ] && [ -n "$(find "$INSTALL_DIR" -mindepth 1 -maxdepth 1 2>/dev/null || true)" ]; then
    echo "$INSTALL_DIR exists and is not empty, but install.sh was not found." >&2
    exit 1
  fi

  echo "Cloning VPSTTT Chat source into $INSTALL_DIR..."
  mkdir -p "$(dirname "$INSTALL_DIR")"
  git clone "$REPO_URL" "$INSTALL_DIR"
fi

echo "Starting VPSTTT Chat self-hosted installer..."
cd "$INSTALL_DIR"
set -- \
  --domain "$DOMAIN" \
  --email "$EMAIL" \
  --name "$INSTANCE_NAME" \
  --portal-origin "$PORTAL_ORIGIN"

if [ -n "$EXTERNAL_IP" ]; then
  set -- "$@" --external-ip "$EXTERNAL_IP"
fi
if [ "$SKIP_DNS_CHECK" -eq 1 ]; then
  set -- "$@" --skip-dns-check
fi
if [ "$FORCE" -eq 1 ]; then
  set -- "$@" --force
fi

sh deploy/self-hosted/install.sh "$@"
