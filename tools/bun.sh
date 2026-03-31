#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-"$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")"}"
BUILD_ROOT="${REPO_ROOT}/build"
BUILD_BIN="${BUILD_ROOT}/bin"

NAME=bun
RELEASE=1.3.10
OSX_X64_RELEASE_SHA256=c1d90bf6140f20e572c473065dc6b37a4b036349b5e9e4133779cc642ad94323
OSX_AARCH64_RELEASE_SHA256=82034e87c9d9b4398ea619aee2eed5d2a68c8157e9a6ae2d1052d84d533ccd8d
LINUX_RELEASE_SHA256=f57bc0187e39623de716ba3a389fda5486b2d7be7131a980ba54dc7b733d2e08

ARCH=x64

RELEASE_BINARY="${BUILD_BIN}/${NAME}-${RELEASE}"

main() {
  ensure_binary

  "${RELEASE_BINARY}" "$@"
}

ensure_binary() {
  if [[ ! -f "${RELEASE_BINARY}" ]]; then
    echo "info: Downloading ${NAME} ${RELEASE} to build environment"
    mkdir -p "${BUILD_BIN}"

    case "${OSTYPE}" in
      "darwin"*)
        os_type="darwin"
        if [[ "$(uname -m)" == "arm64" ]]; then
          ARCH="aarch64"
          sum="${OSX_AARCH64_RELEASE_SHA256}"
        else
          sum="${OSX_X64_RELEASE_SHA256}"
        fi
        ;;
      "linux"*)
        os_type="linux"
        sum="${LINUX_RELEASE_SHA256}"
        ;;
      *) echo "error: Unsupported OS '${OSTYPE}' for ${NAME} install, please install manually" && exit 1 ;;
    esac

    release_archive="/tmp/${NAME}-${RELEASE}.zip"
    URL="https://github.com/oven-sh/bun/releases/download/bun-v${RELEASE}/bun-${os_type}-${ARCH}.zip"
    curl -sSL -o "${release_archive}" "${URL}"
    echo "${sum}  ${release_archive}" | sha256sum --check --quiet -

    release_tmp_dir="/tmp/${NAME}-${RELEASE}"
    mkdir -p "${release_tmp_dir}"
    unzip -q "${release_archive}" -d "${release_tmp_dir}"

    find "${BUILD_BIN}" -maxdepth 1 -regex '.*/'${NAME}'-[0-9\.]+$' -exec rm {} \;  # cleanup older versions
    mv "${release_tmp_dir}/bun-${os_type}-${ARCH}/bun" "${RELEASE_BINARY}"
    chmod +x "${RELEASE_BINARY}"

    # Cleanup
    rm -rf "${release_archive}" "${release_tmp_dir}"
  fi
}

main "$@"
