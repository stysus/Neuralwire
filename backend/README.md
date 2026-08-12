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
- Cron scheduler (default: every 6 hours) that fetches feeds and inserts
  articles as **drafts** (semi-automatic moderation workflow)
- AI summaries via any OpenAI-compatible API (`gpt-4o-mini` by default);
  falls back to a plain 300-character excerpt when no API key is set
- CORS enabled for `http://localhost:5173` and `http://127.0.0.1:5173` (SvelteKit dev server)

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
on first run. On boot it runs an initial RSS fetch and then polls every
6 hours.

## Environment variables

| Variable             | Default                       | Description                                          |
| -------------------- | ----------------------------- | ---------------------------------------------------- |
| `PORT`               | `8080`                        | HTTP listen port                                     |
| `DB_PATH`            | `data/neuralwire.db`          | SQLite database file path                            |
| `CORS_ALLOW_ORIGIN`  | `http://localhost:5173,http://127.0.0.1:5173` | Comma-separated allowed frontend origins |
| `AI_SUMMARY_API_KEY` | *(empty)*                     | API key for the OpenAI-compatible summary endpoint   |
| `AI_SUMMARY_MODEL`   | `gpt-4o-mini`                 | Model used for summaries                             |
| `AI_SUMMARY_BASE_URL`| `https://api.openai.com/v1`   | OpenAI-compatible API base URL                       |
| `CRON_SCHEDULE`      | `0 */6 * * *`                 | Cron expression for the RSS fetcher                  |
| `FETCH_ON_STARTUP`   | `true`                        | Run one fetch cycle when the server boots            |

Without `AI_SUMMARY_API_KEY`, articles are summarized by truncating the
plain-text content to 300 characters.

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

| Method | Path                              | Description                       |
| ------ | --------------------------------- | --------------------------------- |
| POST   | `/api/admin/news`                 | Create a draft article            |
| POST   | `/api/admin/news/{id}/publish`    | Publish a draft                   |
| POST   | `/api/admin/news/{id}/reject`     | Reject a draft                    |
| DELETE | `/api/admin/news/{id}`            | Delete an article                 |

### Create draft example

```bash
curl -X POST http://localhost:8080/api/admin/news \
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
# List published news
curl http://localhost:8080/api/news?category=ai&page=1&page_size=10

# Publish the draft created above (replace {id})
curl -X POST http://localhost:8080/api/admin/news/{id}/publish

# Reject instead
curl -X POST http://localhost:8080/api/admin/news/{id}/reject

# Delete
curl -X DELETE http://localhost:8080/api/admin/news/{id}
```

## Project layout

```
backend/
├── cmd/server/          # entrypoint
└── internal/
    ├── ai/              # OpenAI-compatible summarizer (+ fallback)
    ├── api/             # HTTP handlers, middleware, routing
    ├── config/          # env-var configuration
    ├── database/        # SQLite open, schema, seed data
    ├── fetcher/         # RSS/Atom polling
    ├── models/          # domain types
    ├── repository/      # SQL data access
    ├── scheduler/       # cron scheduling
    └── slug/            # slug helpers
```

## Development

```bash
go test ./...        # run tests
go vet ./...         # static analysis
make run             # build and run (see Makefile)
```
