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
| **Credentials** | ✅ | Type-based, AES-GCM at rest, secret redaction |
| **Job Templates** | ✅ | Link project + inventory + playbook |
| **Job Execution** | ✅ | Real `ansible-playbook` runs |
| **Job Logs** | ✅ | Full stdout/stderr capture |
| **Job Events** | ✅ | Lifecycle events (created, running, successful, failed) |
| **Transactional Outbox** | ✅ | Same-txn event writes, relayed to NATS JetStream |
| **NATS Dispatch** | ✅ | Durable consumers execute jobs from the bus |
| **Workflow Orchestration** | ✅ | AWX-style DAG with success/failure/always links |
| **Event-driven Workflows** | ✅ | DAG advances from `job.*` bus events |
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

GET    /workflow-templates  # List workflow templates
POST   /workflow-templates  # Create workflow (nodes + links)
POST   /workflow-templates/{id}/launch  # Launch workflow run
GET    /workflow-jobs       # List workflow runs
GET    /workflow-jobs/{id}  # Workflow run detail (DAG status)
```

### UI

React + TypeScript SPA ([shadcn/ui](https://ui.shadcn.com/) + Tailwind CSS 4) with:
- Dashboard with stats
- Organizations, users, and role assignment management
- Projects list with sync buttons
- Job Templates with launch
- Jobs list with status badges
- Job Detail with full Ansible output (terminal-style log view)
- Workflow templates + visual DAG builder
- Credentials management
- Error states and loading indicators

## Quick Start

### Prerequisites

- Go 1.23+
- Node.js 20+
- Docker (PostgreSQL + NATS)
- Python 3.12+ with `uv` (for Ansible)

### 1. Start PostgreSQL and NATS

```bash
docker-compose up -d
# or:
docker run -d --name aiwx-postgres \
  -e POSTGRES_DB=aiwx \
  -e POSTGRES_USER=aiwx \
  -e POSTGRES_PASSWORD=aiwx \
  -p 5432:5432 \
  postgres:16-alpine

docker run -d --name aiwx-nats \
  -p 4222:4222 -p 8222:8222 \
  nats:2.10-alpine -js -m 8222
```

### 2. Run Migrations

```bash
docker exec -i aiwx-postgres psql -U aiwx -d aiwx < migrations/0001_init.up.sql
docker exec -i aiwx-postgres psql -U aiwx -d aiwx < migrations/0002_awx_targeting.up.sql
docker exec -i aiwx-postgres psql -U aiwx -d aiwx < migrations/0003_workflows.up.sql
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
JWT_SECRET="dev-secret-change-in-production-32bytes!!" \
NATS_URL="nats://localhost:4222" \
PORT=8080 \
AIWX_SEED_ADMIN=1 \
SEED_ADMIN_PASSWORD=admin \
./bin/aiwx
```

> `JWT_SECRET` must be at least 32 bytes (set `AIWX_ALLOW_WEAK_JWT=1` for local experiments).

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
┌─────────────┐     ┌──────────────────────────────────────────┐
│   React UI  │────▶│  API (Go modular monolith)               │
└─────────────┘     │  auth · RBAC · workflows · SSE           │
                    └──────┬───────────────────┬───────────────┘
                           │ same txn          │
                    ┌──────▼───────┐    ┌──────▼───────┐
                    │ PostgreSQL   │    │ Outbox Relay │
                    │ (source of   │    └──────┬───────┘
                    │  truth)      │           │
                    └──────▲───────┘    ┌──────▼───────────────┐
                           │            │ NATS JetStream       │
                           │            │ AIWX_JOB_EVENTS      │
                           │            └──────┬───────────────┘
                           │                   │ durable consumers
                    ┌──────┴───────┐    ┌──────▼───────┐
                    │  Reconciler  │    │  Dispatcher  │
                    └──────────────┘    │  (runner)    │
                                        └──────┬───────┘
                                               │
                                        ┌──────▼───────┐
                                        │ansible-play  │
                                        │  book        │
                                        └──────────────┘

Workflow engine consumes job.* events and advances the DAG.
Slow 30s fallback pollers cover NATS outage / lag.
```

- **API**: REST endpoints, JWT auth, RBAC, workflow templates
- **Transactional outbox**: Job create + status transitions write events in the same DB transaction
- **Outbox relay**: Publishes pending events to NATS JetStream with retry
- **Dispatcher**: Durable consumer `aiwx-dispatcher` claims jobs (idempotent) and runs `ansible-playbook`
- **Workflow engine**: Durable consumer `aiwx-workflow` advances DAGs from `job.*` terminal events
- **Fallback pollers**: 30s claim/advance loops if the bus is unavailable
- **Reconciler**: Marks stale running jobs as failed

### Event bus

| Stream | Subjects | Purpose |
|--------|----------|---------|
| `AIWX_JOB_EVENTS` | `aiwx.job.>` | Job lifecycle (`created`, `running`, `successful`, `failed`, …) |
| `AIWX_PROJECT_EVENTS` | `aiwx.project.>` | Project sync started/completed/failed |
| `AIWX_SYSTEM_EVENTS` | `aiwx.system.>` | System / credential access events |

Envelope: `{ id, type, aggregate_type, aggregate_id, organization_id, payload, timestamp, idempotency_key }`

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `JWT_SECRET` | Yes | Secret for signing JWT tokens (≥ 32 bytes) |
| `NATS_URL` | No | NATS URL (default: `nats://localhost:4222`) |
| `PORT` | No | API port (default: 8080) |
| `AIWX_SEED_ADMIN` | No | Set to `1` to seed admin user |
| `SEED_ADMIN_PASSWORD` | No | Password for seeded admin |
| `CRED_ENC_KEY` | No | AES-GCM key for credential encryption (falls back to `JWT_SECRET`) |
| `AIWX_ALLOW_WEAK_JWT` | No | Set to `1` to allow short JWT secrets in dev |
| `AIWX_ALLOW_NO_NATS` | No | Set to `1` to run without NATS (poll-only jobs) |
| `AIWX_OUTBOX_INTERVAL` | No | Outbox relay tick (default: `1s`) |
| `AIWX_OUTBOX_BATCH` | No | Outbox batch size (default: `50`) |
| `AIWX_JOB_TIMEOUT` | No | Max job runtime (default: `30m`) |
| `AIWX_WORKER_POLL_FALLBACK` | No | Fallback job poll interval (default: `30s`) |
| `ANSIBLE_PLAYBOOK` | No | Path to `ansible-playbook` binary |
| `AIWX_ARTIFACTS_DIR` | No | Project artifact root (default: `/tmp/aiwx-artifacts`) |

## Development

```bash
# Run tests
go test ./...

# Lint
golangci-lint run

# Build UI
cd ui && npm run build

# Health
curl localhost:8080/healthz
curl localhost:8080/readyz   # checks PostgreSQL + NATS
```

## Roadmap

- [ ] Execution Environments (container-based execution)
- [ ] K8s operator
- [ ] Vault / external secret managers
- [ ] SSE fan-out from NATS (currently DB-poll streaming)
- [ ] Multi-instance dispatcher capacity / worker pools
- [ ] Credential YAML injectors (UI ready)

## License

MIT
