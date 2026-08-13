# Neuralwire Frontend

SvelteKit frontend for the **Neuralwire** AI news website (global audience,
English by default). Renders the public news feed, category/search pages,
article detail pages and the admin moderation panel.

## Features

- Public feed with category filters, search and per-article detail pages
- **Responsive news grid**: 1 column (mobile) → 2 (tablet) → 3 (desktop) → 5
  (very wide / 2xl) on the home, category and search pages
- **Curator model article pages**: each story shows an AI digest summary plus
  a "READ FULL STORY" link to the original source (full text is never
  republished)
- **Admin panel** (`/admin`):
  - Dashboard with draft/published/rejected counts, manual fetch trigger and
    live fetch progress bar that survives page refreshes, plus
    `[CANCEL_FETCH]` to abort a running cycle
  - Scoring thresholds editor (`// VALUE_SCORE_THRESHOLDS`)
  - Draft review with publish/reject/delete workflows
  - Draft list shows the advisory value score badge
    (`HIGH`/`MEDIUM`/`LOW`), sub-score breakdown (AI impact/novelty/quality +
    heuristic), confidence, method and AI reason, with filtering by category
    and by value label

## Developing

Once you've created a project and installed dependencies with `npm install` (or `pnpm install` or `yarn`), start a development server:

```sh
npm run dev

# or start the server and open the app in a new browser tab
npm run dev -- --open
```

## Building

To create a production version of your app:

```sh
npm run build
```

You can preview the production build with `npm run preview`.

> To deploy your app, you may need to install an [adapter](https://svelte.dev/docs/kit/adapters) for your target environment.

## Recent changes

### 2026-08-13 — Responsive 5-column grid (by Codex agent)

- Home (`+page.svelte`), category (`category/[slug]/+page.svelte`) and search
  (`search/+page.svelte`) news grids now render **5 columns on very wide
  screens** via `2xl:grid-cols-5`. Mobile stays 1 column, tablet 2, desktop 3.
- Verified with `npm run check` (0 errors).

### Known open issues

- 4 static images are missing and currently 404 in dev:
  `/images/digital_ghost.jpg`, `/images/neuromancer_implants.jpg`,
  `/images/digital_bots.jpg`, `/images/code_matrix.jpg` — decide whether to
  restore the files or remove the references.
- Pre-existing a11y warning (unrelated):
  `admin/preview/[id]/+page.svelte:456` — the `COVER IMAGE URL` label is not
  associated with its input (add `for` on the label + `id` on the input).
