#!/usr/bin/env bash
set -euo pipefail

# Will be set to false if any of the steps fail
did_checks_pass=true

# Minimum versions
MIN_GO_VERSION="1.25"

SCRIPT_ROOT="$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")"
REPO_ROOT="${SCRIPT_ROOT}"
# Use alternate root if provided as command line argument.
if [[ -n "${1-}" ]] && [[ "$1" == *"/"* ]]; then
  REPO_ROOT="${1}"
  shift
fi

echo "${REPO_ROOT}"

BUILD_ROOT="${REPO_ROOT}/build"

# param 1 - required version
# param 2 - current version
# returns true or false if the version is ok
is_version_ok() {
  required_version=$1
  current_version=$2
  if [ "$(printf '%s\n' "$required_version" "$current_version" | sort -V | head -n1)" = "$required_version" ]; then
    return 0
  else
    return 1
  fi
}

check_os() {
  # If we're on OSX lets check for brew and coreutils as they are requirements
  if [[ "$OSTYPE" == "darwin"* ]]; then
    # check brew is installed
    if command -v brew --version &> /dev/null; then
      # check if coreutils is installed
      if ! brew ls --versions coreutils > /dev/null; then
        echo "coreutils is not installed, this is a required dependency."
        echo "install by running [brew install coreutils]"
        did_checks_pass=false
      fi
    else
        echo "brew is not installed (visit https://brew.sh/ to install), unable to verify coreutils dependency."
        did_checks_pass=false
    fi
  fi
}

check_go() {
  if ! command -v go -v &> /dev/null; then
    echo "golang is not installed or cannot be found in the current PATH, this is a required dependency."
    did_checks_pass=false
  else
    current_version=$(go version | { read -r _ _ v _; echo "${v#go}"; })
    if ! is_version_ok $MIN_GO_VERSION "$current_version"; then
      echo "golang version must be >= $MIN_GO_VERSION, current version $current_version"
      did_checks_pass=false
    fi
  fi
}

check_bun() {
  # Bun is now managed via tools/bun.sh and no system installation is required
  echo "Bun is managed via tools/bun.sh - skipping system check"
}

main() {
  echo "Running pre-flight checks..."

  # always check OS level requirements
  check_os

  if [ $# -ge 1 ] && [ -n "$1" ]; then
    if [ "$1" == "go" ]; then
      check_go
    elif [ "$1" == "bun" ]; then
      check_bun
    fi
  else
    check_go
    check_bun
  fi

  if [ "$did_checks_pass" = false ] ; then
    printf "\nPlease refer to the development requirements"
    return 1
  fi
  echo "Pre-flight checks satisfied!"
}

main "$@"
