# Changelog

The Go API's own history from before 4.0.0, when it was versioned and released apart from the SPA, in the format of [Keep a Changelog](https://keepachangelog.com/).

This history is closed and takes no new entries. Since 4.0.0 the API and the SPA are one app with one version, whose changes are in [CHANGELOG.md](../CHANGELOG.md) at the repo root.

## [3.28.0] - 2026-11-07

### Added

- Every backend deploy installs and checks a daily database backup kept for 28 days beside the database, and the deploy fails if the backup can't be installed.
- After a backend deploy the live database is pulled into the local development copy, keeping the three previous copies; `-NoDbPull` skips it.
- After a deploy, a smoke check of `/api/version`, the page, its entry script and the plugin index rolls back what the run changed if anything fails; the address it checks can be set with `-SiteUrl`, `DEPLOY_SITE_URL` or `$DeploySiteUrl`.

### Changed

- A rollback now restores the database along with the binary, so anything written while the new release ran is lost with it.
- With the plugin in a deploy, a version not higher than the published one stops the run before the backend and frontend go live.
- Adopted the shared lint rules, CI workflows and repo files.

## [3.27.0] - 2026-10-31

### Added

- A plaque day can cap its artist requests (0 means no limit), staff see how many requests each day holds, and a request past the limit is refused with a 409.
- The festival dashboard's Ema days report how many artist requests are left, so the request option can close when the day is full.
- The artist can post a finished plaque for an open request straight to its day's Discord webhook as `Requested by <character>, made by <artist>`, which marks the request complete (PNG, JPEG, GIF or WebP, up to 10 MB).

## [3.26.0] - 2026-10-30

### Added

- Ema plaque boards: a festival can have one, taking plaques on days that may not overlap, each day with its own Discord webhook, and one plaque per person per day.
- Staff review plaques designed on the site: approving posts the image to the day's webhook, rejecting frees the day, and either way the stored image is deleted.
- The artist has a queue of requested plaques behind its own permission.
- Visitors can submit a plaque (a PNG) or ask the artist for one for the day open now, rate limited and bot-checked.
- The festival dashboard carries the board, its days (never their webhooks), the day open now and the visitor's own plaques.
- `GET /api/google-fonts` lists Google Fonts families, cached for 12 hours.
- Discord posts can carry an image.

### Changed

- The festival list is also readable with the Ema permission, and deleting a festival deletes its plaque board and the plaque images still waiting for review.

## [3.25.0] - 2026-10-29

### Added

- `POST /api/festival-dashboard/{id}` returns one visitor's standing at a festival: their stamp card, the garapon linked to its rally and every raffle filed under the festival with their own entry, from the tokens and name their device holds.
- Each public stamp says which map stall it belongs to, and the public stamp card and garapon responses carry `festival_link` when the rally belongs to a published festival.

### Changed

- Queries and AniList lookups stop when the request that started them is abandoned, and a dropped connection is logged as `canceled=true` instead of reading as a server error.
- Adopted the shared lint rules.

### Removed

- The `-secret` flag, which never did anything; a service that still passes it will not start.

### Fixed

- An upload whose last write never reached the disk, such as on a full disk, is now reported as failed instead of as a success with a file missing or cut short.

## [3.24.1] - 2026-10-16

### Fixed

- Hosted fonts now load in Safari, including on every iPhone, where font requests from an allowlisted Carrd site were refused with a 403.

## [3.24.0] - 2026-09-19

### Added

- A `hide_custom_cards` setting takes the Custom Card request off the public home page while bingo itself stays up; it defaults off.

## [3.23.0] - 2026-09-05

### Added

- Tea rooms record whether they are locked, separate from being lockable: `locked` and `locked_until` are set independently and both appear on the public rooms API.
- A lock with an expiry lifts on its own and tells the admin channel the room's name and number, which the in-game plugin uses to alert its operator.

## [3.22.0] - 2026-08-30

### Added

- A per-stall label color (white by default) and a per-stall selected-outline color for the ring around a stall a visitor taps.
- A per-occupant Booth Page link, shown beside the affiliate's Website and Discord buttons in the stall panel.

