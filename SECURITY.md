# Security Policy

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability in
Neuralwire, please **do not** open a public issue. Instead, report it
privately so we can fix it before it is disclosed.

**Email:** stysus@proton.me

Please include:
- A description of the vulnerability.
- Steps to reproduce (if possible).
- Affected version / commit.
- Any suggested fix (optional).

We aim to acknowledge reports within 3 business days and will keep you
updated on progress.

## Scope

In scope:
- The Go backend (REST API, scheduler, auth, rate limiting).
- The SvelteKit frontend (XSS, CSRF, client-side issues).
- Deployment configuration (Docker, env handling).

Out of scope:
- Content of curated articles (copyright issues go through the DMCA page).
- Third-party services (Railway, Cloudflare, GitHub) — report to their
  respective programs.

## Supported Versions

The latest `main` branch is the supported version. Releases are tagged (e.g.
`v1.0.0`); older releases are supported on a best-effort basis.

## Security Features

- Bearer-token admin auth with constant-time validation.
- CSRF protection on admin mutations.
- Rate limiting (login, views, global).
- Security headers (CSP, X-Frame-Options, nosniff, HSTS via Cloudflare).
- Production hardening: refuses to boot with default credentials.
- `APP_ENV=production` guard in `cmd/server/main.go`.
