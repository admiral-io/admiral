#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-"$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")"}"
BUILD_ROOT="${REPO_ROOT}/build"
BUILD_BIN="${BUILD_ROOT}/bin"

NAME=air
RELEASE=v1.64.5
OSX_RELEASE_SUM=543a213aa705ee7dc35c8579bce821df6ed27ab753db5039ed68afd978b42d24
LINUX_RELEASE_SUM=38569c050cc7b2a85755fba7dc4281bf1a7e09c3d6e5db1b41f935564c894140

ARCH=amd64

RELEASE_BINARY="${BUILD_BIN}/${NAME}-${RELEASE}"

ensure_binary() {
  if [[ ! -f "${RELEASE_BINARY}" ]]; then
    echo "info: Downloading ${NAME} ${RELEASE} to build environment"
    mkdir -p "${BUILD_BIN}"

    case "${OSTYPE}" in
      "darwin"*) os_type="darwin"; sum="${OSX_RELEASE_SUM}" ;;
      "linux"*) os_type="linux"; sum="${LINUX_RELEASE_SUM}" ;;
      *) echo "error: Unsupported OS '${OSTYPE}' for ${NAME} install, please install manually" && exit 1 ;;
    esac

    release_archive="/tmp/${NAME}-${RELEASE}"
    URL="https://github.com/cosmtrek/air/releases/download/${RELEASE}/air_${RELEASE:1}_${os_type}_${ARCH}"
    curl -sSL -o "${release_archive}" "${URL}"
    echo "${sum}" "${release_archive}" | sha256sum --check --quiet -

    find "${BUILD_BIN}" -maxdepth 1 -regex '.*/'${NAME}'-[A-Za-z0-9\.]+$' -exec rm {} \;  # cleanup older versions
    mv "${release_archive}" "${RELEASE_BINARY}"
    chmod +x "${RELEASE_BINARY}"
  fi
}

ensure_fd() {
  if [[ "${OSTYPE}" == *"darwin"* ]]; then
    ulimit -n 1024
  fi
}

ensure_binary
ensure_fd

"${RELEASE_BINARY}"
