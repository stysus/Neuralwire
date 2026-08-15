# Contributing to Neuralwire

Thanks for your interest in contributing to Neuralwire! We welcome
contributions that improve the platform while keeping the curator model
intact: AI assists, humans decide what gets published.

Please read our [Code of Conduct](CODE_OF_CONDUCT.md) before participating.

## Ways to Contribute

- **Report bugs** — open an issue with clear steps to reproduce.
- **Suggest features** — open an issue describing the problem you're solving.
- **Fix / implement** — fork the repo, create a branch, open a pull request.

## Development Setup

### Prerequisites
- Go 1.25+
- Node.js 24+
- npm

### Backend
```bash
cd backend
cp .env.example .env   # configure env (defaults work for local dev)
go mod download
go run ./cmd/server    # starts on :8080
```

### Frontend
```bash
cd frontend
cp .env.example .env.local   # set PUBLIC_API_URL=http://localhost:8080/api
npm install
npm run dev                  # starts on :5173
```

## Branch & PR Workflow

This repo uses a two-branch workflow with branch protection:

```
development (work freely, CI runs, no deploy)
      │  push + open PR
      ▼
main (protected, CI must pass, deploys to production)
```

1. Create a feature branch from `development`.
2. Commit and push. CI runs on every push.
3. Open a **Pull Request** to `main`.
4. CI must pass:
   - `Build, vet & test` (backend)
   - `Install, check, lint & build` (frontend)
5. A maintainer reviews and merges.

> `main` is protected: direct pushes are blocked and all commits must come
> through PRs with green CI.

## Coding Standards

### Backend (Go)
- Run `gofmt` before committing (`gofmt -l .` must output nothing).
- Run `go vet ./...` and `go test ./...` locally.
- Keep dependencies minimal; prefer the standard library.

### Frontend (SvelteKit)
- Run `npm run check` (svelte-check) and `npm run lint` (prettier + eslint).
- Keep TypeScript types strict.
- Do not modify `backend/` in frontend PRs and vice versa.

## Commit Messages

Write concise, descriptive commit messages that explain the *why*:

```
type(scope): short summary

Body explaining context, especially non-obvious decisions.
```

Examples:
- `feat(api): add admin image upload endpoint`
- `fix(ui): article share buttons open correct share URLs`
- `perf(db): add index on news(url)`
- `docs: add contributing guide`

## Project Structure

```
backend/    Go REST API + scheduler
frontend/   SvelteKit app (adapter-static)
.github/    CI workflows
```

See the root [README](README.md) for full details.

## Questions?

Open an issue or reach out via the contact in our [Security Policy](SECURITY.md).
