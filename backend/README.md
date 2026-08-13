# Neuralwire Backend

Go REST API backend for the **Neuralwire** AI news website (global audience,
English by default).

## Features

- REST API on port `8080`
- SQLite storage (pure-Go driver, no CGO required)
- RSS feed fetcher for 14 AI sources (OpenAI, Google AI, Anthropic, Meta AI,
  DeepMind, Hugging Face, AWS ML, GitHub, MIT AI, arXiv, TechCrunch AI,
  VentureBeat AI, The Verge AI, Machine Learning Mastery)
- **Manual RSS ingestion** (no background scheduler): trigger a fetch cycle
  with `POST /api/admin/fetch`; fetched articles land as **drafts** in a
  semi-automatic moderation workflow
- **Curator model**: the scraper reads the full article only as material for
  the AI summary, then discards it. Original full text is **never stored or
  republished**, keeping the site safe from copyright issues. Readers are
  directed to the source via the article URL.
- AI summaries, categorization and news-value scoring via any
  OpenAI-compatible API (works with reasoning models such as DeepSeek);
  graceful fallbacks when the API is unavailable
- **AI value scoring (Level 2)**: every draft is rated 0-100 by a weighted
  blend of AI judgment and deterministic heuristics, labelled HIGH/MEDIUM/LOW
  with a sub-score breakdown, confidence, advisory recommendation and reason.
  Scoring is advisory only — it **never auto-publishes**; admins stay the
  final decision makers.
- **Admin-configurable scoring thresholds** persisted in `app_settings`
  (defaults: LOW <60, MEDIUM 60–79, HIGH ≥80) via `GET/PUT /api/admin/settings`
- Live fetch progress (`GET /api/admin/fetch/progress`) that survives page
  refreshes, plus **cancel in-flight fetch** (`POST /api/admin/fetch/cancel`)
- Cover images are upgraded to high-resolution CDN variants (Contentful,
  imgix, Cloudinary, Unsplash, Google, WordPress) so heroes render sharply
- **Trending / most-read ranking**: public `POST /api/news/{id}/view` records
  article reads (deduplicated per visitor via `viewer_key` + 6h cooldown) and
  `GET /api/news/trending?window=day|week|all&limit=N` returns the most-read
  published articles with view counts
- CORS enabled for `http://localhost:5173` and `http://127.0.0.1:5173` (SvelteKit dev server)
- **Security headers** on every response: `X-Content-Type-Options: nosniff`,
  `X-Frame-Options: DENY`, `Referrer-Policy`, `Permissions-Policy`, and a
  permissive-for-images `Content-Security-Policy` (anti-XSS/clickjacking)
- **CSRF protection** on admin mutations: state-changing admin requests with a
  browser `Origin` header must come from an allowed origin; non-browser
  clients (no Origin) are allowed since they still need the bearer token
- Admin API protected by simple bearer-token auth (`POST /api/admin/login`)

## Requirements

- Go 1.25+ (newer toolchains are downloaded automatically when using the
  standard toolchain settings)

## Quick start

```bash
cd backend
go build -o bin/server ./cmd/server
./bin/server
```

The server listens on `http://localhost:8080` and creates `data/neuralwire.db`
on first run. RSS ingestion is **manual**: there is no automatic fetching or
cron scheduler, so restarts never flood the drafts table. Trigger a fetch
cycle with `POST /api/admin/fetch` (see below).

## Environment variables