### Changed

- A stall must name its affiliate: the form starts on "Select an affiliate..." and a map won't save until every stall has one.
- A stall's title is optional, and an untitled stall is named by its affiliate in the editor, the stall lists and the raffle and stamp-rally pickers.
- The admin map page draws the map exactly as the public page does, and saving a map leaves you on that map.
- The stall info panel no longer repeats the affiliate name or lists the affiliate's owners.

## [3.21.0] - 2026-08-28

### Changed

- A participant is now a character name plus a home world picked from a list everywhere, so stamp rally cards and garapon links no longer hold one typed "Firstname Lastname @ World" string.
- Stamp rally sign-up and lookup match on name and world, so two players sharing a character name on different worlds are two people.

### Security

- A stamp-card link is no longer a spendable drawing token: new sign-ups issue separate tokens for the card and the drawing link, and the drawing link staff issue no longer doubles as a stamp card.
- `POST /api/stamp-lookup` no longer returns the garapon drawing token; it returns the card token and how many draws remain.
- `POST /api/garapon/{token}/draw` is now rate limited.

## [3.20.0] - 2026-08-26

### Changed

- Holders of the Festival Map and Tea Rooms pages can use the shared image picker, the festival map and stamp rally forms can list affiliates, and the pending custom-card preview opens for the Manage Cards page without full admin.
- The backend deploy snapshots the live database before migrating it (keeping the newest three), deploys the API before the SPA, and moves the plugin download and index into place atomically.

### Fixed

- A long announcement now posts instead of failing, and an announcement edited while it posts keeps the edit instead of reposting (`@everyone` and all) once sent. Skipping with an explicit count of 0 is now honored.
- Bulk card generation can no longer hand out duplicate ids, deleting a raffle's winning entry clears the winner, a failed theme delete no longer leaves the site on the default theme, and two admins editing fonts at once can't discard each other's changes.
- The live log tail no longer gets admins disconnected, and a player's live connection rejects an unknown card id, is capped at 500 connections and now receives the notice when its card is deleted.
- The first auto-draw number comes straight away, starting a game whose selected patterns were all deleted now says to reload and pick again, and a draw landing while a board loads is no longer undone.
- A festival map is no longer unpublished by a save with a misspelled or missing status, deleting an affiliate no longer erases the stalls it named, and the public map badges only a stamp rally that is running.
- Saving a stamp rally or festival map from an editor that never loaded the full record no longer wipes its stamps, prizes or stalls, and deleting a garapon drawing link no longer destroys a part-stamped rally card.
- Upgrades no longer risk stopping the server from booting or losing data, and the server now refuses to start against a database newer than itself, naming both versions.
- Signing in with a passkey now works for accounts whose password has ever been changed or reset.

### Security

- Card and pattern changes no longer reach anonymous players' live connections; only admin connections receive them.
- Saving the Settings page as a non-admin no longer overwrites secret settings, such as every club's Discord webhook, with blanks.
- Public forms are bounded: a raffle sign-up above 100 entries per person is rejected, names are capped at 60 characters, wrong stamp passwords are rate limited, and opening the passkey prompt has its own rate limit.
- A reading list item can be reached only through its own list, and a festival map save naming another map's stall is refused with a 400.

## [3.19.0] - 2026-08-24

### Added

- Festival maps: staff can build a floor plan from a title, description, dates and a base image, place stalls on it, and mark it in progress, published or closed, behind the new `festival-map` page permission.
- Public festival map endpoints show published maps only, with each stall's affiliate card, whether it is open now, and a stamp rally badge when an open linked rally places a stamp on it.
- A stall is a pitch that can hold several occupants (affiliate, title, description, offering, times), so a booth that changes hands between days is one shape, and a stall of type "other" can carry its own caption.
- A map can have a shortcode, so it links as `/festival-maps/obon-2026`, and links by numeric id keep working.
- A stamp rally can be linked to a festival map, its stamps then naming the map's stalls.
- A raffle can be filed under a festival and assigned to one of its stalls, and the public map links it only while the raffle is running.

