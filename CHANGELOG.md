# Changelog

All notable changes to the **Senpan App Suite** app are recorded here, in the format of [Keep a Changelog](https://keepachangelog.com/), and the app follows [Semantic Versioning](https://semver.org/).

The app is the Vue SPA and the Go API, built into one binary with one version. The plugin (SenpanCompanion, a Dalamud/FFXIV plugin) keeps its own version, as Dalamud needs, and its own changelog, [plugins/CHANGELOG.md](plugins/CHANGELOG.md).

Before 4.0.0 this version line was the SPA's own, released apart from the API, whose history is in [backend/CHANGELOG.md](backend/CHANGELOG.md).

The app's major version rises for a breaking change to the JSON API or the live event streams the plugin relies on, its minor version for backward-compatible additions, and its patch version for fixes.

## [Unreleased]

## [5.6.0] - 2026-11-25

### Changed

- The festival dashboard gives each section the page's full width, one under another, and each folds away from its heading while still showing its tally. The stamp card and the garapon drum sit beside their stall and prize lists, and the raffles sit side by side.
- The festival dashboard's header puts the days first, then a folded description set as readable text, then who is visiting.
- The festival map fills its section on the dashboard.

### Fixed

- A stall's name on a festival map no longer breaks in the middle of a word or loses lines off its edge: a name that doesn't fit its stall is drawn smaller until it does, on the public map and in the admin editor.
- The stamp-rally mark on a festival map stall no longer covers the end of the stall's name.
- A festival map stall's details no longer repeat its name, as in "Atelier Yao - Atelier Yao", when the stall is titled after its affiliate, has no affiliate, or its title starts with the affiliate's name.
- In the admin map editor, a stall with no affiliate shows its title, as the public map does, instead of no name.

## [5.5.0] - 2026-11-16

Needs one-time steps at deploy (docs/ops.md, Upgrading to 5.5.0).

### Changed

- The admin pages' text editor (announcements, raffles, book clubs, the festival's pages, presets, tea rooms and the rest) is a new one, with the same top toolbar, emoji button and formatting, and a row for a link's address. It saves line breaks, Discord timestamps, mentions and emoji as typed. Saving a field written with the old editor may tidy its Markdown: `*` bullets become `-`, `__bold__` becomes `**bold**`, and trailing-space line breaks become plain newlines; opening a form without typing changes nothing.
- Every font is served by the site itself: the body font, and any Google font picked as the header font or in the ema plaque designer, which the site fetches once and keeps, so visitors' browsers no longer contact Google.
- The admin's changelog shows the app's whole history, its releases before 4.0.0 included, and the plugin's in its own view.
- The header-font picker on the Settings page reads the list of Google Fonts through the site, so the Google Fonts key never reaches the browser.
- The Carrd Upload page shows its previews from this site rather than from carrd.senpan.cafe; copied links still give the public address.
- The deploy needs an `AppPort` setting and no longer assumes a port.
- The deploy archives the uploads (images, fonts, Carrd files and sounds) daily on the host, keeping seven days, and copies them to the dev machine with the database; the service is capped at 384 MB of memory.
- The server writes a log file only when `-log-file` names one; the service's unit names the same file as before.
- A new database starts with no account: the first admin is made on the host with `app-suite admin`, which is also the way back in when nobody can sign in. The name `admin` is no longer reserved or protected.

### Added

- System -> Settings lists the sites that may show a festival map in a frame. It starts with the sites allowed to read the tea rooms.
- Owner accounts, named in the server's settings (`APPSUITE_OWNERS`): nobody deactivates, demotes, renames or deletes an owner, or sets its password from the Users page, and the Users page marks them.
- An admin can't deactivate, demote or delete its own account, so the last admin can't lock everyone out.

### Fixed

- A festival map's embed shows again on the sites that list allows; every page had refused to appear in another site's frame.
- Screen readers name each admin form field by its label, name every table's actions column, and no longer meet drag handles they couldn't use on the festival map and placement editors.
- Markdown tables keep their column alignment.

### Security

- Every page is served with a strict Content-Security-Policy: a fresh nonce on each page, no inline styles or scripts, and only the hosts the site uses. A setting turns it to report-only without a deploy.
- The site sends `Referrer-Policy: same-origin` on every answer, and HSTS from the web server for all three names.
- Visitors' addresses are restored by the web server from Cloudflare, so the rate limits and the log count each visitor, and an IPv6 visitor's whole /64 counts as one.
- An uploaded SVG's inline styles are kept as presentation attributes, so flourishes look the same under the policy.
- An account named `admin` still using the password `admin` has its password cleared on upgrade; a linked Discord account or passkey still signs it in, and the host command sets a new password.

## [5.4.0] - 2026-11-13

Needs a one-time step in the Discord Developer Portal (docs/ops.md, Upgrading to 5.4.0).

### Added

- Sign in with Discord, from the sign-in page, for an account linked to a Discord account. Nobody signs up by signing in with Discord.
- On the Users page, an admin makes a one-time Discord sign-up link: whoever opens it gets an active account named after their Discord username, with no pages until an admin grants them. The link works for a week.
- On the Users page, an admin makes a one-time link that links a Discord account to an existing account and signs its owner in, or unlinks one. The link works for a day; only the admin account links its own.
- The Users page shows the Discord account each account signs in with, and an admin can rename an account.
- An account made with Discord sets its first password, or adds a passkey, without being asked for a current password. Unlinking Discord or removing a passkey is refused when it would leave the account no way to sign in.

