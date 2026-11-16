# Senpan App Suite — Plan

The apps of a Discord community, the Senpan Tea House and Atelier YAO, at **apps.senpan.cafe**: bingo for its events, raffles, book clubs, scheduled Discord announcements, the tea house's affiliates and tea rooms, and the festival's lottery drum, stamp rally, map and ema plaques. Staff run it from the website's admin pages and, in game, from the FFXIV plugin; players and visitors use the public pages without an account.

This file holds the product decisions, the data model and the phase order. Anything that changes them changes this file in the same change, with a dated line in [docs/decisions.md](docs/decisions.md). How the code is built, and every recipe, is in [AGENTS.md](AGENTS.md).

## Decisions

| Area | Decision |
| --- | --- |
| Shape | One Go binary with the Vue SPA inside it, on the droplet behind Apache and Cloudflare, with SQLite beside it. The FFXIV plugin is a second admin UI over the same API, never talking to another client: the server is the one source of truth. |
| Accounts | Staff only. None is built in: the first admin is made on the host, and the owners the server's settings name can't be removed by anyone. Accounts are made inactive (the hidden registration page) or by an admin's one-time Discord link, and an admin grants each page. Visitors and players never sign in: a card, a drawing link or a stamp card is the capability, by its unguessable link. |
| Bingo | 75-ball cards, drawn by staff with an optional delay before players see each number; players stamp their own boards on their own device, and the app never tracks a player's board for them. |
| Raffles | Entries merge per character and world; the entrant list is public, and a waiver is private. A paying raffle draws on settled tickets, a details-only one on every entry. |
| Festival | A garapon draw isn't gated on the participant's stamp card being complete. A stamp card and its drawing link are separate secrets. Every plaque designed on the site is reviewed by staff before it reaches Discord, and a person hangs one plaque a day. |
| Discord | Posts go to each page's webhook through one sender, with a dry run and an override for development, which runs on a copy of the live database. Moving to a built-in bot is built on the branch `bot-replacement` ([plans/discord-bot.md](plans/discord-bot.md)). |
| Uploads | An upload keeps its file name, and a same-named upload replaces it, since staff swap pictures in place and the Carrd sites link by name. Fonts are licensed, so they are never served as files, only through per-site tokenized links. |
| Look | Themes change only the design tokens, never classes. The header font is any Google Font an admin picks, and the ema designer offers them all; the site fetches each once and serves it, as it serves the body font. |

## Data model

The tables, their columns and their indexes are in [AGENTS.md](AGENTS.md#database-tables), and the schema the migrations build is `backend/schema.sql`. In short: accounts with their passkeys, Discord links and access tokens; bingo's cards, patterns, games and draws; raffles and their entries; the book clubs' reading lists; announcements with their types and roles; affiliates and tea rooms; and the festival's maps (pitches and their occupants), garapons, stamp rallies and ema boards.

## Phases

1. **Done:** the suite's features through 5.4.0, and its move onto shared-tooling's standards ([plans/shared-tooling-rollout.md](plans/shared-tooling-rollout.md), complete 2026-11-15).
2. **Next:** the Discord bot in place of webhooks, rebuilt on `main` from the `bot-replacement` branch ([plans/discord-bot.md](plans/discord-bot.md)).
3. **Later:** what more Discord sign-in could do ([plans/discord-sign-in-later.md](plans/discord-sign-in-later.md)).