### Changed

- A stamp's stall name on the public card and the stamp log comes from its map stall, then its affiliate, then the venue.

## [3.18.0] - 2026-08-24

### Added

- Stamps in a stamp rally have a type, food or game; existing stamps read as food.
- A rally can complete on a count of each type instead of the whole card.
- The public stamp card shows the completion rule and each stamp's type, and marks a stamp as expired once its stall can never be collected again.

### Changed

- On a rally that completes on counts, expired stamps no longer complete a card; it completes only by meeting its counts.

## [3.17.0] - 2026-08-24

### Added

- A raffle can be closed and reopened without a winner, so a raffle with no entries or a details-only raffle no longer stays on the public list indefinitely.

## [3.16.0] - 2026-08-21

### Added

- Raffle entry modes: single (one flat cost per entry), custom (a price ladder, so 50k/100k/150k charges 300,000 for three entries) and details (nothing priced, entered elsewhere); existing raffles are single.
- Part payments: settling an entry can cover only some of its tickets, and the gil waived is recorded.
- The sign-up confirmation states the amount due, which is lower than the total when part of the entry is already settled.
- A public search for whether you have already entered a raffle (`POST /api/raffles/{id}/lookup`) returns the exact spelling used, matching part of a name and showing no gil figures.
- A raffle can have a "Where to Pay" screenshot, and a Hide Bingo setting controls whether bingo appears on the public home page.
- The per-player entry allowance, and so the price ladder, is capped at 100; going over is refused with a 400.

### Changed

- The collected-gil total prices what each entry actually paid, counting part payments and waivers.
- The winner draw weights settled tickets, so a part-paid entrant gets one chance per ticket paid, and a details-only raffle draws from every entry.
- Adding tickets to an already-settled entry puts it back to unpaid so the new tickets are collected for.

### Fixed

- Settling the same entry twice no longer stacks the waiver and understates what the raffle collected, and the admin raffle list no longer slows down when a raffle has a very large allowance.

### Security

- The tickets paid and gil waived on an entry no longer appear on the public winner of a closed raffle.

## [3.15.0] - 2026-08-11

### Added

- Stamp rally participants can sign themselves up: a rally has a public sign-up option (off by default), and `POST /api/stamp-signup/{id}` issues a card and, when an open garapon is linked, its drawing link.
- Sign-up names are unique per rally ignoring case, a repeat gets a 409 pointing at the lookup page, and sign-up is rate limited and bot-checked.
- `POST /api/stamp-lookup` hands a participant their links back from the exact name they used, and a miss returns an empty 200 so it can't be used to discover who signed up.
- A garapon has a default number of draws (1 unless changed), used when a link is issued without a number.
- `-webhook-dry-run` / `APPSUITE_WEBHOOK_DRY_RUN=1` suppresses every outbound Discord post and `-webhook-override` / `APPSUITE_WEBHOOK_OVERRIDE` redirects them to one test webhook, so a development server on a copy of the live database can't post to production channels.

### Changed

- "Skip next occurrence" on an announcement is now "skip the next N occurrences" (up to 52), so a Friday and Saturday announcement can sit out a whole weekend; setting 0 clears it.

### Fixed

- Saving an announcement no longer cancels a pending skip, and multi-word emoji shortcodes such as `:video_game:` now post as the emoji in announcements, affiliates and tea rooms.

### Security

- Updated the Go toolchain, clearing six standard-library security advisories.

## [3.14.1] - 2026-08-09

### Added

- Every delivered Discord post is logged (`discord post sent`) with the feature, record name, status and duration, so "did that announcement actually go out?" can be answered from the log, and manual sends and publishes name who did them.
- Scheduler start and stop, bingo game start, end and half-time decisions (with the admin), and consumed announcement skips are logged.

## [3.14.0] - 2026-07-24

### Added

- Tea rooms have an optional Room Owner line, returned by every tea-room read including the public cross-origin API.

## [3.13.0] - 2026-07-24

### Added