| Variable             | Default                       | Description                                          |
| -------------------- | ----------------------------- | ---------------------------------------------------- |
| `PORT`               | `8080`                        | HTTP listen port                                     |
| `APP_ENV`            | `development`                 | Runtime environment. In `production`, startup refuses default admin credentials or the dev token secret |
| `TRUST_PROXY`        | `false`                       | Trust `X-Forwarded-For` for client IP (enable only behind a trusted reverse proxy; otherwise spoofable) |
| `DB_PATH`            | `data/neuralwire.db`          | SQLite database file path                            |
| `USER_AGENT`         | `Mozilla/5.0 (compatible; NeuralwireBot/1.0-dev; +https://neuralwire.example)` | User-Agent for outbound RSS/scrape requests; set a real bot UA + domain before going public |
| `CORS_ALLOW_ORIGIN`  | `http://localhost:5173,http://127.0.0.1:5173` | Comma-separated allowed frontend origins |
| `AI_SUMMARY_API_KEY` | *(empty)*                     | API key for the OpenAI-compatible summary endpoint   |
| `AI_SUMMARY_PROVIDER`| `openai`                      | Preset: `openai`, `gemini`, `openrouter`, `groq`, `ollama` |
| `AI_SUMMARY_MODEL`   | `gpt-4o-mini`                 | Model used for summaries                             |
| `AI_SUMMARY_BASE_URL`| `https://api.openai.com/v1`   | OpenAI-compatible API base URL                       |
| `SCRAPE_MAX_PER_SOURCE` | `5`                        | Newest articles scraped per source per cycle         |
| `SCRAPE_MAX_INSERT_PER_SOURCE` | `5`               | Hard cap on new drafts stored per source per cycle (scraped or fallback alike) |
| `SCRAPE_TIMEOUT_SECONDS` | `15`                      | Per-article scrape timeout (seconds)                 |
| `SCRAPE_DELAY_MIN_SECONDS` | `1`                     | Lower bound of the random politeness delay (seconds) between external requests |
| `SCRAPE_DELAY_MAX_SECONDS` | `2`                     | Upper bound of the random politeness delay (seconds) between external requests |
| `SCRAPE_MIN_CONTENT_CHARS` | `500`                    | Minimum content length for a fetched draft; shorter articles are skipped as low quality |
| `VIEW_RATE_LIMIT` | `30`                     | Per-IP rate limit (per minute) for `POST /api/news/{id}/view`; `<=0` disables |
| `TRENDING_CACHE_TTL_SECONDS` | `300`               | Cache TTL (seconds) for trending results; `<=0` disables |
| `LOGIN_RATE_LIMIT` | `5`                       | Per-IP login attempts per minute (anti brute force); `<=0` disables |
| `GLOBAL_RATE_LIMIT` | `120`                    | Per-IP requests per minute for every endpoint (anti scan/bot); `<=0` disables |
| `ADMIN_USERNAME`     | `admin`                       | Admin login username                                 |
| `ADMIN_PASSWORD`     | `admin123`                    | Admin login password. **Change it outside development** |
| `ADMIN_TOKEN_SECRET` | dev value (see `.env.example`) | HMAC secret signing admin bearer tokens. **Change it outside development** |

Without `AI_SUMMARY_API_KEY`, articles fall back to deterministic summaries,
categories and heuristic value scores, so the pipeline still produces
reviewable drafts.

## How ingestion works (curator model)

There is **no background scheduler** — fetching only happens when an
authenticated admin calls `POST /api/admin/fetch`, which runs one fetch cycle
across all sources and returns per-source statistics:

```json
{
  "total_new": 3,
  "scraped": 2,
  "fallback": 1,
  "skipped_low_quality": 10,
  "sources": [
    { "name": "OpenAI Blog", "inserted": 2, "scraped": 2, "fallback": 0, "skipped_low_quality": 0 },
    { "name": "Reddit r/artificial", "inserted": 1, "scraped": 0, "fallback": 1, "skipped_low_quality": 0 }
  ]
}
```

RSS feeds only expose short excerpts, so each new article goes through a
hybrid extraction:

1. For the **newest N articles per source per cycle** (`SCRAPE_MAX_PER_SOURCE`,
   default 5) it fetches the article URL and extracts the full readable body
   with a readability algorithm (codeberg.org/readeck/go-readability/v2),
   after stripping navigation, ads, sidebars and interactive widgets.
   Before each external request (RSS feed fetch or article scrape) the
   fetcher waits a random delay between `SCRAPE_DELAY_MIN_SECONDS` and
   `SCRAPE_DELAY_MAX_SECONDS` (default 1–2s), so upstream sites see at most
   one request per 1–2 seconds.
