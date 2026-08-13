# Neuralwire Frontend

SvelteKit frontend for the **Neuralwire** AI news website (global audience,
English by default). Renders the public news feed, category/search pages,
article detail pages and the admin moderation panel.

## Features

- Public feed with category filters, search and per-article detail pages
- **Trending / most-read** section on the homepage: a `TrendingNews.svelte`
  ranking of the top-5 most-read articles this week (view counts tracked via
  `POST /api/news/{id}/view` fired when an article page opens, deduplicated
  per browser via a localStorage `nw_viewer_id`)
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

### 2026-08-13 — Simplified category headers (by Frontend agent)

- Removed the debug text `FILTER ACTIVE: INDEX_QUERY = "*"` from the category page.
- Removed the `// CHRONICLE_INDEX` suffix from the CATEGORY header.
- Removed the ` // RESOLVED_IMAGE` debug text suffix from the cover image source label on the article page.
- Cleaned up tech-themed debug and double-slash snake_case labels in the admin panel.
- Removed brackets and programmatic uppercase labels from admin navigation, action buttons, and dropdowns.
- Cleaned up remaining technical uppercase snake_case labels like PUBLIC_SITE, metadata stats, pagination, and editor action buttons.
- Cleaned up save thresholds button label brackets and casing in the admin settings dashboard.
- Added a "Load More" button to the homepage news grid (`frontend/src/routes/+page.svelte`) that displays up to 15 items initially, appends 15 more items reactively per click, and resets when switching categories.
- Implemented inline "Hide Feed" buttons and a fixed floating "Collapse Feed" FAB on the homepage (`frontend/src/routes/+page.svelte`) that resets the visible count to 15 and smoothly scrolls back to the top of the news grid.
- Verified with `npm run check` (0 errors) and formatted with Prettier.

### 2026-08-13 — Trending / most-read articles (by Codex agent)

- New `TrendingNews.svelte` component fetching
  `GET /api/news/trending?window=week&limit=5`, rendered on the homepage
  (ranking 01-05 with view counts, loading/no-data/error states).
- Article detail pages now fire a fire-and-forget
  `POST /api/news/{id}/view` on mount with a per-browser `nw_viewer_id` from
  localStorage.
- `News` interface in `mockData.ts` gained `view_count: number`.

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