- Affiliates have an optional subtitle shown under the name, in any script.

## [3.12.0] - 2026-07-21

### Changed

- A panic in a request or background task is now logged and answered with a 500 instead of crashing the server, slow clients are timed out, and a failed start shuts down cleanly.
- WebSocket upgrades are now access-logged, and Discord posts stop when the request is canceled or the server shuts down.

### Fixed

- A draw held for its reveal delay is now dropped if its game ended meanwhile, so a number from one game can't show up in the next, and switching auto-draw on twice at once no longer triggers a redundant draw.
- The announcement scheduler could post `@everyone` twice after a brief database hiccup following a successful post.
- Deleting a garapon player with its stamp card, or a stamp rally with its garapon links, is now all-or-nothing, and a stamp can no longer be updated through another rally's id.

### Security

- The sign-in throttle now keys on the real client IP behind Cloudflare, so one visitor tripping it no longer affects everyone, and starting a passkey sign-in counts toward the login limit.
- Registration no longer reveals which usernames exist: a new name, a taken name and the reserved name all get the same response.
- A personal access token in a `?token=` query is accepted only for the WebSocket upgrade, and a password change or admin reset logs out the account's other sessions and revokes its token.
- Uploaded SVGs and Carrd uploads are checked more strictly, so scripts can't ride in on a crafted SVG or a renamed `.png`, `.mp3` or `.mp4`.
- A raffle entry can be marked paid or deleted only through its own raffle, and raffle staff reads now require the same `teahouse-raffles` permission as writes.
- The AniList endpoint setting can't be pointed at an internal service, font access tokens can't be forged when the server's random source is unavailable, and token links in the logs are hashed whatever their letter casing.

## [3.11.0] - 2026-07-20

### Added

- Auto-draw: a game can draw numbers by itself, the first the moment auto is switched on and then one every interval in seconds; it is off by default, including after a restart, and a manual draw switches it off.
- Starting a game accepts auto and an interval, both adjustable live, game presets gain an auto-call setting and an interval, and the state is pushed to every admin console.

### Changed

- Half-time is detected by the server, so any draw, manual or automatic, crossing the midpoint prompts every admin console and pauses a running auto-draw. Answering yes alerts players and leaves auto paused, answering no resumes it, and auto switches off the moment a new winner is recognized.

## [3.10.0] - 2026-07-19

### Added

- Themes can be public or private (existing themes stay private); the theme picker lists public ones, and a private theme answers 404.
- Custom card requests: the public `POST /api/cards/request` takes a hand-built board and a chosen 6-character ID (rate limited and bot-checked) and stores it as pending until approved, rejecting a taken ID or duplicate board.
- Cards have a Protected flag and a world; approving a pending custom card makes it approved and protected, and Delete All spares protected cards.
- A custom card cost setting (gil) is shown on the public request page.

## [3.9.0] - 2026-07-16

### Added

- The public garapon page now includes the token of the stamp rally card issued with the drawing link, so it can link to the participant's stamp card.

## [3.8.0] - 2026-07-12

### Added

- Stamp rallies have a "Where to Redeem" image, shown on the admin and public stamp card pages.

## [3.7.1] - 2026-07-12

### Changed

- Affiliate embed open times show as Discord short time (4:00 PM) instead of long time with seconds, still in each viewer's own time zone.

## [3.7.0] - 2026-07-12

### Added

