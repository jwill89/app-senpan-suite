# Decisions

The decisions worth remembering, one dated line each, newest last. The larger ones' reasoning is in [PLAN.md](../PLAN.md).

- 2026-11-06 — A failed smoke check puts the pre-deploy database snapshot back with the previous binary, so the release starts on the schema it knows; what was written in the seconds the new release ran is lost with it.
- 2026-11-06 — CI is the gate: the pre-push hook that ran the whole check before every push is gone.
- 2026-11-07 — The bootstrap `admin`/`admin` account stays: an install changes its password as soon as it makes it, and otherwise it serves only development. (Replaced on 2026-11-16.)
- 2026-11-07 — Font Awesome kits hold only the styles the site imports, not hand-picked icons, so a new icon needs no new kit version.
- 2026-11-09 — The overhaul onto the shared standards ships as five releases, each deployed on its own: one app version and the SPA in the binary (4.0.0), the data layer (4.1.0), the API on huma (5.0.0), the front-end cleanup (5.1.0) and nothing served from a document root (5.2.0).
- 2026-11-09 — The plugin moves with the API in 5.0.0, deployed the same day: no endpoint keeps its old field names, and the live feed moves from WebSockets to server-sent events, its traffic being one-way.
- 2026-11-10 — Uploads keep the uploaded file name, and a same-named upload overwrites: staff replace a picture in place by its name, and the Carrd sites' links depend on names.
- 2026-11-10 — JPEG and PNG uploads are re-encoded; GIF and WebP are checked and kept as sent, since Go writes no WebP and re-encoding a GIF risks its animation.
- 2026-11-11 — The unit and the vhost ship from templates in the repo with every deploy; the service's secrets live in a root-only env file, and the unit may write only to the data folder.
- 2026-11-13 — Nobody makes an account by signing in with Discord: an admin's one-time link signs someone up or links an account.
- 2026-11-15 — No restore drill is run on the host; its steps are in docs/ops.md for when one is wanted.
- 2026-11-15 — A reusable component's classes (those of `frontend/src/components/common/`) are BEM, with the component's name as the block; the shared CSS objects keep joining their parts with `-`.
- 2026-11-15 — The one text editor is Tiptap 3 with its Markdown package, storing Markdown, in place of Milkdown.
- 2026-11-15 — The CHANGELOG is one file per versioned part: the app's (with the SPA's own history before 4.0.0), the plugin's, and the backend's closed history; each version is tagged at its release commit, the plugin's and the backend's with their existing `Plugin-v` and `Backend-v` prefixes.
- 2026-11-16 — The festival map's embed stays: other sites may frame it when System -> Settings lists them, and every other page refuses every framer.
- 2026-11-16 — The plugin takes the shared C# style with no exceptions of its own.
- 2026-11-16 — Every font is served by the site: a Google font picked as the header font or in the ema designer is fetched by the server once and kept, so no visitor's browser loads from Google.
- 2026-11-16 — No account is built in: the first admin is made on the host with `app-suite admin`, the way back in when nobody can sign in, and the owner accounts the env file names can't be removed by anyone.
- 2026-11-27 — Gpose booths: the gpose types are one site-wide list, while each booth sets which it offers and each one's price and length; a booth is optionally filed under a festival and pinned to a stall, as a raffle is; a timeslot booth offers start times on a step the booth sets, and its waitlist is open at any time; a booth sets how many sign-ups one character may have waiting (one by default); only staff cancel a sign-up; staff record each one paid; sign-ups are made on the website only, while the plugin works them.
- 2026-11-27 — A gpose sign-up alerts its visitor (a chime, a vibration on a phone, the tab's title flashing, a banner) until they dismiss it: a timeslot booking on the festival dashboard from 10 minutes before its slot, and a staff call from the admin page or the plugin, on the dashboard and the booth's page.
- 2026-11-27 — Every environment variable the app reads carries the `APPSUITE_` prefix, as in the owner's other apps: Discord sign-in's are `APPSUITE_DISCORD_APPLICATION_ID` and `APPSUITE_DISCORD_CLIENT_SECRET`, and the names without the prefix are not read.
- 2026-11-28 — Staff may upload a completed gpose sign-up's gpose for the visitor to view and download on their festival dashboard and the booth's page; it is kept 24 hours from the upload, then deleted, and left out of the uploads backup so no copy outlives that day.
