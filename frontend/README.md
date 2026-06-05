# Frontend (Vite + Vue 3 + TypeScript)

Incremental migration of the legacy CDN SPA (`/index.html` + `/assets/js/app.js`)
into a built, component-based, typed application. This currently contains a
**proof-of-concept**: the player view migrated to a real `.vue` component tree.
The legacy app is untouched and remains the live frontend until cutover.

## Prerequisites

- Node 18+ (developed on Node 22)
- The Go backend running on `:8080` for live data:
  ```powershell
  cd ..\src; go run . -addr :8080 -db ..\data\bingo.sqlite
  ```

## Commands

```powershell
npm install        # once
npm run dev        # Vite dev server at http://localhost:5173 (proxies /api → :8080)
npm run typecheck  # vue-tsc, no emit
npm run build      # type-check + emit production bundle to dist/
npm run preview    # serve the built dist/ locally
```

## Layout

```
src/
  main.ts                  app entry
  App.vue                  shell (join flow → PlayerView; future: Vue Router)
  api/client.ts            typed fetch wrapper (replaces app.js `api()`)
  types/models.ts          TS mirror of the Go internal/model package
  composables/
    useGameSocket.ts       WebSocket connect / reconnect / keepalive
  components/
    BingoBoard.vue         presentational 5×5 board
  views/
    PlayerView.vue         migrated player experience (the POC slice)
```

## How dev mode talks to the backend

`vite.config.ts` proxies `/api` (HTTP **and** WebSocket) to `http://localhost:8080`,
so the frontend behaves exactly as it does in production, where Apache serves the
SPA and reverse-proxies `/api` to the Go binary. No CORS, no env juggling.

## Production cutover (later, not yet done)

`npm run build` emits `dist/` (`index.html` + hashed `assets/`). To go live,
copy `dist/*` into the Apache web root that currently serves `index.html` +
`assets/`. The Go backend is unchanged. Until then, the legacy SPA stays live.

## Migration notes / open items

- **Type generation:** `types/models.ts` is hand-maintained. Consider generating
  it from the Go `internal/model` package (e.g. `tygo`) so they can't drift.
- **Themes coupling:** the custom-CSS themes feature targets specific class names
  in the legacy markup. Porting each view's markup must preserve (or deliberately
  migrate) those class names, or existing user themes will break.
- **Pin third-party libs:** the legacy app loads `marked` from a CDN with no
  version pin. When the markdown-rendering views are migrated, add `marked`
  (and a sanitizer such as DOMPurify) as pinned dependencies here.
- **Next views to migrate:** home → raffles (public, low coupling) are good
  follow-ups; the admin dashboard is the largest and should come last.