## [5.3.0] - 2026-11-11

Needs a one-time step at deploy (docs/ops.md, Upgrading to 5.3.0).

### Changed

- The deploy installs the service's systemd unit and the Apache vhost from templates in the repo, so the host runs what the repo says, and disables any other enabled Apache site serving the vhost's names. The unit is no longer edited on the host.
- The service reads its secrets, the Turnstile secret and the session secret, from a root-only env file instead of `Environment=` lines in its unit. The deploy stops before building anything if that file is missing or readable by others, or if the unit still passes `-webroot`.
- The deploy needs a `ServiceUser` setting (the account the service runs as) and takes an optional `AppPort`, `EnvFile` and `HostKey`; `RemoteDbPath` is gone.
- The app listens on the loopback address only.

### Security

- The server is built with Go 1.27.4, which fixes a way it could crash while reading a request.

## [5.2.0] - 2026-11-10

Needs a one-time step at deploy (docs/ops.md, Upgrading to 5.2.0).

### Changed

- The app serves its own pictures (`/images/`), Carrd sites (`/carrd/`) and plugin repository (`/plugin/`) from a data folder beside its database; every address stays the same, `fonts.senpan.cafe/kit.css` and its font links included.
- Pictures and Carrd files are cached for a day, so a replaced picture can show the old one until then unless it is purged at Cloudflare.
- Fonts are reachable only through `kit.css` and its tokenized links, gated by each font's allowed sites, never as plain files.
- The `-webroot` flag is now `-data`. A unit still passing `-webroot` won't start.
- The deploy installs the Apache vhost itself and needs a `CertName` setting (the certificate the vhost uses) in place of a web root setting.

### Fixed

- The game sounds play again. Since 4.0.0 a release carried none of the four sound effects, so the "game" sound mode and the Yoever reaction were silent; the app now serves `/sounds/*.mp3` from the data folder's `sounds/`.

## [5.1.0] - 2026-11-10

### Changed

- Ema Requests is no longer a menu link: a plaque board's artist requests are a table on its own page, with Post Plaque, Complete and Reopen on each row. The Ema Plaques link counts plaques awaiting review and requests awaiting the artist together.
- `festival-ema` is now the one permission for all of Ema Plaques. An account that held `festival-ema-requests` holds it in its place, and so sees the boards' setup, webhooks included.
- `GET /api/v1/ema-requests` is gone; a board's requests are among its plaques (`GET /api/v1/ema-boards/{id}/plaques`, `kind` `artist`).

## [5.0.1] - 2026-11-10

### Changed

- No user-visible changes.

## [5.0.0] - 2026-11-10

### Changed

- The whole API moves under `/api/v1` with camelCase fields everywhere, described by an OpenAPI document. It needs plugin 4.0.0.0; an older plugin can't reach this server.
- Every error is problem details (`application/problem+json`) with a readable message and, for a failed validation, each field named. A rate-limited request says when to try again, and a failed Discord post answers 429, 503 or 500 instead of 502.
- The live feed is server-sent events (`GET /api/v1/events` for staff, `GET /api/v1/board/{id}/events` for a board) in place of the WebSocket. Staff hear only what their pages cover, and a number still inside its draw delay is hidden from anyone without the Game page.
- Signing in and out and signing up are under `/api/v1/auth/`. A personal access token can't make or revoke tokens, change the password or touch passkeys, and adding a passkey asks for the account's password.
- Saving settings is refused with a 409 when the copy saved from is older than what is stored, so a page left open can't undo a later save. The Google Fonts key is an admin-only integration, and the Discord webhooks of affiliates, tea rooms and each book club are set on their own pages.
- The public tea-room API at `/api/v1/tea-rooms/public` answers with camelCase fields (`{ "teaRooms": [...] }`) and, cross-origin, only to the sites on the new "Allowed sites for the public tea-room API" setting (System -> Settings). The Carrd pages' scripts need the new fields and address.
- Image uploads are checked on their bytes: JPEG and PNG are re-encoded, which drops metadata such as EXIF and GPS; GIF and WebP are kept as sent; an image over 40 megapixels or 16384 pixels a side is refused.
- A raffle sign-up is refused when the raffle is closed, outside its window, details-only or out of allowance, and an allowance over 100 is refused. Picking another winner or confirming one names the winner it means, so two consoles can't pick over each other.
- Other actions refuse with a reason: drawing from a closed or empty garapon, stamping a stamp already collected or at a closed or paused stall, publishing a map with no base image, deleting an announcement type or role still in use, and publishing an empty reading list.

### Removed

- The WebSocket (`/api/ws`), the hand-written API spec at `/api/docs` and the old `/api/fonts/pub/` font address; `fonts.senpan.cafe`'s own addresses are unchanged.

### Fixed