- Affiliates have an embed color, a Discord link, a Carrd link and a drag order that editing keeps.
- Affiliates share one admin-set Discord webhook, and an affiliate can be posted to Discord as an embed with its color, name, details, logo, screenshot, location, opening hours (in each viewer's own time zone) and links.

## [3.6.0] - 2026-07-11

### Added

- The "It's Yoever" bingo reaction: `POST /api/game/yoever` shows a reaction with the player's name and a count to every connected client, refused when no game is active, when it is switched off, or (with a 429) while the card is on cooldown.
- Admins can switch the reaction on or off live, and it resets at the start of each game.
- A `yoever_cooldown_seconds` setting sets the per-card cooldown (default 180, 0 disables it).

### Fixed

- Saving settings no longer rejects a Google Fonts API key as a Discord webhook, which had stopped any admin with that key set from saving any setting.

### Security

- Updated the Go toolchain to fix a TLS Encrypted Client Hello privacy leak.

## [3.5.0] - 2026-07-10

### Added

- Tea Rooms: bookable rooms with a name, subtitle, unique room number, per-half-hour gil cost, hashtags, description, seasonal, open, lockable and discounted flags, an image and an embed color, managed by admins with drag ordering behind the new `teahouse-tea-rooms` page permission.
- A room can be posted to Discord as an embed with its cost (halved when discounted), room number and Open/Closed status, to one shared admin-set webhook.
- A public rooms API (`GET /api/tea-rooms/public` and `/public/{number}`) needs no sign-in and can be read from any site, so an external Carrd site can show a room's live availability and pricing.

## [3.4.0] - 2026-07-07

### Added

- Request logs identify who made the request (an `auth` field of `session`, `token`, `bot` or `anon`, plus the account's `user` name) and name Cloudflare-verified bots.

### Changed

- The service runs as a dedicated `senpan` user instead of root.

### Fixed

- The image category list no longer loses custom categories, and categories can't be created or renamed onto folders owned by other features.
- The default app title is "Senpan App Suite" (it was "Nifty App Suite"), so an unconfigured instance no longer shows the wrong name.

### Security

- The live server-log tail on the admin WebSocket is now admin-only; it had reached any signed-in account and any plugin token.
- Capability tokens (garapon and stamp card draw links, font kit tokens, the `token` query parameter and the Referer) are replaced in request logs with a short non-reversible hash.
- Font family names, the header font setting and theme flourish paths are validated before they are stored, rejecting characters that could break CSS.
- The client IP in the logs honors `CF-Connecting-IP` and `X-Forwarded-For` only from the local reverse proxy, so a client can't forge it.

## [3.3.0] - 2026-07-05

### Added

- Structured JSON logs go to stdout and, with `-log-file` (default `/var/log/senpan/senpan.log`), to a rotating file; `-log-file=""` keeps development to stdout.
- An admin log viewer: `GET /api/logs` tails the file newest-first with level, text and limit filters, and every log line is pushed live to admin WebSocket clients.
- A DEBUG toggle that needs no restart (`POST /api/logs/level`) adds a richer `request detail` line per request, and the level reverts to INFO on restart.

### Changed

- Book club covers post as the large full-width Discord embed image instead of the small thumbnail when a reading list is published.

### Fixed

- Request logs now record the real client address instead of the Apache proxy's.
- A failed book-club AniList lookup now returns a 424 carrying AniList's actual message instead of a 502 that Cloudflare replaced with its own error page.

## [3.1.0] - 2026-07-03

### Changed

- There are no permanent image categories any more: the ten built-in ones can be renamed and deleted like any other.
- Anyone holding a page permission that uses images can browse the image categories and images for the shared picker; upload, delete and category management still need `system-images`.

## [3.0.0] - 2026-07-02

### Added

- Automatic WOFF2 conversion: a font with no uploaded WOFF2 gets one converted and served by default (selectable per font), and existing fonts are converted at startup.

### Changed

- Breaking: uploaded fonts are no longer served as static files, so legacy direct font URLs stop working; external sites use the generated stylesheet at `GET /api/fonts/pub/kit.css`, and font files are served behind a token that rotates every 7-14 days.
- Font requests are checked per font: a cross-origin site needs an origin on that font's allowlist, and `kit.css` lists only the fonts the requesting site may use. The `uploaded_fonts` list in `GET /api/settings` changed from file names to `{name, family, token}` objects.
- Breaking: fonts are grouped into families by shared base name, `GET /api/fonts` returns the grouped shape, and admins can rename a file, edit a font's CSS name, served variant and allowed origins, or delete a whole font.

## [2.2.0] - 2026-07-02

### Added

- Passkeys: an account can register, list and delete passkeys, and sign in with one without a username.
- Public raffle entry is bot-checked when Turnstile is configured, so flooded entries can't skew the weighted winner pick.

## [2.1.0] - 2026-07-02

### Added

- A number can no longer be drawn twice in one game; the database now enforces it as a backstop.

### Fixed

- A server panic when a WebSocket client disconnected, a 500 on a stamp card page after one of its stamps was removed, and a crash of the admin page when editing an announcement with no buttons.
- Shutting down mid-sweep could let the announcement scheduler post again on the next start.
- A failure while starting a game could leave a half-initialized active game, and uploads could leave temporary files behind after a large batch.

### Security

- Uploaded SVGs are sanitized before they are stored (scripts, `<foreignObject>`, event handlers and external references are dropped), and font uploads reject hidden-file names.
- The `anilist_api_url` setting is limited to hosts under `anilist.co`, and the Google Fonts API key is kept out of the public `GET /api/settings` response.
- The public raffle-entry endpoint is rate limited per IP, and a raffle that is not yet open or past its window can no longer be read by non-admins by guessing its id.
- Cross-origin state-changing requests that ride a session cookie are rejected; plugin (Bearer-token) and cookie-less requests are exempt.
- The admin WebSocket is re-authorized periodically, so an account deactivated or deleted mid-session loses its admin socket, and an enforcing Content-Security-Policy is in place.

## [2.0.0] - 2026-06-30

### Changed

- Breaking: the HTTP API moved from action dispatchers to REST (`POST` creates, `PUT` replaces, `PATCH` updates part of a record, `DELETE` removes, commands are `POST /api/<resource>/{id}/<verb>`), needing the 2.0.0 SPA; login, logout and register keep their small action bodies.
- Reading lists moved from `/api/reading-lists?club=...` to `/api/book-clubs/{club}/reading-lists[...]`, and an id from another club answers 404, so one club's permission can't reach another club's list.

## [1.7.0] - 2026-06-30

### Added

- An OpenAPI 3 description of every endpoint is served at `GET /api/openapi.yaml`, with a hosted reference at `GET /api/docs`; both are public.

## [1.6.0] - 2026-06-30

### Changed

- The book club cover upload keeps the uploaded file's (sanitized) name and overwrites a same-named file, and deleting a reading-list item or list removes its cover only once no other item uses the same file.

### Fixed

- Simultaneous requests could exceed limits: two sign-ups for the same character and world could both pass the entry limit, and the public garapon draw cap could misfire.

## [1.5.0] - 2026-06-29

### Added

- Personal access tokens for external API clients such as the FFXIV Dalamud plugin: a request sends `Authorization: Bearer <token>`, acts as the owning account and meets the same per-page permission checks.
- `GET` and `POST /api/account/token` let an account see its token's details, generate (replacing) or revoke it; there is one token per account, shown in full only when generated.

## [1.4.0] - 2026-06-29

### Added

- A garapon can be linked to an open stamp rally, and creating a drawing link on it also issues a stamp rally card with the same token (`stamp_card_token`).
- A stamp rally is open or closed: a closed rally is read-only, and a card with collected stamps can be deleted only when the rally is closed.
- The stamp rally list returns each rally's stamp count and active stamp count.

### Changed

- The stamp log is kept when a card or stamp is deleted, since each entry records the participant and stall names; deleting a rally still removes its whole log.

## [1.3.0] - 2026-06-29

### Added

- Stamp Rally: admins create events with stamps and prizes placed by percentage, pause and resume a stamp, issue tokenized participant cards and see an event-wide collection log, behind the new `festival-stamp-rally` page permission.
- The public participant flow: `GET /api/stamp-card/{token}` shows the card (prizes hidden until it completes) and `POST /api/stamp-card/{token}/stamp` collects a stamp by password, updating the admin log live. A card completes once every stamp is collected or expired.
- Three new image categories for stamp cards, stamps and prizes.

## [1.2.0] - 2026-06-29

### Added

- Affiliates: admins can manage partner establishments (`GET` and `POST /api/affiliates`) with owners and opening hours behind the new `teahouse-affiliates` page permission, with two new image categories, Affiliate Logos and Affiliate Images, for the logo and screenshot pickers.

## [1.1.1] - 2026-06-28

### Added

- Cloudflare Turnstile is also checked on registration to stop mass sign-ups, enforced only when a secret is set, like login.

## [1.1.0] - 2026-06-28

### Added

- Cloudflare Turnstile on login: when `APPSUITE_TURNSTILE_SECRET` (flag `-turnstile-secret`) is set, `POST /api/auth` requires a valid Turnstile token and refuses on any failure; it is skipped when no secret is set.
- `GET /api/config` is a public endpoint exposing the non-secret Turnstile site key (flag `-turnstile-sitekey` or `APPSUITE_TURNSTILE_SITEKEY`) for the login page.

## [1.0.1] - 2026-06-28

### Fixed

- Scheduled announcements could post more than once: one overdue by more than a period re-posted on every scheduler tick, and a webhook call that timed out was retried even though Discord may have received it.

## [1.0.0] - 2026-06-28

### Added

- `GET /api/version` is a public endpoint returning the backend's version.
- Token-based theming: themes are stored as a structured set of design tokens, and the stylesheet is generated server-side from an allowlist of tokens.
- Live admin updates: after an admin change, open admin pages are told to refresh, including garapon draws and raffle entries.
- Garapon, the festival lottery drum, with admin management and a tokenized public draw.

[3.28.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.27.0...Backend-v3.28.0
[3.27.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.26.0...Backend-v3.27.0
[3.26.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.25.0...Backend-v3.26.0
[3.25.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.24.1...Backend-v3.25.0
[3.24.1]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.24.0...Backend-v3.24.1
[3.24.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.23.0...Backend-v3.24.0
[3.23.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.22.0...Backend-v3.23.0
[3.22.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.21.0...Backend-v3.22.0
[3.21.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.20.0...Backend-v3.21.0
[3.20.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.19.0...Backend-v3.20.0
[3.19.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.18.0...Backend-v3.19.0
[3.18.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.17.0...Backend-v3.18.0
[3.17.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.16.0...Backend-v3.17.0
[3.16.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.15.0...Backend-v3.16.0
[3.15.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.14.1...Backend-v3.15.0
[3.14.1]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.14.0...Backend-v3.14.1
[3.14.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.13.0...Backend-v3.14.0
[3.13.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.12.0...Backend-v3.13.0
[3.12.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.11.0...Backend-v3.12.0
[3.11.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.10.0...Backend-v3.11.0
[3.10.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.9.0...Backend-v3.10.0
[3.9.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.8.0...Backend-v3.9.0
[3.8.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.7.1...Backend-v3.8.0
[3.7.1]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.7.0...Backend-v3.7.1
[3.7.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.6.0...Backend-v3.7.0
[3.6.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.5.0...Backend-v3.6.0
[3.5.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.4.0...Backend-v3.5.0
[3.4.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.3.0...Backend-v3.4.0
[3.3.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.1.0...Backend-v3.3.0
[3.1.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v3.0.0...Backend-v3.1.0
[3.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v2.2.0...Backend-v3.0.0
[2.2.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v2.1.0...Backend-v2.2.0
[2.1.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v2.0.0...Backend-v2.1.0
[2.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v1.7.0...Backend-v2.0.0
[1.7.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v1.6.0...Backend-v1.7.0
[1.6.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v1.5.0...Backend-v1.6.0
[1.5.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v1.4.0...Backend-v1.5.0
[1.4.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v1.3.0...Backend-v1.4.0
[1.3.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v1.2.0...Backend-v1.3.0
[1.2.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v1.1.1...Backend-v1.2.0
[1.1.1]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v1.1.0...Backend-v1.1.1
[1.1.0]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v1.0.1...Backend-v1.1.0
[1.0.1]: https://github.com/jwill89/app-senpan-suite/compare/Backend-v1.0.0...Backend-v1.0.1
[1.0.0]: https://github.com/jwill89/app-senpan-suite/releases/tag/Backend-v1.0.0
