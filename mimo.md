Welkom to aiwx project.

## Dev login (local only)

On first start the API seeds a platform admin if the users table is empty:

- email: `admin@aiwx.local`
- password: `admin` (override with `SEED_ADMIN_PASSWORD`)

POST `/api/v1/auth/token` with `{"grant_type":"password","username":"admin@aiwx.local","password":"admin"}`.

This will be my answer to the discontinoued AWX project. 

BUT better.. (in all possible ways)

Features.
- need to run on postgress in a k8s cluster, cloudnative pg as backend
- backend needs to be lightning fast so make use of rust / tokio ascync message bus 
- use go if rust is not possible for example ? 
- it needs to have the same features as awx
    so: rbac, git projectsync, vault cred sync, joblog, execution envioronments 
- it needs to run ansible playbooks from execution envioronments (pods)
- it needs to have a nice gui
- it needs to be multi tenant
- for database sceme you can check how awx scheme now is(which is quite good)
- it needs to have a rest api
- it needs to have a k8s operator
- iterate iterate iterate till something usefull

FIRST Step is to gather all nescesserry information from these github repositorys and information which we need to create a plan creating a application.
save them in a information library stored here (local only, not in this repo):
~/aiwx_information


Local sibling repos used as reference (not in this repo):
~/awx
~/awx-operator
~/ansible-ui
~/awx-ee