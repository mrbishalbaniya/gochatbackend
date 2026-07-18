# CI/CD

GitHub Actions run on every push/PR.

## Backend (`gochatbackend`)

| Workflow | Trigger | What it does |
|----------|---------|--------------|
| `CI` | push / PR | `go vet`, `go test`, `go build` |
| `Deploy` | push to `main` | Runs CI, then hits Render Deploy Hook |

### Render setup

1. Render → your Web Service → **Settings** → **Deploy Hook** → copy URL  
2. GitHub → `gochatbackend` → **Settings** → **Secrets and variables** → **Actions**  
3. Add secret: `RENDER_DEPLOY_HOOK` = that URL  

Also enable **Auto-Deploy** on Render if you want deploys without the hook (either works).

## Frontend (`gochatfrontend`)

| Workflow | Trigger | What it does |
|----------|---------|--------------|
| `CI` | push / PR | `lint`, `tsc`, `next build` |
| `Deploy` | push to `main` | Runs CI, then optional Vercel CLI deploy |

### Vercel setup (pick one)

**A. Recommended:** Import the repo in [vercel.com](https://vercel.com) — Vercel deploys on every push to `main`.

**B. CLI from Actions:** add secrets:

- `VERCEL_TOKEN` — from Vercel → Account → Tokens  
- `VERCEL_ORG_ID` — from `.vercel/project.json` after `vercel link`  
- `VERCEL_PROJECT_ID` — same file  

```bash
cd frontend
npx vercel link
# copy orgId / projectId into GitHub secrets
```

## Production env (Vercel)

```env
NEXT_PUBLIC_API_URL=https://YOUR-SERVICE.onrender.com/api/v1
NEXT_PUBLIC_WS_URL=wss://YOUR-SERVICE.onrender.com/ws
NEXT_PUBLIC_CALL_API_URL=https://YOUR-SERVICE.onrender.com/api/v1
NEXT_PUBLIC_CALL_WS_URL=wss://YOUR-SERVICE.onrender.com/ws/calls
```
