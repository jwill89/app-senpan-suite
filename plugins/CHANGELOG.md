# Changelog

All notable changes to **SenpanCompanion**, a Dalamud/FFXIV plugin ("Senpan Admin Companion") that lets Tea House staff drive app services from in-game, are recorded here, in the format of [Keep a Changelog](https://keepachangelog.com/). It signs in with a personal access token and is distributed through a Dalamud custom repository.

The plugin keeps its own version, apart from the app's, as Dalamud needs: the four-part AssemblyVersion, which follows [Semantic Versioning](https://semver.org/). The app's changelog is [CHANGELOG.md](../CHANGELOG.md) at the repo root.

## [Unreleased]

### Changed

- Needs app 5.5.0, whose account answer says whether the account is an owner.

## [4.0.0.0] - 2026-11-10

### Changed

- Needs app 5.0.0: the plugin moves to the API on `/api/v1`, so an older plugin can't reach a 5.0.0 server and this one can't reach an older server.
- A refused action shows the server's own reason, and a server that can't be reached says what to do.
- Live updates come over the staff event stream instead of the WebSocket, reopening on its own after a drop and reading the game and the cards again each time it opens. A token or account the server refuses stops it until the settings change, and the status bar says so.
- Every console stays in step: a half-time prompt answered elsewhere closes here with a line in your chat log saying who answered, the End Game confirm closes when the game ends or another starts, and game details edited elsewhere arrive without replacing what you are typing.
- The draw delay is the one every console shares, so the plugin's own delay setting is gone.
- Raffles, Garapon, Stamp Rally, Tea Rooms and the Winners log read again when their records change anywhere. A raffle's winner is the server's: a winner picked on any console shows here, and confirming or drawing another names that winner.
- A tea-room lock with no unlock time lasts until someone unlocks the room.
- The server address must be `https://`, or plain `http://` for a server on this PC, so the token is never sent in the clear.

## [3.9.1.0] - 2026-10-29

### Fixed

- A tell or timed-macro message still waiting to be sent when the plugin unloads is dropped instead of reaching chat after the plugin has gone.

## [3.9.0.0] - 2026-10-17

### Changed

- An expiring tea-room lock raises a "Tea room unlocked" notification naming the room, which stays on screen until you dismiss it instead of fading like the game toast. The chime is unchanged, the chat line carries the same sentence, and the website shows the same alert.
- Only staff who can manage tea rooms are alerted; bingo-only staff, who have no way to extend a lock, no longer get it.

## [3.8.0.0] - 2026-09-05

### Added

- Lock a room from the Tea Rooms page, with an unlock time: the board gains a Lock column, and locking asks when the room should free up (a local date and time, or +30m / +1h / +2h / +4h). Left blank, the room stays locked until someone unlocks it.
- When a lock expires you get a two-note chime, a toast and a line in your chat log naming the room.

### Fixed

- The room list corrects itself after a dropped connection, instead of claiming a room is still locked when its lock expired while the connection was down.

## [3.7.0.0] - 2026-08-28

### Changed

- A drawing link or stamp card issued from the plugin now carries the home world, taken straight from the game when the name came from the Nearby... picker. Typing a name by hand still works: the server splits a "Name @ World", and the hint now says so.
- Player and participant lists show "Name @ World", as the rest of the suite does.

### Fixed

- A stamp card issued alongside a drawing link is no longer the same secret. They used to share one token, so the shareable card link spent the player's draws; each now has its own.

## [3.6.0.0] - 2026-08-26

### Fixed

- After a failed load (an invalid server URL, for example) the card list and the account could stay stuck, never refreshing again and with no error shown.
- The winner chime plays reliably.
- A half-time prompt raised near the end of a game no longer pops up during the next one, and one that arrives while the Game page isn't on screen chimes and prints a line in your chat log telling you to open the window.
- The Draws box starts at the garapon's configured default instead of always 1.
- Picking a raffle while another load was in flight no longer leaves the old raffle on screen, so Add entrant and Pick a winner apply to the raffle shown.
- Typing in the auto-tell message no longer saves the settings on every keystroke.
- The Bingo Game page and the cards reload when the live connection returns, so a draw missed during a drop no longer leaves the called-numbers grid short or ends the game with stale winners.
- Clicking Draw Number, the garapon and stamp-rally pickers or Save & Test Connection no longer garbles the rest of the window for a frame.

## [3.5.0.0] - 2026-08-24

### Added

- A Type column on the Stalls table and the collected-stamp log, reading Food or Game; a stall the server sends without a type reads as Food.
- What finishes a card shows on the rally header (every stamp, or the required food and game counts), and the stall summary splits into food and game.

### Changed

## [3.4.1.0] - 2026-08-23

### Added

- A payment dialog on the Paid cell replaces the paid checkbox, with the website's settlement: how many tickets the payment covers and the gil waived. The cell reads Paid / Partial / Unpaid, and Clear payment is there too.
- Timed macros: Test send puts the message out once so it can be checked in chat. It doesn't count towards the send total or start the timer.
- Timed macros: Reset is offered whenever there is progress to clear, not only once a send cap is reached, and is refused while the macro is running.

### Changed

- The macro start button reads Start rather than Send, now that Test send sits next to it.

### Fixed

- A details-only raffle no longer offers to sign anyone up: the Add entrant form is replaced by a note that players enter it outside the app, and the draw's caption says it runs over every entrant rather than paid tickets. Entrants for one are still recorded on the website.

## [3.4.0.0] - 2026-08-21

### Added

- The raffle header names the pricing per entry type: the ladder for a custom-cost raffle (`Cost per entry: 50000 / 100000 / 150000`), the flat price for a single-cost one, and "Details only - players sign up elsewhere" for a raffle that takes no sign-ups through the app.
- Part-paid entrants read as `settled / total` tickets with a tooltip, and hovering the Paid box reports what has been waived on that entry. Ticking the box settles the rest, waiving nothing; entering an amount waived stays on the website.

## [3.3.2.0] - 2026-07-28

### Changed

- Status and table glyphs are icons now: the live/offline badge, the garapon ball swatch, the stamp-rally completion mark and the roll stars were drawn as plain text, which the game's font renders inconsistently.
- On-screen text uses plain ASCII punctuation: a hyphen for an empty table cell and three dots for progress labels such as "Loading...".

## [3.3.1.0] - 2026-07-24

### Added

- The Tea Rooms page gains an Owner column beside the room name, showing the character who owns each room.

### Changed

### Fixed

- Accented characters now render. Text anywhere in the window (for example a room name containing an ō) used to drop any glyph the game's font doesn't include; macron romanizations (ō, ā, ē, ī, ū) now display, and every other glyph is unchanged.

## [3.3.0.0] - 2026-07-24

### Added

- A Tea House -> Tea Rooms page lists every room by number and name (with its per-half-hour cost) and lets staff toggle each room's Open and Discount (50% off) status with a click. It needs the Tea Rooms permission, and everything else about a room stays on the website.

## [3.2.1.0] - 2026-07-21

### Fixed

- Every outgoing chat message, auto-tells and Timed Text Macros alike, goes through one send queue that spaces messages a second apart, so concurrent features can no longer defeat the game's chat throttle.
- A rejected token (401 or 403) no longer makes the plugin retry every few seconds forever: it backs off until the token is changed, while transient drops back off progressively and reset on a clean reconnect.
- A failed initial card load retries after a short cooldown instead of leaving the card list empty until restart.
- The outgoing-message length limit counts bytes, as the game does, so a message with multibyte characters is no longer wrongly truncated or sent over budget, and never splits a character.
- Timed-macro progress saves in the background, so the file write no longer stalls the window.

### Security

- The personal access token is encrypted at rest with Windows DPAPI instead of stored as plain text in the config, so a synced, backed-up or world-readable config no longer exposes a usable token. An existing plain-text token is migrated once on load and cleared from disk.
- The log warns once when the configured server URL is not HTTPS, since the token would be sent in cleartext.
- A hostile or broken server can no longer exhaust memory by streaming an unbounded live message; each is capped at 1 MB.

## [3.2.0.0] - 2026-07-20

### Added

- Auto-draw controls: the New Game view gains an Auto-draw numbers checkbox and a Time Between Calls combo (a preset pre-fills them), and the live game view gains an Auto-Draw on/off checkbox and interval combo kept in sync with the website.

### Changed

- The half-time prompt is server-driven: the plugin opens the mini-game prompt when the server announces it, for manual and automatic draws alike. Yes alerts players and leaves auto paused, No declines and resumes auto if it was paused, and the prompt notes when auto has been paused.

## [3.1.0.0] - 2026-07-19

### Added

- Card status and actions in the Bingo Cards tab: a Status column marks pending custom-card requests, approved ones and Protected cards, and each row gains Approve and Protect / Unprotect. Approving makes the card playable and protects it, and Protected cards are spared by Delete all.

### Changed

## [3.0.1.0] - 2026-07-17

### Changed

- Button colors match the web admin dashboard's default theme (a tan primary action with dark text, an olive secondary and a red destructive), replacing the initial purple accent.
- Bingo win patterns gain a Collapse all / Show all toggle to fold or unfold every pattern category at once while building a game.

### Fixed

- A Timed Text Macro's message is now actually hidden until you expand it. The preview text used to stay visible with only the button label flipping; Show message / Hide message now shows or hides the full text, and the button's stray glyph (it showed as "=") is gone.

## [3.0.0.0] - 2026-07-17

### Added

- Edit a saved Timed Text Macro in place: each macro has an Edit button that reopens its name, channel, message, interval and send cap in an inline editor (Save / Cancel). Stop a running macro first to edit it.
- A shared design across every page: icon and accent section headers, a three-tier button system (primary / secondary / destructive), compact icon buttons for table-row actions (copy / delete), pill badges and rounded cards.

### Changed

- The always-available tools (Rolls, Timed Text Macros, Settings, About) live under a collapsible General group at the bottom of the sidebar.
- Timed Text Macros shows each macro as its own card: a numbered title with channel and status badges, a schedule summary and a one-line message preview that expands on demand, so a long macro no longer floods the panel.
- Rolls is split into labeled Captured rolls / Filter / Find the winner / Rolls sections, and Settings groups its fields under Connection and Automatic tells.

## [2.3.0.0] - 2026-07-16

### Added

- A Rolls page, needing no account or permission, that watches chat for `/random` and `/dice` rolls near you and lists the player, home world, number rolled (with its ceiling for `/random N`) and time.
- Rolls can be filtered to a player and/or the last N minutes, and a Highest / Lowest / Closest-to-N query brings the matching rolls to the top; a player who rolled more than once in the window is flagged. Rolls are held in memory only, clear on logout, and a Clear button wipes them.
- A Timed Text Macros page, needing no account or permission: any number of named announcements, each sent on a chosen public channel (Say / Yell / Shout) every N minutes, with an optional send cap and a live countdown. A long message is split into parts sent a second apart.
- Macros are saved across logout and restart but always reload stopped; logout also stops every running macro.

### Fixed

- Long text wraps to the window instead of running off the right edge on the Rolls, Timed Text Macros, Settings, Garapon, Stamp Rally, Raffle and Bingo pages.

## [2.2.1.0] - 2026-07-16

### Added

- Editable auto-tell templates for all three tells (bingo card, garapon, stamp rally): enabling a tell in Settings reveals a message editor, with placeholders for the recipient's name and the link. The defaults reproduce the earlier fixed messages.

### Changed

- Long tells are split to fit the in-game chat limit, at a sentence end where possible and otherwise a word boundary, with the parts sent a second apart. The Settings editor warns when a template will split.

## [2.2.0.0] - 2026-07-16

### Added

- A Garapon page to issue a per-player drawing link for an existing garapon (pick the name from the nearby list, set the draw count, copy the link and, with a Stamp Rally, the card link), and a Garapon Draw Log page. Needs the Festival Garapon permission.
- A Stamp Rally page to issue a participant card for a nearby player and pause or resume individual stalls, and a Stamp Rally Log page showing the collected-stamp log. Needs the Festival Stamp Rally permission.
- An optional `/tell` that sends a player their link or card when you issue one for someone picked from the nearby list; off by default and switched on per feature in Settings.
- A collapsible sidebar replaces the flat tab bar, with Bingo and Festival sections, Settings and About. Pages the account lacks the permission for are hidden, and the window opens on the first page it can reach.

## [2.1.0.0] - 2026-07-11

### Added

- "It's Yoever" live controls on the Bingo Game tab: a checkbox to switch the reaction on or off for all players during a live game, and a "Yoevers: N" counter. Both follow the server live.

### Changed

## [2.0.1.0] - 2026-07-02

### Security

- The personal access token is sent in the `Authorization` header on the live connection instead of the URL query string, where it could land in access logs.

## [2.0.0.0] - 2026-07-01

### Changed

- The plugin's API client was reworked for the backend's move to a hybrid RESTful style with proper HTTP status codes, and needs the correspondingly migrated backend.

## [1.0.0.0] - 2026-06-29

### Added

- First published release, through the Dalamud custom repo.

### Changed

- Cards are sorted by creation date.

## [0.1.0.0] - 2026-06-29

### Added

- Initial Dalamud plugin (Senpan Companion): operate bingo and raffle features from within the game, authenticated by a personal access token.

[Unreleased]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v4.0.0.0...HEAD
[4.0.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.9.1.0...Plugin-v4.0.0.0
[3.9.1.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.9.0.0...Plugin-v3.9.1.0
[3.9.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.8.0.0...Plugin-v3.9.0.0
[3.8.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.7.0.0...Plugin-v3.8.0.0
[3.7.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.6.0.0...Plugin-v3.7.0.0
[3.6.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.5.0.0...Plugin-v3.6.0.0
[3.5.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.4.1.0...Plugin-v3.5.0.0
[3.4.1.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.4.0.0...Plugin-v3.4.1.0
[3.4.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.3.2.0...Plugin-v3.4.0.0
[3.3.2.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.3.1.0...Plugin-v3.3.2.0
[3.3.1.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.3.0.0...Plugin-v3.3.1.0
[3.3.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.2.1.0...Plugin-v3.3.0.0
[3.2.1.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.2.0.0...Plugin-v3.2.1.0
[3.2.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.1.0.0...Plugin-v3.2.0.0
[3.1.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.0.1.0...Plugin-v3.1.0.0
[3.0.1.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v3.0.0.0...Plugin-v3.0.1.0
[3.0.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v2.3.0.0...Plugin-v3.0.0.0
[2.3.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v2.2.1.0...Plugin-v2.3.0.0
[2.2.1.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v2.2.0.0...Plugin-v2.2.1.0
[2.2.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v2.1.0.0...Plugin-v2.2.0.0
[2.1.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v2.0.1.0...Plugin-v2.1.0.0
[2.0.1.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v2.0.0.0...Plugin-v2.0.1.0
[2.0.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v1.0.0.0...Plugin-v2.0.0.0
[1.0.0.0]: https://github.com/jwill89/app-senpan-suite/compare/Plugin-v0.1.0.0...Plugin-v1.0.0.0
[0.1.0.0]: https://github.com/jwill89/app-senpan-suite/releases/tag/Plugin-v0.1.0.0
