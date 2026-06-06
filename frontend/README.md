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
  main.ts                  app entry (installs the router)
  App.vue                  shell: header + <RouterView>
  router/index.ts          routes (home, /play/:cardId, /raffles, /raffles/:id)
  api/client.ts            typed fetch wrapper (replaces app.js `api()`)
  types/
    generated.ts           GENERATED from Go internal/model — do not edit
    models.ts              re-exports generated + API envelopes & WS messages
  lib/
    markdown.ts            marked + DOMPurify (sanitized markdown → v-html)
    assets.ts              prize-image URL resolver
  composables/
    useGameSocket.ts       WebSocket connect / reconnect / keepalive
    useAuth.ts             admin session state + router-guard helper
    useToast.ts            toast notifications (singleton)
  components/
    BingoBoard.vue         presentational 5×5 board (+ winner highlight)
    PatternPreview.vue     small read-only 5×5 win-pattern grid
    ToastHost.vue          renders active toasts (mounted in App.vue)
  views/
    HomeView.vue           landing: bingo join + raffles card
    PlayerView.vue         player experience (board, stamps, live draws)
    RafflesListView.vue    public open/closed raffle listing
    RaffleDetailView.vue   public raffle detail + sign-up form
    admin/
      AdminLoginView.vue   password login
      AdminLayout.vue      tab bar + nested <RouterView> (guarded)
      AdminGameView.vue    start / draw / winners / verify / end / halftime
      AdminCardsView.vue   generate / search / preview / edit / delete cards
      AdminWinnersLogView.vue  paginated, sortable winners log
      AdminSettingsView.vue    app settings editor
```

Non-home views are lazy-loaded (each is its own chunk). Migrated so far:
**player**, **public raffles**, and the **admin** section's **Game**, **Cards**,
**Winners Log**, and **Settings** tabs (cookie auth + route guard).

Still on the legacy app (next up): admin **Patterns** (drag-drop grid editor),
**Themes** (CodeMirror), and **raffle admin** (entries, winner pick, open/close).

## How dev mode talks to the backend

`vite.config.ts` proxies `/api` (HTTP **and** WebSocket) to `http://localhost:8080`,
so the frontend behaves exactly as it does in production, where Apache serves the
SPA and reverse-proxies `/api` to the Go binary. No CORS, no env juggling.

The proxy uses `changeOrigin: false` deliberately: the backend's WebSocket
upgrade enforces a same-origin check (Origin host must equal Host), so the proxy
must preserve the original Host (`localhost:5173`) to match the browser's Origin.
In production this maps to Apache's `ProxyPreserveHost On`.

## Production cutover (later, not yet done)

`npm run build` emits `dist/` (`index.html` + hashed `assets/`). To go live,
copy `dist/*` into the Apache web root that currently serves `index.html` +
`assets/`. The Go backend is unchanged. Until then, the legacy SPA stays live.

The router uses HTML5 history mode (clean URLs), so deep links like `/raffles/1`
must fall back to `index.html`. Add to the SPA vhost (keep `/api` proxied to Go):

```apache
<Directory /var/www/www.yoursite.com>
    FallbackResource /index.html
</Directory>
```

Uploaded prize images keep their existing path (`assets/images/raffles/...`) and
are still served by Apache from the web root.

## Type generation

`src/types/generated.ts` is generated from the Go `internal/model` package with
[tygo](https://github.com/gzuidhof/tygo), so the domain types can't drift from
the backend. Regenerate after changing a model struct:

```powershell
go install github.com/gzuidhof/tygo@latest   # once; ensure $(go env GOPATH)/bin is on PATH
cd ..\src; go generate ./...                 # emits frontend/src/types/generated.ts
```

Config lives in `src/tygo.yaml`. The API response envelopes and WebSocket
message types have no Go struct counterpart (the handlers build them inline),
so they stay hand-written in `src/types/models.ts`, which re-exports the
generated types alongside them.

## Migration notes / open items

- **Themes coupling:** the custom-CSS themes feature targets specific class names
  in the legacy markup. Porting each view's markup must preserve (or deliberately
  migrate) those class names, or existing user themes will break.
- **Markdown is now sanitized:** `marked` + `DOMPurify` are pinned deps and all
  admin-authored markdown is sanitized in `lib/markdown.ts` before `v-html`,
  closing the stored-XSS hardening gap the legacy CDN `marked` left open.
- **Next views to migrate:** the admin dashboard (game, cards, patterns, winners
  log, raffle admin, themes) — the largest surface — should come last. It needs
  session-aware fetches (already cookie-based) and the themes/CodeMirror editor.