- Half-time reaches every console: answering it closes the prompt on all of them and in the plugin, and a console that opens mid-game gets the pending prompt.
- Ending a game from an out-of-date view no longer ends a newer game with the old one's winners, and starting a game while one is live is refused unless the request says `replace`.
- Admin pages stay in step when another admin changes patterns, categories, presets, the winners log, a map, the draw delay or an announcement, and a stamp card's completion reaches the Stamp Rally page. The card preview saves only the fields its editor changed.
- Renaming an image category no longer moves its folder, which broke every stored picture path under it.
- A right stamp password no longer clears the wrong guesses before it; ten wrong passwords in ten minutes from one address hold back even a right one.

## [4.1.0] - 2026-11-09

### Changed

- Sign-ins are kept in the database, so a deploy or restart no longer signs everyone out.
- Every time the API sends is RFC 3339 in UTC to the second (`2026-11-09T21:30:00Z`), and a raffle or stamp-rally time that isn't in that form is refused with a 400.
- A database that 4.0.0 has not yet brought up to date is refused at start.

### Fixed

- The admin's access-token and passkey dialogs show their times correctly.

## [4.0.0] - 2026-11-09

### Changed

- The frontend (3.32.0) and the backend (3.28.0) give way to one app version, 4.0.0, for the site and the API together. The plugin keeps its own version.
- The Go binary serves the site itself, with the same cache rules and security headers Apache set; Apache serves only the web root's own folders (`images/`, `carrd/`, `plugin/`) and proxies the rest to the app.
- `GET /healthz` answers with the version running and the schema's (503 when the database doesn't answer), for the deploy and an uptime check.
- The admin footer shows the app's version and the plugin's, and says when the server runs a different version from the open tab.
- The deploy ships the newest release, skips one the host already runs (`-Force` deploys it again), and takes `-Target app` or `-Target plugin`; `task deploy:frontend` and `-SkipBuild` are gone.

## [3.32.0] - 2026-11-07

### Changed

- Adopted the shared lint rules and test settings.

### Fixed

- Every page now has a main landmark and a level-one heading for screen readers; the visible page titles are unchanged.
- The footer's links stand out from its text by weight as well as color, and the Admin Portal button on the home page is easier to read.
- The player's volume control and the stamp lookup's world picker are now labeled for screen readers.

## [3.31.1] - 2026-11-01

### Changed

- Building the app now needs Node 26.

## [3.31.0] - 2026-10-31

### Added

- Each day on a plaque board has an Artist Requests limit beside its webhook (empty for no limit), with the number of requests so far shown next to it.
- On a limited day, the festival dashboard's Ask the Artist option shows how many requests are left and is disabled once the day is full; designing on the site is unaffected.
- Post Plaque in Ema Requests: the artist uploads the finished plaque (PNG, JPEG, GIF or WebP, up to 10 MB), looks it over and posts it to that day's Discord channel, which marks the request complete.

### Fixed

- Sending a plaque from an out-of-date page now refreshes the dashboard and shows why it can't be sent.

## [3.30.0] - 2026-10-30

### Added