2. The scraped body is used **only as material** for the AI summary,
   categorization and value scoring. The original text is then discarded:
   the stored article keeps a `summary` and an empty `content`. The site
   never republishes original copyrighted text; each article links out to the
   source via its URL.
3. If scraping succeeds, the scraped title replaces the RSS title when it is
   longer/more descriptive.
4. If scraping fails, times out (`SCRAPE_TIMEOUT_SECONDS`, default 15s) or
   the budget is exceeded, the article falls back to the RSS excerpt
   (`item.Content` / `item.Description`) as summarize/scoring material.
5. **Quality gate:** articles whose material is shorter than
   `SCRAPE_MIN_CONTENT_CHARS` (default 500) are **not** stored as drafts;
   they are logged as `skipped (low quality)`.
6. **Per-source insert budget:** at most `SCRAPE_MAX_INSERT_PER_SOURCE`
   (default 5) new drafts are stored per source per cycle, whether they came
   from scraping or an RSS-excerpt fallback. The loop stops once the cap is
   reached, so a single fetch cannot flood the drafts table even when RSS
   excerpts are long.
7. **Image extraction:** if the RSS feed provided no usable image, the first
   `<img>` from the scraped HTML (already absolute) becomes `image_url`. Cover
   URLs are passed through `UpgradeImageURL` so known CDNs (Contentful,
   imgix, Cloudinary, Unsplash, Google, WordPress) return a high-resolution
   variant instead of a small thumbnail (e.g. `?w=300&q=30` → `?w=1600&q=80&fm=webp`).
8. **Value scoring:** each draft is rated by `scoring.ScoreService` —
   `0.6 × AI score + 0.4 × heuristic score` — and labelled HIGH/MEDIUM/LOW
   using the configurable thresholds. The AI supplies impact/novelty/quality
   sub-scores, confidence, an advisory recommendation and a reason; when the
   AI is unavailable the heuristic score is used with `method: "heuristic"`.

The per-source log line reports the split, e.g.
`fetcher: source "Google AI Blog" inserted 5 new draft(s) (scraped: 5, fallback: 0, skipped-low-quality: 0)`.

## Value scoring (Level 2 AI)

Every fetched draft is rated before it is stored. The pipeline in
`internal/scoring`:

1. **AI verdict** — `ai.ScoreValue` asks the model for `{score, impact,
   novelty, quality, confidence, recommendation, reason}` as JSON. The
   parser is tolerant (strips code fences, clamps ranges).
2. **Heuristic fallback** — `scoring.RuleScorer` scores deterministically
   from source authority, headline signals (launch/rumor keywords) and
   evidence density (numbers, article length). Used when the AI is
   unavailable, and always blended into the final score.
3. **Weighted final score** — `0.6 × AI score + 0.4 × heuristic score`,
   computed on the backend (never trusted wholesale to the model).
4. **Label** — mapped to `HIGH` / `MEDIUM` / `LOW` using the thresholds from
   `app_settings` (defaults LOW <60, MEDIUM 60–79, HIGH ≥80). Thresholds are
   editable by admins via `GET/PUT /api/admin/settings` — never hardcoded.
5. **Advisory only** — scoring attaches `value_score`, `value_breakdown`,
   `value_confidence`, `value_recommendation`, `value_reason`,
   `value_label`, `value_method` to a draft. It **never auto-publishes**;
   admins review and decide in the admin panel.

## API

All endpoints return JSON. List endpoints use a `{ "data": [...], "pagination": {...} }`
envelope.

### Public

| Method | Path                | Description                                        |
| ------ | ------------------- | -------------------------------------------------- |
| GET    | `/api/health`       | Liveness check                                     |
| GET    | `/api/news`         | Published articles; `?category=`, `?q=` (title/summary keyword search, multi-word tokens matched with AND), `?page=`, `?page_size=` |
| GET    | `/api/news/{id}`    | Single published article (404 if draft/rejected)   |
| GET    | `/api/news/trending`| Most-read published articles; `?window=day\|week\|all` (default week), `?limit=` (default 5) |
| POST   | `/api/news/{id}/view` | Record one read of an article; optional body `{viewer_key}` for per-visitor dedup |
| GET    | `/api/categories`   | All categories                                     |

