Welkom to aiwx project.

## Dev login (local only)

Set `AIWX_SEED_ADMIN=1` and `SEED_ADMIN_PASSWORD` to seed a platform admin on first start:

```bash
AIWX_SEED_ADMIN=1 SEED_ADMIN_PASSWORD=admin ./bin/aiwx
```

- email: `admin@aiwx.local`
- password: from `SEED_ADMIN_PASSWORD` (required when seeding)

POST `/api/v1/auth/token` with `{"grant_type":"password","username":"admin@aiwx.local","password":"<password>"}`.

Required env vars: `DATABASE_URL`, `JWT_SECRET`. The process exits with code 1 if either is missing.

## Status

**MVP** — implemented and tested:

- ✅ RBAC with org/team scoping (`internal/rbac`)
- ✅ Tenant isolation (SQL org-scoping on all resources)
- ✅ Job execution pipeline (worker + local runner + reconciler)
- ✅ Transactional outbox for job events
- ✅ React UI with error states, job templates, structured events
- ✅ Config validation, gated seed admin

**Deferred (Phase 5):**

- ❌ Credential encryption at rest (plaintext JSONB today)
- ❌ SSE job event streaming
- ❌ Audit log UI
- ❌ K8s operator, Vault sync, EE pod execution
- ❌ CloudNativePG/NATS/Helm, SSO

## Features (long-term vision)

- need to run on postgres in a k8s cluster, cloudnative pg as backend
- backend needs to be lightning fast so make use of rust / tokio async message bus
- use go if rust is not possible for example
- it needs to have the same features as awx
    so: rbac, git projectsync, vault cred sync, joblog, execution environments
- it needs to run ansible playbooks from execution environments (pods)
- it needs to have a nice gui
- it needs to be multi tenant
- for database scheme you can check how awx scheme now is (which is quite good)
- it needs to have a rest api
- it needs to have a k8s operator
- iterate iterate iterate till something useful

FIRST Step is to gather all nescesserry information from these github repositorys and information which we need to create a plan creating a application.
save them in a information library stored here (local only, not in this repo):
~/aiwx_information


Local sibling repos used as reference (not in this repo):
~/awx
~/awx-operator
~/ansible-ui
~/awx-ee