- A festival with a plaque board gets an Ema section on its dashboard, offering one plaque per person per festival day and showing their plaques and where each stands.
- Design it yourself: save the blank plaque, decorate it in any image editor and post it in the festival's Discord, following the board's instructions.
- Design it here: an image editor on the page for placing pictures and text, with the finished plaque downloaded or sent for staff review before the app posts it to Discord under the visitor's name.
- Ask the artist: describe the plaque and the board's artist makes it.
- Festival -> Ema Plaques lets staff set up one board per festival (template, instructions, options offered, and each day's Discord webhook) and review submitted plaques; approving posts one, rejecting lets its author send another.
- Festival -> Ema Requests is the artist's own queue of what visitors asked for, marked complete once posted, under its own permission.

## [3.29.0] - 2026-10-29

### Added

- A festival dashboard at `/festival/<shortcode>` puts the stall map, stamp rally, garapon and raffles of a festival on one page, so a visitor can take part in all of them and see their stamps collected, draws left and raffles entered at the top.
- On the dashboard, the map marks stalls you have already stamped with a ring and a check, and a stall you still need offers a stamp-password box in its panel.
- A stamp card or garapon link for a rally in a published festival offers "Open the festival dashboard", which brings the link along.
- The dashboard remembers who you are on this device only, and "Forget this device" clears it.

## [3.28.0] - 2026-10-17

### Added

- A tea room whose unlock time arrives raises a pop-up on every open admin page for accounts that can manage tea rooms, naming the room, with a chime, a vibration on phones that support it and an Open Tea Rooms button; the site has to be open to hear it.
- Alerts stay until dismissed and stack in the top-right column instead of replacing each other, and a "Saved" toast no longer wipes one.

## [3.27.0] - 2026-09-19

### Added

- A Hide Custom Cards toggle in System -> Settings removes just the Custom Card request from the home page while games carry on; Hide Bingo still hides both, and existing request links and pending requests are untouched.

## [3.26.0] - 2026-09-05

### Added

- A tea room can be locked from a button on its row, optionally with a time (in your own timezone) at which it unlocks by itself; the row reads "Locked until 8:00 PM" or "Locked", and the same fields are on the room's edit form.

## [3.25.0] - 2026-08-31

### Added

- A published festival map can be embedded on another site with an `<iframe>` snippet from its admin page; the embedded map has panning, zoom, the day switcher and stall details, and stays current as stalls change.

### Changed

- Tapping a stall opens its details in a draggable, resizable panel over the map instead of further down the page, leading with whether the stall is open.
- The zoom, Fit and day controls sit in their own bar above the plan, and the legend is gone.
- A stall on a stamp rally is marked with a stamp icon instead of the stamp's artwork, which stays in the stall's panel.
- Festival times read like dates ("Sun, Aug 30, 2026, 8:00 PM - 1:00 AM EST") on festival pages.

### Fixed

- The public festival map no longer opens cropped or too small, and Fit shows the whole plan.
- Closing and dragging the stall panel work again.
- A vendor shows only on the days its hours cover, and a shared stall shows both vendors only when two are really there the same day.
- A stall names a day only when it means something, and the stall type is back on the map under the name.
- The booth page link, selection outline color and label color set in the map editor now save.
- Both tenants of a shared stall are shown in full, and line breaks in descriptions render instead of a literal `<br />`.

## [3.24.0] - 2026-08-28

### Added

- Your browser remembers the stamp rallies you signed up for, for 60 days: returning shows your links again, the rally list marks what you joined, and "Forget these links here" clears them.
- A raffle you have already entered fills in the name you used, so more entries top up your entry instead of starting a second one.

### Changed

- Home world is picked from a list on every form (sign-up, raffle entry, "Find My Links" and the custom card request), and the rally sign-up asks for the name and world separately.

### Security

- "Find My Links" no longer shows a drawing link, only the garapon and how many draws are left; the stamp-card link is still returned.

## [3.23.0] - 2026-08-26

### Changed

- A first visit for a player or public visitor now downloads about a third as much, since the admin pages and Markdown editor are left out (admins still get them offline after one visit), and the winners log loads faster.

### Fixed

- A mistyped current password when changing your own password no longer logs you out.
- The public raffle list shows only enterable raffles.
- Stamps on a rally linked to a festival map are labeled by their stall in the admin tables.
- Four icons that drew as nothing now render: the three Export CSV buttons and the Skip Occurrences heading.
- The festival map and stamp card placement editors are usable without a mouse, and book club row actions have accessible names.
- Stamp Rally and Festival Map Edit and Duplicate wait until the record has loaded, so a quick click can no longer wipe its stamps, prizes or pitches.
- Festival map pitch names are one size across the plan instead of varying with each pitch's height.

### Security

- The Settings page hides the Google Fonts API key and per-club Discord webhook URLs from accounts that are not full admins.
- CSV exports of the stamp and garapon logs can no longer carry a live spreadsheet formula from a participant name.
- Affiliate Carrd and Discord links on the public festival map can no longer use a non-http scheme.

## [3.22.0] - 2026-08-24

### Added

- Festival Map, a new page at the top of the admin Festival section: a titled floor plan with a markdown description, the festival's date ranges and a base image, with a status of In Progress, Published or Closed (only Published reaches the public).
- A visual stall editor draws stalls as circles or rectangles in their own color on the base image; drag to move, resize or rotate them, and positions scale to any screen.
- Each stall names what it offers under its title (Game, Food, Food & Game or Other, where Other takes its own wording such as "Omikuji"), which also seeds its shape and color.
- A stall can hold several occupants, each with its own affiliate, title, description, offering and dates, so a booth that changes hands between days is one shape; "Add another day" adds one.
- A day switcher on the public map, shown when the festival spans more than one date, leads with who is in each pitch that day and dims pitches nobody occupies.
- The public interactive map at `/festival-maps` and `/festival-maps/:id`: drag to pan, wheel or pinch to zoom, and tap a stall for what it offers, its hours, whether it is open now and the affiliate's card; the home page gets a Festival Map card once a map is published.
- A map can carry an optional shortcode such as "obon-2026" so it links as `/festival-maps/obon-2026`, while the id URL keeps working.
- Raffles can be filed under a Festival Map and pinned to one stall, and a pinned raffle appears in that stall's panel while it is open.
- A stamp rally can be linked to a festival map so its stamps are picked from that map's stalls, taking the stall's affiliate and stamp type; on the public map those stalls are ringed with their stamp art, with a "highlight the rally's stalls only" toggle and a link to the rally's sign-up page.
- Closed Garapons, Stamp Rallies and Festival Maps now have the row-level Copy action closed Raffles already had.

### Changed

- Duplicating a stamp rally or raffle no longer keeps its festival link, so next year's copy doesn't file itself under the finished festival.

## [3.21.0] - 2026-08-24

### Added

- Food stamps and game stamps: each stall on a rally card has a type picked in the rally form (Add Food Stamp / Add Game Stamp), and every existing stall is a food stamp.
- Card Completion on the rally form: keep the whole-card rule or require a number of each type, such as 3 food and 3 game stalls.
- The type shows in the rally's Stamps table, the stamp log and the participant's Stalls list, and the rally detail states what finishes a card.
- With a per-type rule, the participant's card shows progress per type ("1 of 3 food stamps - 0 of 3 game stamps collected") and a notice when the card can no longer be finished.

### Changed

- A complete card can keep collecting from stalls still open when the rally counts types, instead of locking the participant out.

## [3.20.0] - 2026-08-24

### Added

- Close / Reopen on a raffle, next to Duplicate, so a raffle with no entries or a details-only raffle can be taken off the public list; closing asks first, and neither touches a recorded winner.
- Duplicate on raffles, stamp rallies and garapons opens the create form pre-filled from an existing item, with `(Copy)` added to the title and the old run's dates cleared; issued cards, drawing links, draw logs and raffle entries stay with the original.
- Garapon and Stamp Rally show open counts in the admin sidebar.
- An "Entries Paid For" box in the raffle payment dialog records a part payment and shows how many entries remain unpaid.

## [3.19.0] - 2026-08-21

### Added

- Three raffle entry types on the raffle form's new Entry Type picker: Details Only (no sign-up form, instructions shown under "How to Enter"), Single Cost per Entry (the original behavior) and Custom Cost per Entry (a price per entry, which also sets the per-player limit).
- The public raffle page lists a custom raffle's price ladder with running totals, the sign-up form's Total Cost follows it, and raffle cards name the pricing.
- Recording a payment can waive part of the cost: the Paid button opens a dialog showing the entry's tickets, cost, amount already waived and amount outstanding, and the admin Add Entry row takes a waived amount too.
- Partly paid entries are visible: the entries table reads `3 (1 paid)` with a Partial pay button, and the Cost column shows `-X waived = Y`.
- A "Hide Bingo" switch in System -> Settings removes Join Bingo and the Custom Card request from the public home page while no game is running, leaving existing `/play/:cardId` and `/card-requests` links working.
- A "Where to Pay" image on raffles, shown to players with the sign-up instructions and on the confirmation after signing up.
- An "Already Entered?" search on the raffle page reports each match's name, world, entry count and payment status, and "Use this name" fills the form with that exact spelling so a near-miss doesn't start a second entry.

### Changed

- Custom Card now sits directly under Join Bingo on the home page.
- Stamp Rally sign-up links open in a new tab, and the warning reads "Save these links or keep this page open."
- The raffle sign-up confirmation leads with Amount Due and adds an "Already Covered" line for a returning entrant, and Max Entries Per Person is capped at 100.

### Fixed

- A long price ladder no longer overruns the raffle card.
- A raffle with a large per-player allowance no longer freezes the page while pricing an entry.

## [3.18.0] - 2026-08-11

### Added

- Public Stamp Rally sign-up: `/stamp-rallies` lists rallies open for sign-up, `/stamp-rallies/:id` is one rally's linkable sign-up form, and `/stamp-lookup` finds a participant's links; the home page shows a Stamp Rally card only while a rally is open.
- Sign-up asks for a full `Firstname Lastname @ World` name, with one card per name per rally; a repeat sign-up is sent to the lookup page.
- After signing up, the page shows the stamp card link and, when the rally has one, the Garapon draw link, each with a copy button and a warning that these links are the only way back in.
- The stamp rally form has a "Public sign-up" toggle (off by default), and the garapon form a "Default Draws" field for the allowance a self-service sign-up issues.
- The markdown editor has an emoji button.

### Changed

- The markdown editor keeps its formatting controls in a bar at the top, limited to formatting Discord can render.

## [3.17.0] - 2026-07-28

### Added

- Admin tables have banded rows and a hover highlight, and Manage Cards, the Winners Log, Server Logs and the garapon and rally logs let you drag a column's edge to resize it.
- Manage Cards and the Winners Log have row checkboxes and a "Delete N selected" action, with the header checkbox selecting every row matching the current filter.
- The garapon draw log can be filtered to a single prize, and the Winners Log, garapon draw log and rally stamp log export as CSV (every matching row, in the current sort order).
- A Konami code on the player board (up, up, down, down, left, right, left, right, B, A) clears every stamp and swells Drani's grin over the screen; it respects reduced motion.

### Changed

- A visual pass aligns the app with the senpan.cafe website (square corners, no shadows, Montserrat body text, uppercase display-font titles, icons in place of text glyphs), and the home page now has a centered logo masthead with Join Bingo as the primary panel; no theme color changed.
- The player's "It's Yoever" controls are one row, `It's Yoever: [Effects] [Sounds]`, and turning Effects off locks Sounds.
- Every table has the same toolbar order (search, per-page selector, count, actions), and tables sort across the full list rather than just the visible page.
- The theme editor's live preview shows the app's real panels, buttons, badges and board, so the contrast report matches what players see.

### Fixed

- Success and error text, including "request submitted", "you're entered" and "Sent to players!", was nearly invisible on every theme and is now readable.
- Page titles sat off-center on pages with a Back button, and pattern previews had uneven cell spacing.
- Three Server Logs buttons showed no fill, and buttons ignored a theme's accent hover colors.
- The connection-status dot kept pulsing for players who asked for reduced motion, and stamp-rally stall badges had unreadable white text on every theme.

## [3.16.0] - 2026-07-24

### Added

- The tea-room form has an optional Room owner field naming the character who owns the room, shown in the plugin's tea-room table and the public tea-room API.

## [3.15.0] - 2026-07-24

### Added

- The affiliate form has an optional Subtitle under Name, shown beneath the name in the affiliates list.
- Accounts with the Images permission get an Upload button beside every image picker's category selector and can drop files on the thumbnail grid, so a missing image no longer means leaving the form.

### Changed

- Image pickers list images alphabetically by file name (digit-aware) instead of newest first.

## [3.14.1] - 2026-07-21

### Changed

- Stacked dialogs behave correctly: only the top one reacts to Escape or holds focus.
- The admin sidebar sections, the pattern delete, rename and collapse controls and expandable table rows work from the keyboard and screen readers.

### Fixed

- Moving between admin pages no longer drops the live connection, so draws, winners and log updates in the gap aren't missed.
- A re-delivered draw no longer double-counts a number or replays its chime, and the "Live" badge no longer flashes "closed" during a deliberate reconnect.
- Fast navigation could show an older response over a newer one (Carrd folders, public raffle details, stamp-rally panels).
- Double-clicking the custom card request submit button no longer sends it twice.
- The player board recovers from corrupted saved stamps and keeps a stamp opacity of 0.
- The draw hotkey (Space/Enter) no longer fires behind a dialog or while a button-like control is focused.
- Loading the admin login or register page while signed in now redirects instead of showing the form.

### Security

- Theme values in the editor preview and theme flourish artwork are sanitized in the browser so a hostile theme can't inject CSS or script.

## [3.14.0] - 2026-07-20

### Added

- The New Game form has an "Auto-draw numbers" toggle with a "Time Between Calls" selector (10s to 5m) so the server draws a number every interval; during a live game an Auto-Draw On/Off button and interval selector change it at any time.
- Game Presets have the same Auto and Time Between Calls fields, which applying a preset pre-fills.

### Changed

- The half-time prompt appears on every admin screen at once for any draw; auto-draw pauses at half-time and when a winner is recognized, and "No mini-game" resumes it.
- The Current Game view leads with Draw Number, Delay and End Game, and moves Auto-Draw, It's Yoever and Winner Sound into a collapsible Game Settings panel.
- The Current Game "Live" badge and sidebar live-game dot are green instead of red.

### Fixed

- Several buttons and notices (the Images and Carrd Save buttons, the theme editor's contrast warning, the version-mismatch flag) showed a blank space instead of their icons.

## [3.13.0] - 2026-07-19

### Added

- A public theme picker in the site footer: themes marked Public in the Themes editor can be chosen by each player and are remembered per browser, with a "Default" option that follows the admin's active theme.
- A Personal Card Requests page (`/card-requests`, linked from the home page) where a player enters a character name and world, builds a 5x5 card by hand or with Generate Random, picks a custom 6-character card ID, and sees the gil cost and terms before submitting.
- Manage Cards shows a hollow star for a pending custom card, a filled star for an approved one and a lock for a Protected card, with Approve and Protect/Unprotect row actions; Delete All now keeps Protected cards.
- A Custom Card Cost setting (System -> Settings -> Gameplay), the gil price shown on the Personal Card Requests page.
- The Called Numbers tracker dims any BINGO column the game's win patterns never use.

## [3.12.0] - 2026-07-16

### Changed

- The Plugin version in the admin sidebar footer now reflects the version currently published to the plugin repository.

## [3.11.0] - 2026-07-16

### Added

- The Garapon drawing page shows a "View your Stamp Rally card" link when the garapon is tied to a stamp rally and the player holds a linked card.

## [3.10.0] - 2026-07-12

### Added

- The stamp-rally editor has a "Where to Redeem" image picker, usually a screenshot, shown on the participant's card with the redeem instructions once the card is complete.

## [3.9.0] - 2026-07-12

### Added

- The Images tab shows a percentage bar while files upload, then "Processing...", instead of a spinner that looked stuck.

### Fixed

- Large or multiple image uploads over a slow connection were cut off after 30 seconds with "Request timed out. Please try again."; font, Carrd and book-club uploads had the same problem and now take as long as they need.

## [3.8.0] - 2026-07-12

### Added

- Affiliates has a shared Discord webhook on a Webhook sub-screen and a Post button on each row that posts the affiliate as an embed.
- The affiliate list is drag-sortable with a persistent order, and each row shows the logo (or screenshot), an embed-color swatch and Discord, Carrd and open-times badges.
- The affiliate form has fields for an embed accent color, a Discord Link and a Carrd Link.

## [3.7.0] - 2026-07-11

### Added

- An "It's Yoever" button on the player board: while a game is running and the reaction is on, a player triggers a sound and a bouncing head image on every connected screen, captioned with their name, followed by a cooldown (default 3 minutes) that survives a refresh.
- Two per-device toggles, "Show It's Yoever effects" and "Play It's Yoever sound", on the player board and the admin Game tab.
- In a live game the Game tab has an on/off switch for the reaction and a "Yoevers: N" counter, both reset for each new game.
- An "It's Yoever" Cooldown (seconds) setting in Gameplay, 0 to 3600, where 0 means no limit.
- The Frontend, Backend and Plugin versions in the admin sidebar footer open their changelog in a modal, and the Plugin version is new.

### Fixed

- Saving a numeric setting failed with "Invalid JSON."

## [3.6.0] - 2026-07-10

### Added

- A Tea Rooms page (Senpan Tea House -> Tea Rooms) with a drag-orderable list of bookable rooms, each with a name, room number, gil price per half hour, hashtags, description, status flags and image, and row actions to post to Discord, open/close, discount, copy the API link, edit and delete.
- A Webhook sub-page stores the shared Discord webhook the rooms post to, and the page is gated by a new `teahouse-tea-rooms` permission.
- A posted room's Discord embed shows the name, description, cost (halved with a "Currently Discounted!" note when discounted), room number and Open/Closed status.

## [3.5.0] - 2026-07-07

### Added

- The Server Logs viewer has a User column naming who made each request: an admin's username, a verified bot's name, or "-" for anonymous traffic.

### Fixed

- Admins without image access got a 403 error toast on every image change, and a renamed or deleted image category raised an "Unknown image category" toast on later updates.

### Security

- A font name or header font containing characters that could break out of CSS is refused and replaced with the default header font.

## [3.4.0] - 2026-07-05

### Added

- A Server Logs page (System -> Logs, admin-only): a live-tailing viewer with colored method and status badges, a minimum-level filter, text search, a pause/resume "Live" toggle, a per-page selector and pagination, with each line expanding to its full JSON on click.
- A Debug On/Off button changes the server's log level without a restart.

### Fixed

- An error with no JSON body, such as an empty or HTML 502 from the gateway, now reads `Request failed (HTTP 502)` instead of a bare `Request failed`.

## [3.1.0] - 2026-07-03

### Changed

- The image picker in the announcement, raffle, garapon, affiliate, stamp-rally and theme-flourish editors browses the whole image library through its own category dropdown, opening in the selected image's category (flourish pickers offer only `.svg`).
- Every image category can now be renamed and deleted, and the "Permanent" badge is gone; deleting a category still removes its folder and files.
- Open image pickers pick up another admin's uploads live.

## [3.0.0] - 2026-07-02

### Added

- An "Embed on External Sites" panel on Font Upload copies a permanent `kit.css` `<link>` snippet for Carrd sites, each receiving only the fonts whose allowed sites include it; a font's Copy URL button copies a tokenized link that expires in 1 to 2 weeks.
- A live-preview format switch previews a font in each of its actual formats.

### Changed

- Uploaded fonts load from rotating same-origin links instead of static `https://fonts.senpan.cafe/<file>` URLs, so the font picker and player-facing fonts work regardless of any external allowlist.
- Font Upload is organized around families: files sharing a base name appear as one font, the table shows CSS Name, Serves, Modified and actions, and an Edit modal holds the CSS name, served-version picker, allowed-sites list and per-file rename and delete.

## [2.2.0] - 2026-07-02

### Added

- The login page offers a usernameless "Sign in with a passkey" option, and every account can add, name and remove passkeys under User Options -> Add Passkey, alongside the password.
- The public raffle sign-up shows the Cloudflare Turnstile bot check, and requires it, when Turnstile is configured.

## [2.1.0] - 2026-07-02

### Changed

- Clickable cards, chips and collapsible headers work from the keyboard and screen readers.
- Pattern and announcement drag-and-drop no longer needs `unsafe-eval`, so the site can enforce a strict Content-Security-Policy.

### Fixed

- Switching quickly between raffles, garapons, stamp rallies or book-club lists could leave the wrong record showing.
- Removing a row from a repeating list (garapon prizes, affiliate hours and owners, announcement buttons, reading-list sources) could leave the inputs showing the wrong row's values.
- API requests now time out after 30 seconds instead of leaving a spinner forever on a hung network.

### Security

- The post-login `?redirect=` value is checked to be a same-origin path, and a 401 now clears the whole signed-in session so stale page permissions can't linger.

## [2.0.0] - 2026-06-30

### Changed

- The app uses the REST resource paths introduced in backend 2.0.0, and requires backend 2.0.0 or later.

## [1.5.2] - 2026-06-30

### Changed

- No user-visible changes.

## [1.5.1] - 2026-06-30

### Fixed

- A corrupted saved board in the browser, or a full browser storage, could break loading stamps or stamping a cell; a bad saved board now starts clean and a full storage no longer throws.

## [1.5.0] - 2026-06-29

### Added

- Personal access tokens (User Options -> Access Token): an account can generate a token so an external client, such as a Dalamud plugin, can sign in as that account with its page permissions. The secret is shown once with a copy button, and the token can be regenerated or revoked.

## [1.4.0] - 2026-06-29

### Added

- A Garapon can link to an open Stamp Rally; issuing a drawing link then also issues a stamp card sharing the same token, with a copy button for each in the drawing-links table.
- Stamp rallies have an open or closed status separate from the date window: an Open card grid and a Closed table with Close/Reopen, where a closed rally is read-only and not offered for Garapon linking.
- Each open rally card shows "X/Y stalls active" and a Manage stalls panel to pause or resume vendor stalls.

## [1.3.1] - 2026-06-29

### Changed

- Reordered the items within each admin sidebar section.

## [1.3.0] - 2026-06-29

### Added

- A Stamp Rally page (Festival -> Stamp Rally) for authoring stamp-rally events: a card background and "not stamped" placeholder, availability window, markdown details and "How to Redeem" instructions, and stamps and prizes placed with a visual drag, resize and rotate editor.
- Each stamp links to an affiliate stall (or Senpan Tea House), has a password and an optional active window, and can be paused and resumed.
- Admins issue each participant a tokenized `/stamp-card/<token>` link; participants enter stall passwords to collect stamps, and a completed card reveals its prizes and redeem instructions.
- A per-event View Logs page lists every collection (participant, stall, time) and updates live.
- Three new image categories (Stamp Cards, Stamp Stamps, Stamp Prizes) and a `festival-stamp-rally` page permission.

## [1.2.0] - 2026-06-29

### Added

- An Affiliates page (Senpan Tea House -> Affiliates) for partner establishments: name, owners, location, opening hours across time ranges under one timezone, markdown details, and a logo and screenshot picked from two new image categories; gated by a new `teahouse-affiliates` permission.

## [1.1.1] - 2026-06-28

### Added

- The hidden registration form shows the Cloudflare Turnstile bot check too, gated the same way as the login form.

## [1.1.0] - 2026-06-28

### Added

- The admin login shows a Cloudflare Turnstile bot check and requires it when the server has a Turnstile site key.

## [1.0.0] - 2026-06-28

### Added

- A version readout in the admin sidebar footer showing the frontend and backend versions, with a flag when their major versions differ.
- A WCAG 2.1 contrast report in the theme editor: a live audit of the app's text and background pairings with AA and AAA verdicts, updating as colors change.
- A token-based theme editor with a collapsible live preview, per-token help text and color pickers (alpha-capable for translucent tokens).

### Changed

- All built-in themes were retuned for contrast, and a Toji theme was added.

### Removed

- The CodeMirror free-form CSS theme editor; themes are now structured tokens.

[Unreleased]: https://github.com/jwill89/app-senpan-suite/compare/v5.6.0...HEAD
[5.6.0]: https://github.com/jwill89/app-senpan-suite/compare/v5.5.0...v5.6.0
[5.5.0]: https://github.com/jwill89/app-senpan-suite/compare/v5.4.0...v5.5.0
[5.4.0]: https://github.com/jwill89/app-senpan-suite/compare/v5.3.0...v5.4.0
[5.3.0]: https://github.com/jwill89/app-senpan-suite/compare/v5.2.0...v5.3.0
[5.2.0]: https://github.com/jwill89/app-senpan-suite/compare/v5.1.0...v5.2.0
[5.1.0]: https://github.com/jwill89/app-senpan-suite/compare/v5.0.1...v5.1.0
[5.0.1]: https://github.com/jwill89/app-senpan-suite/compare/v5.0.0...v5.0.1
[5.0.0]: https://github.com/jwill89/app-senpan-suite/compare/v4.1.0...v5.0.0
[4.1.0]: https://github.com/jwill89/app-senpan-suite/compare/v4.0.0...v4.1.0
[4.0.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.32.0...v4.0.0
[3.32.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.31.1...v3.32.0
[3.31.1]: https://github.com/jwill89/app-senpan-suite/compare/v3.31.0...v3.31.1
[3.31.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.30.0...v3.31.0
[3.30.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.29.0...v3.30.0
[3.29.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.28.0...v3.29.0
[3.28.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.27.0...v3.28.0
[3.27.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.26.0...v3.27.0
[3.26.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.25.0...v3.26.0
[3.25.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.24.0...v3.25.0
[3.24.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.23.0...v3.24.0
[3.23.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.22.0...v3.23.0
[3.22.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.21.0...v3.22.0
[3.21.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.20.0...v3.21.0
[3.20.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.19.0...v3.20.0
[3.19.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.18.0...v3.19.0
[3.18.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.17.0...v3.18.0
[3.17.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.16.0...v3.17.0
[3.16.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.15.0...v3.16.0
[3.15.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.14.1...v3.15.0
[3.14.1]: https://github.com/jwill89/app-senpan-suite/compare/v3.14.0...v3.14.1
[3.14.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.13.0...v3.14.0
[3.13.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.12.0...v3.13.0
[3.12.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.11.0...v3.12.0
[3.11.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.10.0...v3.11.0
[3.10.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.9.0...v3.10.0
[3.9.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.8.0...v3.9.0
[3.8.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.7.0...v3.8.0
[3.7.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.6.0...v3.7.0
[3.6.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.5.0...v3.6.0
[3.5.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.4.0...v3.5.0
[3.4.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.1.0...v3.4.0
[3.1.0]: https://github.com/jwill89/app-senpan-suite/compare/v3.0.0...v3.1.0
[3.0.0]: https://github.com/jwill89/app-senpan-suite/compare/v2.2.0...v3.0.0
[2.2.0]: https://github.com/jwill89/app-senpan-suite/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/jwill89/app-senpan-suite/compare/v2.0.0...v2.1.0
[2.0.0]: https://github.com/jwill89/app-senpan-suite/compare/v1.5.2...v2.0.0
[1.5.2]: https://github.com/jwill89/app-senpan-suite/compare/v1.5.1...v1.5.2
[1.5.1]: https://github.com/jwill89/app-senpan-suite/compare/v1.5.0...v1.5.1
[1.5.0]: https://github.com/jwill89/app-senpan-suite/compare/v1.4.0...v1.5.0
[1.4.0]: https://github.com/jwill89/app-senpan-suite/compare/v1.3.1...v1.4.0
[1.3.1]: https://github.com/jwill89/app-senpan-suite/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/jwill89/app-senpan-suite/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/jwill89/app-senpan-suite/compare/v1.1.1...v1.2.0
[1.1.1]: https://github.com/jwill89/app-senpan-suite/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/jwill89/app-senpan-suite/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/jwill89/app-senpan-suite/releases/tag/v1.0.0
