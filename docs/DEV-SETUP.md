# Admiral Development Environment Setup

This guide walks you through setting up a local development environment for the `admiral` project. Whether you're contributing code, testing features, or exploring the app, these steps will get you up and running.

## Prerequisites

Before you begin, ensure you have the following installed:

- **Docker**: Required for running dependencies (e.g., databases, services) via Docker Compose.
  - Install: [Docker Desktop](https://www.docker.com/products/docker-desktop/) (includes Docker Compose).
  - Verify: `docker --version` and `docker compose version`.
- **Go**: Needed to build and run the backend.
  - Install: [Go Installation Guide](https://go.dev/doc/install).
  - Recommended version: 1.24 or later (check `go.mod` in the repo for specifics).
  - Verify: `go version`.

## Getting Started

### 1. Set Up Dependencies with Docker Compose

- The project uses Docker Compose to provision external services (e.g., database, idp, object store, and mail sink).
- Run the following command to start these services in the background:

```bash
docker compose -f deploy/docker-compose/docker-compose.yaml up -d
```

- Verify services are running:

```bash
docker ps
```

You should see output similar to the following:

```bash
CONTAINER ID   IMAGE                              COMMAND                  CREATED          STATUS                    PORTS                                                      NAMES
7893d5f91799   quay.io/keycloak/keycloak:24.0     "/opt/keycloak/bin/k…"   38 seconds ago   Up 31 seconds (healthy)   8080/tcp, 8443/tcp, 9000/tcp, 0.0.0.0:9090->9090/tcp       keycloak
4e4a11a7906f   admiral-postgres                   "docker-entrypoint.s…"   38 seconds ago   Up 37 seconds (healthy)   0.0.0.0:5432->5432/tcp                                     postgresql
2e31a6b1ddea   minio/minio:latest                 "/usr/bin/docker-ent…"   38 seconds ago   Up 37 seconds (healthy)   0.0.0.0:9000-9001->9000-9001/tcp                           minio
9a51bbfb9d51   axllent/mailpit:latest             "/mailpit"               38 seconds ago   Up 37 seconds (healthy)   0.0.0.0:1025->1025/tcp, 0.0.0.0:8025->8025/tcp, 1110/tcp   mailpit
```

- Each service (Keycloak, PostgreSQL, Minio, Mailpit) should have a STATUS of Up and (healthy).
- If any service is not running or healthy, check the logs with

```bash
docker logs <container_name>
```

Replace `<container_name>` with the name of the service (e.g., `keycloak`, `postgresql`, `minio`, `mailpit`).

### 2. Start the Development Environment

- Use the make dev target to run both the frontend and backend simultaneously:

```bash
make dev
```

_This launches the full stack in development mode with hot reloading (if configured)._

## Docker Compose Components

The `docker-compose.yaml` file in `deploy/docker-compose/` sets up the following services:

### PostgreSQL Database

- **Purpose:** Persistent storage for the application.
- **Port:** 5432
- **Database Name:** admiral
- **Credentials:**
  - Username: `admiral`
  - Password: `shipitnow`

_Notes: The schema is applied via migrations in Step 2. Connect manually with `psql -h localhost -U admiral -d admiral` if needed (password: `shipitnow`)._

### Keycloak (Local Identity Provider)

- **Purpose:** Provides authentication and user management via an OpenID Connect-compatible IdP.
- **Port:** 9090
- **Admin Console:** http://localhost:9090/admin/master/console/
- **Admin Credentials:**
  - Username: `admiral`
  - Password: `shipitnow`
- **Realm:** A realm named `admiral` is pre-configured.
- **Users in `admiral` Realm:**
  - **Admin User:**
    - Username: `admin`
    - Password: `admin`
    - Role: Administrator privileges.
  - **Demo User:**
    - Username: `demo`
    - Password: `demo`
    - Role: Typical user for testing.

_Notes: Use the admin console to manage users, roles, or clients as needed._

### Minio (Object Storage)
- **Purpose:** Provides S3-compatible object storage for application assets and artifacts.
- **Port:**
  - 9000 (API access)
  - 9001 (Web console)
- **Credentials:**
  - Username: `admiral`
  - Password: `shipitnow`

_Notes: Buckets are automatically created during container initialization. Access the MinIO web console at http://localhost:9001.

### Mailpit (Email Sink)

- **Purpose:** Captures and displays outgoing emails in development, preventing real emails from being sent.
- **Web Interface:** http://localhost:8025/
- **Port:** 8025 (web UI), 1025 (SMTP)

_Notes: Check the UI to view emails triggered by the app (e.g., registration, password resets)._

## Running Components Separately (Optional)

If you prefer to work on the backend or frontend independently:

- **Backend Only**

```bash
make server-dev
```

_Runs the Go server, typically on localhost:8080 (check config.yaml for port)._

- **Frontend Only**

```bash
make web-dev
```

_Runs the web app, often on localhost:8888 (confirm in frontend config or Makefile)._

## Useful Commands

- **List All Make Targets**

```bash
make help
```

_Displays available make commands and their purposes._

- **Stop Docker Services**

```bash
docker compose -f deploy/docker-compose/docker-compose.yaml down -v
```

## Troubleshooting

- Docker Errors: Ensure Docker is running and you have permissions (e.g., add your user to the docker group on Linux).
- Go Command Fails: Verify Go modules are downloaded (go mod download).
- Port Conflicts: Check if ports (e.g., 9090, 5432, 8025) are in use (lsof -i :9090) and adjust configs if needed.

## Pre-commit Hooks Setup

Admiral uses [pre-commit hooks](https://pre-commit.com/) to ensure code quality and consistency. These hooks automatically run checks before each commit to catch issues early.

### Quick Setup

```bash
# Install pre-commit (if not already installed)
pip install pre-commit
# or
brew install pre-commit

# Setup hooks for this project
./tools/precommit.sh
```

### What's Included

- **Code formatting**: Go fmt, imports, YAML/JSON formatting
- **Linting**: golangci-lint, Dockerfile linting, project-specific linters
- **Testing**: Go unit tests  
- **Security**: Private key detection, basic security checks
- **Commit standards**: Conventional commit message formatting

### Manual Usage

```bash
# Run all hooks on all files
pre-commit run --all-files

# Run specific hook
pre-commit run go-fmt --all-files

# Skip hooks for a commit (not recommended)
git commit --no-verify
```

### Hooks Configuration

The hooks are configured in `.pre-commit-config.yaml` and include:

- **Basic checks**: Trailing whitespace, line endings, file format validation
- **Go tools**: go fmt, go vet, go imports, golangci-lint, go mod tidy
- **Tests**: Automated Go unit test execution
- **Project integration**: Uses existing `make server-lint` and `make web-lint` commands
- **Commit standards**: Enforces conventional commit message format

## Next Steps

- Explore the codebase.
- Check README.md or other docs in the repo for app-specific details.
- Submit issues or PRs to https://github.com/mberwanger/admiral!
