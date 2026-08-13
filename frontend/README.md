# Neuralwire Frontend

SvelteKit frontend for the **Neuralwire** AI news website (global audience,
English by default). Renders the public news feed, category/search pages,
article detail pages and the admin moderation panel.

## Features

- Public feed with category filters, search and per-article detail pages
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