### Admin (moderation workflow)

**Authentication:** all `/api/admin/*` routes except `POST /api/admin/login`
require an `Authorization: Bearer <token>` header. Obtain a token by logging
in; tokens are HMAC-signed with `ADMIN_TOKEN_SECRET` and expire after 24h.

| Method | Path                              | Description                       |
| ------ | --------------------------------- | --------------------------------- |
| POST   | `/api/admin/login`                | Login, returns `{token, token_type, expires_in}` |
| POST   | `/api/admin/fetch`                | Manually trigger one RSS fetch cycle, returns fetch stats |
| GET    | `/api/admin/fetch/progress`       | Live progress of the in-flight cycle `{running, done_sources, total_sources, percent, current_source}` |
| POST   | `/api/admin/fetch/cancel`         | Abort the running fetch cycle (`cancelled: true/false`) |
| GET    | `/api/admin/settings`             | Current scoring thresholds `{low_max, medium_min, medium_max, high_min}` |
| PUT    | `/api/admin/settings`             | Update scoring thresholds (persisted in `app_settings`) |
| GET    | `/api/admin/news`                 | All articles (any status); `?status=draft\|published\|rejected`, `?category=`, `?value_label=HIGH\|MEDIUM\|LOW`, `?page=`, `?page_size=` |
| GET    | `/api/admin/news/{id}`            | Full article (including content) regardless of status |
| POST   | `/api/admin/news`                 | Create a draft article            |
| POST   | `/api/admin/news/{id}/publish`    | Publish a draft                   |
| POST   | `/api/admin/news/{id}/reject`     | Reject a draft                    |
| DELETE | `/api/admin/news/{id}`            | Delete an article                 |

### Manual fetch example

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/admin/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r .token)

curl -X POST http://localhost:8080/api/admin/fetch \
  -H "Authorization: Bearer $TOKEN"
# => {"total_new":3,"scraped":2,"fallback":1,"skipped_low_quality":10,"sources":[...]}
```

### Login example

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/admin/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r .token)
```

### Create draft example

```bash
curl -X POST http://localhost:8080/api/admin/news \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "OpenAI releases a new model",
    "url": "https://openai.com/blog/new-model",
    "source": "OpenAI Blog",
    "category": "ai",
    "summary": "OpenAI announced a new model today.",
    "content": "Full article body...",
    "image_url": "https://openai.com/cover.png"
  }'
```

### Workflow example

```bash
# List published news (public, no auth)
curl http://localhost:8080/api/news?category=ai&page=1&page_size=10

# List all articles including drafts (admin)
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/admin/news?status=draft&page=1&page_size=20"

# Get the full draft article (admin)
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/admin/news/{id}

# Publish the draft created above (replace {id})
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/admin/news/{id}/publish

# Reject instead
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/admin/news/{id}/reject

# Delete
curl -X DELETE -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/admin/news/{id}
```

## Project layout

```
backend/
├── cmd/server/          # entrypoint
└── internal/
    ├── ai/              # OpenAI-compatible summarizer/categorizer/scorer (+ fallback)
    ├── api/             # HTTP handlers, middleware, routing
    ├── auth/            # HMAC bearer-token issue/validate
    ├── config/          # env-var configuration
    ├── database/        # SQLite open, schema (additive migrations), seed data
    ├── fetcher/         # RSS/Atom polling (+ politeness throttle, insert budget)
    ├── models/          # domain types (+ value-scoring fields, thresholds)
    ├── repository/      # SQL data access (+ settings_repo for app_settings)
    ├── scoring/         # AI+heuristic weighted news-value scoring
    ├── scraper/         # full-content extraction + image URL upgrade
    └── slug/            # slug helpers
```

## Development

```bash
go test ./...        # run tests
go vet ./...         # static analysis
make run             # build and run (see Makefile)
```
