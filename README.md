# AIWX

A multi-tenant automation controller — a modern reimplementation of [AWX](https://github.com/ansible/awx) in Go.

> **Status: Early MVP** — core pipeline works, many features still in progress.

## Screenshots

| | |
|:--:|:--:|
| ![Login](docs/screenshots/login.png) | ![Dashboard](docs/screenshots/dashboard.png) |
| *Login* | *Dashboard* |
| ![Jobs](docs/screenshots/jobs.png) | ![Job detail](docs/screenshots/job-detail.png) |
| *Jobs* | *Job detail with Ansible output* |
| ![Workflows](docs/screenshots/workflows.png) | ![Workflow builder](docs/screenshots/workflow-builder.png) |
| *Workflows* | *Visual workflow builder* |
| ![Projects](docs/screenshots/projects.png) | ![Job templates](docs/screenshots/job-templates.png) |
| *Projects* | *Job templates* |
| ![Organizations](docs/screenshots/organizations.png) | ![Credentials](docs/screenshots/credentials.png) |
| *Organizations & RBAC* | *Credentials* |

## What Works

### Core Features

| Feature | Status | Notes |
|---------|--------|-------|
| **Authentication** | ✅ | JWT tokens, password grant, seeded admin |
| **RBAC** | ✅ | Role-based access with org/team scoping |
| **Multi-tenancy** | ✅ | Org isolation, cross-tenant access blocked |
| **Projects** | ✅ | Git clone, branch tracking, commit SHA |
| **Project Sync** | ✅ | Real `git clone`, stores artifacts |
| **Credentials** | ✅ | Type-based, secret redaction |
| **Job Templates** | ✅ | Link project + inventory + playbook |
| **Job Execution** | ✅ | Real `ansible-playbook` runs |
| **Job Logs** | ✅ | Full stdout/stderr capture |
| **Job Events** | ✅ | Lifecycle events (created, running, successful, failed) |
| **Reconciler** | ✅ | Marks stale running jobs as failed |

### API

All endpoints at `/api/v1`:

```
POST   /auth/token          # Get JWT token
GET    /auth/me             # Current user

GET    /organizations       # List orgs
POST   /organizations       # Create org (admin only)

GET    /projects            # List projects
POST   /projects            # Create project
POST   /projects/{id}/sync  # Trigger git sync

GET    /credentials         # List credentials
POST   /credentials         # Create credential
GET    /credential-types    # List credential types

GET    /inventories         # List inventories
POST   /inventories         # Create inventory

GET    /job-templates       # List templates
POST   /job-templates       # Create template
POST   /job-templates/{id}/launch  # Launch job

GET    /jobs                # List jobs
GET    /jobs/{id}           # Job details
GET    /jobs/{id}/logs      # Job output (stdout/stderr)
GET    /jobs/{id}/events    # Job lifecycle events
POST   /jobs/{id}/cancel    # Cancel job
```

### UI

React + TypeScript SPA ([shadcn/ui](https://ui.shadcn.com/) + Tailwind CSS 4) with:
- Dashboard with stats
- Organizations, users, and role assignment management
- Projects list with sync buttons
- Job Templates with launch
- Jobs list with status badges
- Job Detail with full Ansible output (terminal-style log view)
- Credentials management
- Error states and loading indicators

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 20+
- Docker (for PostgreSQL)
- Python 3.12+ with `uv` (for Ansible)

### 1. Start PostgreSQL

```bash
docker run -d --name aiwx-postgres \
  -e POSTGRES_DB=aiwx \
  -e POSTGRES_USER=aiwx \
  -e POSTGRES_PASSWORD=aiwx \
  -p 5432:5432 \
  postgres:16-alpine
```

### 2. Run Migrations

```bash
docker exec -i aiwx-postgres psql -U aiwx -d aiwx < migrations/0001_init.up.sql
docker exec aiwx-postgres psql -U aiwx -d aiwx -c "CREATE TABLE IF NOT EXISTS job_logs (job_id uuid NOT NULL, sequence bigint NOT NULL, content text NOT NULL, source text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (job_id, sequence));"
```

### 3. Install Ansible

```bash
uv venv .venv
source .venv/bin/activate
uv pip install ansible-core
```

### 4. Build & Run Backend

```bash
go build -o bin/aiwx ./cmd/aiwx

DATABASE_URL="postgres://aiwx:aiwx@localhost:5432/aiwx?sslmode=disable" \
JWT_SECRET="dev-secret-change-in-production" \
PORT=8080 \
AIWX_SEED_ADMIN=1 \
SEED_ADMIN_PASSWORD=admin \
./bin/aiwx
```

### 5. Run UI

```bash
cd ui
npm install
npm run dev
```

Open http://localhost:3000 — login with `admin@aiwx.local` / `admin`

### 6. Create a Test Playbook

```bash
mkdir -p /tmp/my-playbook && cd /tmp/my-playbook
cat > site.yml << 'EOF'
---
- hosts: localhost
  tasks:
    - debug: msg="Hello from AIWX!"
EOF
git init && git add . && git commit -m "Initial"
```

Then in the UI:
1. Create Project → Git URL: `/tmp/my-playbook`, Branch: `master`
2. Click Sync → wait for "success"
3. Create Job Template → select project, playbook: `site.yml`
4. Click Launch → watch real Ansible output!

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│   React UI  │────▶│   API (Go)   │────▶│ PostgreSQL  │
└─────────────┘     └──────┬───────┘     └─────────────┘
                           │
                    ┌──────▼───────┐
                    │   Worker     │
                    │  (Go)        │
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │ansible-play  │
                    │  book        │
                    └─────────────┘
```

- **API**: REST endpoints, JWT auth, RBAC
- **Worker**: Polls for pending jobs, executes `ansible-playbook`
- **Sync Worker**: Git clone + artifact creation
- **Reconciler**: Marks stale jobs as failed

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `JWT_SECRET` | Yes | Secret for signing JWT tokens |
| `PORT` | No | API port (default: 8080) |
| `AIWX_SEED_ADMIN` | No | Set to `1` to seed admin user |
| `SEED_ADMIN_PASSWORD` | No | Password for seeded admin |

## Development

```bash
# Run tests
go test ./...

# Lint
golangci-lint run

# Build UI
cd ui && npm run build
```

## Roadmap

- [ ] Execution Environments (container-based execution)
- [ ] Credential YAML support
- [ ] Project sync as job entry
- [ ] SSE/WebSocket job streaming
- [ ] Workflow orchestration
- [ ] K8s operator
- [ ] Vault integration

## License

MIT
