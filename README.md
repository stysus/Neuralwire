# Neuralwire — AI News & Editorial Platform

> An editorial news portal for artificial intelligence, neural networks, and the future of computation. Bridging the gap between silicon and humanity.

**Live site:** [https://neuralwire.info](https://neuralwire.info)

---

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Features](#features)
- [Getting Started](#getting-started)
- [Environment Variables](#environment-variables)
- [Development Workflow](#development-workflow)
- [CI/CD](#cicd)
- [Deployment](#deployment)
- [Security](#security)
- [Project Structure](#project-structure)
- [License](#license)

---

## Overview

Neuralwire is a **semi-automated news curation platform**. It ingests RSS feeds from curated sources, extracts readable article content, generates AI summaries and category classifications, scores news value, and presents the results in a modern editorial frontend.

The system uses a **curator model**: AI assists with summarization, categorization, and value scoring, but **human admins make all publishing decisions**. Nothing is auto-published — every article is reviewed as a draft before going live.

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Browser (visitor)                     │
│        https://neuralwire.info (Cloudflare CDN)          │
└──────────────────────┬──────────────────────────────────┘
                       │ HTTPS
┌──────────────────────▼──────────────────────────────────┐
│              Railway (single container)                  │
│  ┌──────────────────────────┐  ┌──────────────────────┐  │
│  │   Go Backend (port 8080) │  │  SvelteKit Frontend  │  │
│  │   REST API + static serve│  │  (adapter-static)    │  │
│  └────────────┬─────────────┘  └──────────▲───────────┘  │
│               │  serves /api/* & static    │             │
│  ┌────────────▼─────────────┐              │             │
│  │      SQLite (volume)     │              │             │
│  │   /app/data/neuralwire.db│              │             │
│  └──────────────────────────┘              │             │
└────────────────────────────────────────────┼─────────────┘
                                             │
                        ┌────────────────────▼──────────┐
                        │  External RSS/Atom Sources     │
                        │  + AI API (DeepSeek, OpenAI)   │
                        └───────────────────────────────┘
```

**Key design decision:** The frontend is built with `adapter-static` and **served by the Go backend itself** (same origin). This means:
- No separate static hosting needed
- API calls use relative paths (`/api/*`)
- One container serves everything — simple deployment

---

## Tech Stack

### Backend
- **Go 1.25** — high-performance, type-safe REST API
- **SQLite** (`modernc.org/sqlite`) — pure-Go, zero-CGO, single-file database
- **go-readability** — article content extraction
- **gofeed** — RSS/Atom parsing
- **Standard library** `net/http` + `log/slog` — minimal dependencies, no heavy frameworks

### Frontend
- **SvelteKit 2** — modern, compiler-based framework
- **Svelte 5** (runes mode)
- **Tailwind CSS 4** — utility-first styling
- **adapter-static** — prerendered static output
- **TypeScript**

### Infrastructure
- **Railway** — container platform (auto-deploy from GitHub)
- **Docker** — multi-stage build (node → go → alpine runtime)
- **Cloudflare** — CDN, WAF, DNS, SSL
- **GitHub Actions** — CI (build, vet, test, lint)
- **Let's Encrypt** — SSL certificates (via Railway/Cloudflare)

---

## Features

### Content Pipeline
- **RSS ingestion** — fetches multiple feeds with polite rate limiting (1-2s random delay)
- **Article scraping** — extracts full readable content via readability
- **AI summarization** — concise summaries via OpenAI-compatible APIs (DeepSeek, Gemini, etc.)
- **AI categorization** — auto-classifies into categories
- **AI value scoring** — 0-100 news value rating (impact, novelty, quality) with confidence
- **Heuristic fallback** — deterministic scoring when AI is unavailable

### Curator Model
- Articles arrive as **drafts** — never auto-published
- Admin reviews, edits, publishes, or rejects
- **Value labels** (High/Medium/Low) help admins prioritize
- Live fetch progress + cancel in-flight fetch

### Public Features
- Home feed with featured articles
- Category pages (AI, Industry, Machine Learning, Research, Tools)
- **Trending / most-read** ranking (view dedup per visitor)
- **Related articles** (TF-IDF weighted similarity)
- **Search** (backend multi-word AND matching)
- **SEO**: sitemap.xml, robots.txt, Open Graph, Twitter Card, canonical URLs

### Monitoring & Performance
- **Structured logging** (Go `slog`, JSON or text, configurable level)
- **Health checks** — `/api/health` & `/api/healthz` (DB ping, 200/503)
- **Prometheus metrics** — `/api/metrics` (requests, latency, fetch cycles, AI calls)
- **HTTP compression** — gzip/brotli, ~70% bandwidth savings
- **ETag conditional requests** — 304 Not Modified
- **Per-route cache headers**
- **DB indexes** for hot paths (verified via EXPLAIN QUERY PLAN)

### Security
- Bearer-token admin auth (constant-time comparison)
- **CSRF protection** on admin mutations
- **Rate limiting** — login (brute force), views, global (anti-scan)
- **Security headers** — CSP, X-Frame-Options, nosniff, etc.
- **Client IP anti-spoofing** (TRUST_PROXY aware)
- **Production hardening** — refuses to boot with default credentials
- **Cloudflare** — WAF, anti-DDoS, CDN

---

## Getting Started

### Prerequisites
- Go 1.25+
- Node.js 24+
- npm

### 1. Clone & install

```bash
git clone git@github.com:stysus/Neuralwire.git
cd Neuralwire
```

### 2. Backend setup

```bash
cd backend
cp .env.example .env        # configure your env
go mod download
go run ./cmd/server         # starts on :8080
```

### 3. Frontend setup

```bash
cd frontend
cp .env.example .env.local  # set PUBLIC_API_URL=http://localhost:8080/api
npm install
npm run dev                 # starts on :5173
```

### 4. Access
- Frontend: http://localhost:5173
- API: http://localhost:8080/api
- Admin login: http://localhost:5173/admin

---

## Environment Variables

### Backend (`backend/.env`)

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `APP_ENV` | `development` | `production` enables hardening |
| `DB_PATH` | `data/neuralwire.db` | SQLite file location |
| `ADMIN_USERNAME` | `admin` | Admin login |
| `ADMIN_PASSWORD` | `admin123` | **Change in production** |
| `ADMIN_TOKEN_SECRET` | dev value | **Change in production** |
| `AI_SUMMARY_API_KEY` | *(empty)* | OpenAI-compatible API key |
| `AI_SUMMARY_PROVIDER` | `openai` | `openai`, `gemini`, `deepseek`, `groq`, `ollama` |
| `CORS_ALLOW_ORIGIN` | localhost:5173 | Allowed frontend origins |
| `STATIC_DIR` | `../frontend/build` | Served static frontend |
| `LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error` |
| `LOG_FORMAT` | `text` | `text`/`json` |
| `GLOBAL_RATE_LIMIT` | `120` | Per-IP req/min (0=off) |
| `LOGIN_RATE_LIMIT` | `5` | Login attempts/min (0=off) |
| `VIEW_RATE_LIMIT` | `30` | View count req/min (0=off) |

### Frontend (`frontend/.env.local`)

| Variable | Description |
|---|---|
| `PUBLIC_API_URL` | Backend API URL (dev: `http://localhost:8080/api`, **production: leave unset** → uses relative `/api`) |
| `PUBLIC_SITE_URL` | Public site origin (used for sitemap/OG/canonical) |

---

## Development Workflow

This repo uses a **two-branch workflow with branch protection**:

```
development (work freely, CI runs, NO deploy)
      │  push + open PR
      ▼
main (protected, CI must pass, deploys to production)
```

1. Create a feature branch from `development`
2. Commit and push — CI runs on every push
3. Open a **Pull Request** to `main`
4. CI must pass (`Build, vet & test` + `Install, check, lint & build`)
5. Merge → Railway auto-deploys to production

> `main` is **protected**: direct pushes are blocked, force-push is blocked, and all commits must come through PRs with green CI.

---

## CI/CD

### GitHub Actions (`.github/workflows/`)

**`backend-ci.yml`** — on push/PR touching `backend/**`:
1. `go vet ./...`
2. `go build ./...`
3. `gofmt` format check
4. `go test ./... -race`
5. Builds & uploads server binary artifact

**`frontend-ci.yml`** — on push/PR touching `frontend/**`:
1. `npm ci`
2. `svelte-check` (type check)
3. Prettier + ESLint
4. `vite build`

### Continuous Deployment

- **Railway** watches the `main` branch
- Every push to `main` → auto-builds Docker image → deploys
- **Wait for CI** enabled — Railway waits for GitHub CI to pass before deploying
- Production: https://neuralwire.info (behind Cloudflare)

---

## Deployment

### Docker (production)

The root `Dockerfile` is multi-stage:
1. **node:24-alpine** — builds SvelteKit frontend (`adapter-static` → `build/`)
2. **golang:1.25-alpine** — compiles Go backend (`CGO_ENABLED=0`)
3. **alpine:3.20** — minimal runtime, non-root user, health check

```bash
# Local build & run
docker build -t neuralwire .
docker run -p 8080:8080 \
  -e APP_ENV=production \
  -e ADMIN_USERNAME=... -e ADMIN_PASSWORD=... -e ADMIN_TOKEN_SECRET=... \
  neuralwire
```

Or with docker-compose:
```bash
docker compose up -d --build
```

### Railway (current production)

1. Connect GitHub repo to Railway
2. Set environment variables (see [Environment Variables](#environment-variables))
3. Attach a **volume at `/app/data`** (SQLite persistence)
4. Railway auto-builds from the Dockerfile and deploys

> **Volume note:** the container runs as non-root user `neuralwire`. The `docker-entrypoint.sh` chowns the attached volume before starting so SQLite can write.

### Releases & Rollback

**Releases are tagged** with semantic versions. The latest tag deployed to production is the current release.

```bash
# Create & push a new release tag
git tag -a v1.1.0 -m "Release v1.1.0"
git push origin v1.1.0
```

**Rollback — 1 of 2 ways:**

1. **Via Railway UI (fastest):** Railway → Service → Deployments → select the previous successful deployment → **Redeploy**.

2. **Via git tag (recommended for permanent rollback):**
```bash
# Point main back at a known-good release and push (through a PR)
git checkout -b rollback-v1.0.0 origin/main
git revert --no-commit <bad-commit-range>   # or reset to the tag
git push -u origin rollback-v1.0.0
# Open a PR -> CI must pass -> merge -> Railway auto-deploys
```

**When to rollback:**
- 5xx errors spike after a deploy
- Admin login / fetch pipeline broken
- Frontend blank or assets 404
- Health check (`/api/healthz`) not returning 200

**After rollback:** investigate the root cause on `development`, fix it, and ship a new release — do not keep patching the rolled-back code.

---

## Security

- **Secret management**: all secrets live in Railway environment variables, never in the repo
- **`.env` files**: gitignored, only `.env.example` templates are committed
- **Production guard**: `APP_ENV=production` refuses to boot with default admin credentials or dev token secret
- **Auth**: bearer tokens signed with HMAC (constant-time validation)
- **CSRF**: origin check on admin mutations
- **Rate limiting**: login brute-force, view abuse, global anti-scan
- **Headers**: CSP, X-Frame-Options DENY, nosniff, Referrer-Policy, Permissions-Policy
- **Cloudflare**: WAF rules, DDoS protection, bot mitigation

---

## Project Structure

```
.
├── .github/workflows/        # CI pipelines
├── backend/
│   ├── cmd/server/           # Go entrypoint
│   ├── internal/
│   │   ├── ai/               # AI summarization, categorization, image gen
│   │   ├── api/              # HTTP handlers, middleware, metrics
│   │   ├── auth/             # Bearer token auth
│   │   ├── cache/            # Simple TTL cache
│   │   ├── config/           # Env config loading
│   │   ├── database/         # SQLite schema & migrations
│   │   ├── fetcher/          # RSS ingestion pipeline
│   │   ├── metrics/          # Prometheus counters
│   │   ├── ratelimit/        # Per-IP rate limiting
│   │   ├── repository/       # Data access layer
│   │   ├── scoring/          # AI + heuristic news value scoring
│   │   ├── scraper/          # Readability content extraction
│   │   └── slug/             # URL slug generation
│   └── .env.example          # Backend env template
├── frontend/
│   ├── src/
│   │   ├── lib/              # API client, components, mock data
│   │   └── routes/           # SvelteKit pages (admin, [slug], category, etc.)
│   ├── static/               # robots.txt, images
│   └── .env.example          # Frontend env template
├── Dockerfile                # Multi-stage production image
├── docker-compose.yml        # Local container orchestration
└── docker-entrypoint.sh      # Volume permission fix + user drop
```

---

## API Endpoints

| Method | Path | Description | Auth |
|---|---|---|---|
| GET | `/api/health` | Health check (DB ping) | Public |
| GET | `/api/healthz` | Health check alias | Public |
| GET | `/api/metrics` | Prometheus metrics | Public |
| GET | `/api/news` | List published news (filter/search/page) | Public |
| GET | `/api/news/{id}` | Get article by ID | Public |
| GET | `/api/news/trending` | Most-read articles | Public |
| GET | `/api/news/{id}/related` | Related articles | Public |
| POST | `/api/news/{id}/view` | Record a view | Public |
| GET | `/api/categories` | List categories | Public |
| POST | `/api/admin/login` | Get auth token | Public |
| GET/POST/PUT/DELETE | `/api/admin/news` | Admin CRUD | Bearer |
| POST | `/api/admin/fetch` | Trigger RSS fetch | Bearer |
| GET | `/api/admin/fetch/progress` | Fetch progress | Bearer |
| POST | `/api/admin/fetch/cancel` | Cancel fetch | Bearer |
| GET/PUT | `/api/admin/settings` | App settings | Bearer |

---

## Roadmap / Backlog

Tracked in Linear (project: NEURALWIRE):
- Frontend lazy-loading & resource hints
- Analytics (privacy-friendly: Umami/Plausible)
- JSON-LD structured data
- PostgreSQL migration (future, if needed)
- Full CD pipeline with rollback strategy

---

## License

MIT License — see [LICENSE](LICENSE).

© 2026 NEURALWIRE MEDIA. All rights reserved for curated content; source code
is licensed under MIT.
