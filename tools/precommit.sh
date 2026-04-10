#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(realpath "$(dirname "${BASH_SOURCE[0]}")/..")"

# Will be set to false if any of the steps fail
did_checks_pass=true

check_precommit_installed() {
  if ! command -v pre-commit &> /dev/null; then
    echo "pre-commit is not installed or cannot be found in the current PATH."
    echo "Install it with: pip install pre-commit or brew install pre-commit"
    did_checks_pass=false
    return 1
  fi
  return 0
}

install_hooks() {
  echo "Installing pre-commit hooks..."

  # Change to repo root to ensure we're in the right directory
  cd "${REPO_ROOT}"

  # Install pre-commit hooks
  if ! pre-commit install; then
    echo "Failed to install pre-commit hooks"
    did_checks_pass=false
    return 1
  fi

  # Install commit message hook
  if ! pre-commit install --hook-type commit-msg; then
    echo "Failed to install commit message hook"
    did_checks_pass=false
    return 1
  fi

  return 0
}

test_hooks() {
  echo "Testing basic hooks..."

  # Change to repo root
  cd "${REPO_ROOT}"

  # Test a couple of simple hooks to verify setup
  if pre-commit run trailing-whitespace --all-files >/dev/null 2>&1; then
    echo "✓ trailing-whitespace hook verified"
  else
    echo "⚠ trailing-whitespace hook made fixes (this is normal on first run)"
  fi

  if pre-commit run end-of-file-fixer --all-files >/dev/null 2>&1; then
    echo "✓ end-of-file-fixer hook verified"
  else
    echo "⚠ end-of-file-fixer hook made fixes (this is normal on first run)"
  fi
}

print_usage() {
  cat << EOF
Pre-commit hooks setup for Admiral

Usage: $0 [command]

Commands:
  install    Install pre-commit hooks (default)
  test       Test that hooks are working
  help       Show this help message

What happens after installation:
• Hooks will run automatically before each commit
• If a hook fails, the commit will be blocked until you fix the issues
• Use 'git commit --no-verify' to skip hooks if needed (not recommended)
• Run 'pre-commit run --all-files' to check all files manually

Configured hooks include:
• Basic file checks (trailing whitespace, line endings, etc.)
• Go formatting (go fmt, go vet, go imports)
• Go linting (golangci-lint)
• Go tests
• Dockerfile linting
• YAML/JSON formatting
• Conventional commit messages
• Project-specific linting (make server-lint, make web-lint)
EOF
}

main() {
  local command="${1:-install}"

  case "$command" in
    "install")
      echo "Setting up pre-commit hooks for Admiral..."

      if ! check_precommit_installed; then
        return 1
      fi

      if ! install_hooks; then
        return 1
      fi

      test_hooks

      if [ "$did_checks_pass" = true ]; then
        echo ""
        echo "✓ Pre-commit hooks are now installed!"
        echo "Happy coding! 🚀"
      else
        echo ""
        echo "✗ Setup encountered issues. Please check the errors above."
        return 1
      fi
      ;;
    "test")
      echo "Testing pre-commit hooks..."
      if check_precommit_installed; then
        test_hooks
        echo "✓ Hook tests completed"
      fi
      ;;
    "help"|"-h"|"--help")
      print_usage
      ;;
    *)
      echo "Unknown command: $command"
      print_usage
      return 1
      ;;
  esac
}

main "$@"
