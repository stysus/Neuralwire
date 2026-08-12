# Neuralwire Backend

Go REST API backend for the **Neuralwire** AI news website (global audience,
English by default).

## Features

- REST API on port `8080`
- SQLite storage (pure-Go driver, no CGO required)
- RSS feed fetcher for 5 AI sources:
  - OpenAI Blog
  - Google AI Blog
  - Hugging Face Blog
  - arXiv cs.AI
  - Reddit r/artificial
- **Manual RSS ingestion** (no background scheduler): trigger a fetch cycle
  with `POST /api/admin/fetch`; fetched articles land as **drafts** in a
  semi-automatic moderation workflow
- Full-content scraper: RSS excerpts are too short, so the fetcher scrapes
  the newest articles per source for their full readable body (readability
  extraction + boilerplate stripping), falling back to the RSS excerpt when
  scraping fails; summaries are generated from the full content
- AI summaries via any OpenAI-compatible API (`gpt-4o-mini` by default);
  falls back to a plain 300-character excerpt when no API key is set
- CORS enabled for `http://localhost:5173` and `http://127.0.0.1:5173` (SvelteKit dev server)
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
| `DB_PATH`            | `data/neuralwire.db`          | SQLite database file path                            |
| `CORS_ALLOW_ORIGIN`  | `http://localhost:5173,http://127.0.0.1:5173` | Comma-separated allowed frontend origins |
| `AI_SUMMARY_API_KEY` | *(empty)*                     | API key for the OpenAI-compatible summary endpoint   |
| `AI_SUMMARY_PROVIDER`| `openai`                      | Preset: `openai`, `gemini`, `openrouter`, `groq`, `ollama` |
| `AI_SUMMARY_MODEL`   | `gpt-4o-mini`                 | Model used for summaries                             |
| `AI_SUMMARY_BASE_URL`| `https://api.openai.com/v1`   | OpenAI-compatible API base URL                       |
| `SCRAPE_MAX_PER_SOURCE` | `20`                       | Newest articles scraped per source per cycle         |
| `SCRAPE_TIMEOUT_SECONDS` | `15`                      | Per-article scrape timeout (seconds)                 |
| `SCRAPE_MIN_CONTENT_CHARS` | `500`                    | Minimum content length for a fetched draft; shorter articles are skipped as low quality |
| `ADMIN_USERNAME`     | `admin`                       | Admin login username                                 |
| `ADMIN_PASSWORD`     | `admin123`                    | Admin login password. **Change it outside development** |
| `ADMIN_TOKEN_SECRET` | dev value (see `.env.example`) | HMAC secret signing admin bearer tokens. **Change it outside development** |

Without `AI_SUMMARY_API_KEY`, articles are summarized by truncating the
plain-text content to 300 characters.

## How full-content scraping works

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
   default 20) it fetches the article URL and extracts the full readable body
   with a readability algorithm (codeberg.org/readeck/go-readability/v2),
   after stripping navigation, ads, sidebars and interactive widgets.
2. The extracted article is stored as **clean HTML** (headings, paragraphs,
   lists, figures and images preserved), so the frontend can render it with
   `{@html ...}`. Image `src`/`srcset`, `<source>` and `<video poster>`
   URLs are rewritten to absolute against the final (post-redirect) URL,
   and leftover template placeholders such as `[[duration]]` are removed.
3. If scraping succeeds, the scraped HTML becomes the article `content` (and
   a better scraped title replaces the RSS title when it is longer/more
   descriptive). The AI summarizer then summarizes the **full** scraped
   content.
4. If scraping fails, times out (`SCRAPE_TIMEOUT_SECONDS`, default 15s) or
   the budget is exceeded, the article falls back to the RSS excerpt
   (`item.Content` / `item.Description`).
5. **Quality gate:** articles whose final content is shorter than
   `SCRAPE_MIN_CONTENT_CHARS` (default 500) are **not** stored as drafts;
   they are logged as `skipped (low quality)`.
6. **Image extraction:** if the RSS feed provided no usable image, the first
   `<img>` from the scraped HTML (already absolute) becomes `image_url`. No
   images are generated or replaced — the original scraped media is used.

The per-source log line reports the split, e.g.
`fetcher: source "Google AI Blog" inserted 20 new draft(s) (scraped: 18, fallback: 2)`.

## API

All endpoints return JSON. List endpoints use a `{ "data": [...], "pagination": {...} }`
envelope.

### Public

| Method | Path                | Description                                        |
| ------ | ------------------- | -------------------------------------------------- |
| GET    | `/api/health`       | Liveness check                                     |
| GET    | `/api/news`         | Published articles; `?category=`, `?page=`, `?page_size=` |
| GET    | `/api/news/{id}`    | Single published article (404 if draft/rejected)   |
| GET    | `/api/categories`   | All categories                                     |

### Admin (moderation workflow)

**Authentication:** all `/api/admin/*` routes except `POST /api/admin/login`
require an `Authorization: Bearer <token>` header. Obtain a token by logging
in; tokens are HMAC-signed with `ADMIN_TOKEN_SECRET` and expire after 24h.

| Method | Path                              | Description                       |
| ------ | --------------------------------- | --------------------------------- |
| POST   | `/api/admin/login`                | Login, returns `{token, token_type, expires_in}` |
| POST   | `/api/admin/fetch`                | Manually trigger one RSS fetch cycle, returns fetch stats |
| GET    | `/api/admin/news`                 | All articles (any status); `?status=draft\|published\|rejected`, `?page=`, `?page_size=` |
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
    ├── ai/              # OpenAI-compatible summarizer (+ fallback)
    ├── api/             # HTTP handlers, middleware, routing
    ├── auth/            # HMAC bearer-token issue/validate
    ├── config/          # env-var configuration
    ├── database/        # SQLite open, schema, seed data
    ├── fetcher/         # RSS/Atom polling
    ├── models/          # domain types
    ├── repository/      # SQL data access
    ├── scraper/         # full-content extraction (readability + cleanup)
    └── slug/            # slug helpers
```

## Development

```bash
go test ./...        # run tests
go vet ./...         # static analysis
make run             # build and run (see Makefile)
```
