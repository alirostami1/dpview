#!/bin/sh

set -eu

PROJECT_NAME=dpview
REPO=alirostami1/dpview
INSTALL_DIR=${DPVIEW_INSTALL_DIR:-${HOME}/.local/bin}
BIN_NAME=dpview

log() {
    printf '%s\n' "$*" >&2
}

fail() {
    log "error: $*"
    exit 1
}

need_cmd() {
    command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

usage() {
    cat <<EOF
Install the latest ${PROJECT_NAME} release from GitHub.

Environment overrides:
  DPVIEW_INSTALL_DIR       Install directory (default: ${INSTALL_DIR})

Examples:
  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | sh
  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/scripts/install.sh | \\
    DPVIEW_INSTALL_DIR=\$HOME/bin sh
EOF
}

if [ "${1-}" = "--help" ] || [ "${1-}" = "-h" ]; then
    usage
    exit 0
fi

need_cmd uname

if command -v curl >/dev/null 2>&1; then
    fetch() {
        curl -fsSL "$1"
    }
elif command -v wget >/dev/null 2>&1; then
    fetch() {
        wget -qO- "$1"
    }
else
    fail "missing required command: curl or wget"
fi

OS=$(uname -s)
ARCH=$(uname -m)

case "${OS}" in
    Linux) OS=linux ;;
    Darwin) OS=darwin ;;
    FreeBSD) OS=freebsd ;;
    *)
        fail "unsupported operating system: ${OS}"
        ;;
esac

case "${ARCH}" in
    x86_64|amd64) ARCH=amd64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    armv7l) ARCH=armv7 ;;
    armv6l) ARCH=armv6 ;;
    i386|i686) ARCH=386 ;;
    *)
        fail "unsupported architecture: ${ARCH}"
        ;;
esac

API_URL="https://api.github.com/repos/${REPO}/releases/latest"

log "Resolving latest ${PROJECT_NAME} release for ${OS}/${ARCH} from ${REPO}"
release_json=$(fetch "${API_URL}") || fail "failed to fetch latest release metadata"

extract_first() {
    printf '%s' "${release_json}" | grep -o "\"$1\":[[:space:]]*\"[^\"]*\"" | head -n 1 | sed 's/^[^"]*"[^"]*":[[:space:]]*"//; s/"$//'
}

VERSION=$(extract_first tag_name)
[ -n "${VERSION}" ] || fail "could not determine latest release tag"
VERSION_NO_V=${VERSION#v}

case "${OS}" in
    linux|darwin) ARCHIVE_EXT=tar.gz ;;
    *)
        fail "no installable release archive for operating system: ${OS}"
        ;;
esac

asset_name="${PROJECT_NAME}_${VERSION_NO_V}_${OS}_${ARCH}.${ARCHIVE_EXT}"
asset_url=$(
    printf '%s' "${release_json}" |
        grep -o '"browser_download_url":[[:space:]]*"[^"]*"' |
        sed 's/^[^"]*"browser_download_url":[[:space:]]*"//; s/"$//' |
        while IFS= read -r url; do
            [ "${url##*/}" = "${asset_name}" ] && printf '%s\n' "${url}"
        done |
        head -n 1
)

[ -n "${asset_url}" ] || fail "no release asset matched ${asset_name}"

asset_name=${asset_url##*/}
tmpdir=$(mktemp -d)
cleanup() {
    rm -rf "${tmpdir}"
}
trap cleanup EXIT INT TERM

download_path="${tmpdir}/${asset_name}"
log "Downloading ${asset_name}"
fetch "${asset_url}" >"${download_path}" || fail "failed to download release asset"

extracted_bin=""
case "${asset_name}" in
    *.tar.gz|*.tgz)
        tar -xzf "${download_path}" -C "${tmpdir}" || fail "failed to extract tar archive"
        ;;
    *.zip)
        need_cmd unzip
        unzip -q "${download_path}" -d "${tmpdir}" || fail "failed to extract zip archive"
        ;;
    *)
        extracted_bin="${download_path}"
        ;;
esac

if [ -z "${extracted_bin}" ]; then
    extracted_bin=$(find "${tmpdir}" -type f -name "${BIN_NAME}" | head -n 1)
fi

[ -n "${extracted_bin}" ] || fail "downloaded asset did not contain a ${BIN_NAME} binary"

mkdir -p "${INSTALL_DIR}" || fail "failed to create install directory: ${INSTALL_DIR}"
install_path="${INSTALL_DIR}/${BIN_NAME}"
install -m 755 "${extracted_bin}" "${install_path}" || fail "failed to install binary"

log "Installed ${BIN_NAME} ${VERSION} to ${install_path}"

case ":${PATH}:" in
    *:"${INSTALL_DIR}":*)
        ;;
    *)
        log "warning: ${INSTALL_DIR} is not in PATH"
        ;;
esac
