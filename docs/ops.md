# Deployment and operations

The runbook for the live site. The app is one Go binary that serves everything: the API, its live event streams, the SPA it embeds, and the public files of its data folder (the image library, the Carrd projects, the plugin repository and the sound effects). Apache terminates TLS and proxies every path of every name to it; nothing is served from a document root. The `deploy/` folder holds what the host runs beside the binary: the systemd unit (`senpan.service`) and the Apache vhost (`apache-vhost.conf`), both installed by every app deploy with the host's values filled in from the deploy config, and the `images/` a new host's data folder starts from.

The host's details are settings of the git-ignored `scripts/deploy.config.ps1` (copied from `scripts/deploy.config.example.ps1`), and this guide names them by their placeholders: `<service>` is the `ServiceName` setting, `<service user>` the `ServiceUser`, `<opt dir>` the `RemoteOptDir` (the binary's folder, with the data folder `<opt dir>/data` inside it), `<cert>` the `CertName`, and `<env file>` the `EnvFile` (`/etc/<service>/<service>.env` unless set). The app listens on the loopback address at the port the `AppPort` setting names, which is written nowhere else.

## The data folder

```text
<opt dir>/data/                <- the unit's -data, beside the database
+-- database.sqlite            <- the -db (and its -wal/-shm)
+-- backups/                   <- the daily database copies (see Backups)
+-- images/                    <- PERSISTENT: served at /images/ (from deploy/images/ on a new host)
|   +-- logo.png, favicon.png, apple-touch-icon.png, share_banner.png
|   +-- pwa-192x192.png, pwa-512x512.png, pwa-maskable-512x512.png   <- the PWA's icons
|   +-- raffles/, announcements_main/, announcements_thumb/, garapons/, affiliate_logos/,
|   |   affiliate_images/, stamp_cards/, stamp_stamps/, stamp_prizes/, flourishes/
|   |                          <- the seeded image categories (System -> Images), and any added
|   +-- .categories.json       <- the categories' manifest (never served)
|   +-- announcements/         <- legacy announcement images; copied into announcements_main on first start
|   +-- bookclub/              <- reading-list cover uploads (reserved: not an image category)
|   \-- ema_plaques/           <- ema plaques awaiting review (reserved: not an image category)
+-- fonts/                     <- PERSISTENT: uploaded fonts, never served as files (see Font host)
|   \-- .woff2/                <- the converted WOFF2 copies, rebuilt from the uploads at start
+-- carrd/                     <- PERSISTENT: the Carrd projects, served at carrd.senpan.cafe (see Carrd image host)
+-- google-fonts/              <- the Google Fonts picked for headings and the ema designer, kept by the app (css/, files/); a cache it refills
+-- plugin/                    <- the Dalamud custom repo, replaced whole by each plugin publish (previous kept as plugin.old)
\-- sounds/                    <- the game's sound effects, copied in by hand (see below); served at /sounds/
```

The app serves `/images/<path>` (the library's picture types: jpg, png, webp, gif, svg), `/carrd/<path>` (those and mp3, mp4), `/plugin/pluginmaster.json` and `/plugin/SenpanCompanionAdmin/latest.zip`, and `/sounds/<name>.mp3`, from the data folder (`internal/media`, `serve.go`). It never serves a dotfile (the manifest, a project's `.carrd.json`), never lists a folder, never follows a link out of one, and has no route that reads `fonts/`. Pictures, Carrd files and sounds are cached a day (`public, max-age=86400`): a picture replaced under its name can show the old one until then unless purged at Cloudflare. The plugin's two files are `no-store`. A missing file is a 404 that nothing caches.

A deploy never touches the data folder (the plugin publish swaps only `plugin/`). The build strips its own `images/` and `sounds/`, so the binary never carries a copy of either.

**The sounds** (`level_up`, `moogle_noise`, `queue_pop` and `this_is_bad`, each `.mp3`) are the player's "game" sounds and the Yoever reaction (`frontend/src/lib/sound.ts`). They are kept out of the repository (`frontend/public/sounds/` is git-ignored, and holds them for `vite dev`), so no release carries them. Copy them into the data folder once, by hand, from a machine that has them:

```powershell
plink -i "<KeyPath>" <user>@<host> "install -d -m 755 <opt dir>/data/sounds"
pscp -i "<KeyPath>" frontend\public\sounds\*.mp3 <user>@<host>:<opt dir>/data/sounds/
```

Readable is enough: the app never writes them. Until they are there, `/sounds/*.mp3` answers 404 and the sounds stay silent; nothing else breaks.

## The vhost

`apache-vhost.conf` holds the three names' VirtualHosts, with the app's address (`127.0.0.1` and the `AppPort` setting) and the certificate's name filled in; every app deploy installs it as `/etc/apache2/sites-available/<service>.conf` where it differs. That one file holds both ports, and the deploy enables it, disables any other enabled site that serves its names (certbot's `-le-ssl` file, say), tries it on a copy of Apache's config and checks it with `apachectl configtest` before anything stops. A site that also serves a name the vhost does not, or a vhost that drops a port those names were served on, stops the run with nothing changed. The previous file is kept as `.old` and everything is put back if Apache refuses it or the release does not come up; Apache reloads once the new release answers. Certbot never edits the file. Edit it in the repo, never on the host.

| Name | Proxied to |
| --- | --- |
| `apps.senpan.cafe` | the app, every path (`timeout=3600` for the event streams) |
| `fonts.senpan.cafe` | `/api/v1/fonts/pub/`: `kit.css` and `/f/<token>` |
| `carrd.senpan.cafe` | `/carrd/`: `<project>/.../<file>` |

Port 80 redirects all three to https. All three use the certificate the `CertName` setting names (`/etc/letsencrypt/live/<cert>/`). Each port-443 site sends HSTS (`Header always set Strict-Transport-Security "max-age=31536000"`; the app sends none itself, so the header goes out once) and takes the visitor's address from Cloudflare: `RemoteIPHeader CF-Connecting-IP` with `RemoteIPTrustedProxyList /etc/apache2/cloudflare-ips.txt`, so the header is believed only from Cloudflare's ranges. The proxy then appends that address to `X-Forwarded-For`, and the app reads the last entry, from the loopback peer alone; it never reads `CF-Connecting-IP` itself. `mod_deflate` is kept off the event streams (`SetEnvIfNoCase Request_URI "/events$" no-gzip`), or their events would arrive in batches. Apache needs `mod_proxy`, `mod_proxy_http`, `mod_headers`, `mod_rewrite`, `mod_setenvif`, `mod_ssl` and `mod_remoteip`, and the ranges file, which the host setup below writes; refresh it when Cloudflare changes its ranges.

## Host setup

For a new host; the live one has all of it. It records nothing the deploy config doesn't already hold: the settings above name the account, the folders, the certificate and the env file, and `AppPort` the port.

**Memory.** The unit caps the service at `MemoryMax=384M`, with `GOMEMLIMIT=320MiB` so Go's collector holds the heap under it. Measured on 2026-11-15: the service's peak was 83.6 MiB over the 2.7 days since it started (`Get-ServiceMemory`; twice that is 176M), and checking and re-encoding a 40-megapixel picture, the largest an upload may be, holds 161 MiB more at once (the app's own `CheckImage`, measured on the dev machine), so the service's worst case is about 245 MiB. The cap is set from that worst case rather than the quiet peak, which a large upload would pass (AGENTS.md, Departures). The droplet has 2 GB. Measure again when the service's work changes, before a deploy restarts it.

Run this as root on the host, with the four values at the top filled in. It is safe to run again: it makes only what is missing, changes nothing that is there, and ends by saying what is left to do by hand.

```bash
#!/usr/bin/env bash
# One-time setup of a host for the Senpan App Suite, run as root. Safe to run again: it
# makes only what is missing, changes nothing that is there, and says what is left to do.
set -euo pipefail

SERVICE='<service>'                     # the ServiceName setting
SERVICE_USER='<service user>'           # the ServiceUser setting
OPT='<opt dir>'                         # the RemoteOptDir setting
CERT='<cert>'                           # the CertName setting
ENV_FILE="/etc/$SERVICE/$SERVICE.env"   # the EnvFile setting, when it names another path
NAMES='apps.senpan.cafe fonts.senpan.cafe carrd.senpan.cafe'

todo=0
note() { printf '  - %s\n' "$*"; todo=1; }

# What the deploy runs on the host: the database snapshot and pull, its checks, the vhost.
missing=''
for tool in sqlite3 curl openssl; do command -v "$tool" >/dev/null || missing="$missing $tool"; done
[ -z "$missing" ] || { apt-get update -qq && apt-get install -y -qq $missing; }
command -v apache2ctl >/dev/null || { echo "Apache is not installed: install apache2 first." >&2; exit 1; }

# The service's own locked account, with no home and no shell.
if ! getent passwd "$SERVICE_USER" >/dev/null; then
  useradd --system --user-group --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin "$SERVICE_USER"
  echo "Made the account $SERVICE_USER."
fi

# The binary's folder and the data folder, the account's own; the folders the app serves
# from and writes to, inside the data folder.
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 755 "$OPT" "$OPT/data"
for dir in images fonts carrd sounds; do
  [ -d "$OPT/data/$dir" ] || install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 755 "$OPT/data/$dir"
done
[ -e "$OPT/data/images/logo.png" ] || note "copy the repo's deploy/images/ into $OPT/data/images/, owned by $SERVICE_USER (docs/ops.md, The data folder)"
ls "$OPT/data/sounds/"*.mp3 >/dev/null 2>&1 || note "copy the four sound effects into $OPT/data/sounds/ (docs/ops.md, The data folder)"

# The env file: root's alone, made with the names it holds and a fresh session secret;
# the other values are typed in on the host.
install -d -o root -g root -m 755 "$(dirname "$ENV_FILE")"
if [ ! -e "$ENV_FILE" ]; then
  ( umask 077
    { printf 'APPSUITE_SESSION_SECRET=%s\n' "$(openssl rand -hex 32)"
      printf '%s=\n' APPSUITE_OWNERS APPSUITE_TURNSTILE_SECRET APPSUITE_TURNSTILE_SITEKEY APPSUITE_DISCORD_APPLICATION_ID APPSUITE_DISCORD_CLIENT_SECRET
    } > "$ENV_FILE" )
  echo "Made $ENV_FILE."
fi
chown root:root "$ENV_FILE"
chmod 600 "$ENV_FILE"
for name in APPSUITE_OWNERS APPSUITE_TURNSTILE_SECRET APPSUITE_TURNSTILE_SITEKEY APPSUITE_DISCORD_APPLICATION_ID APPSUITE_DISCORD_CLIENT_SECRET; do
  grep -q "^$name=." "$ENV_FILE" || note "fill in $name in $ENV_FILE (docs/ops.md, The env file)"
done

# Apache: the modules the vhost uses, and Cloudflare's ranges, which mod_remoteip trusts.
a2enmod -q proxy proxy_http headers rewrite setenvif ssl remoteip >/dev/null
if [ ! -s /etc/apache2/cloudflare-ips.txt ]; then
  ranges="$(curl -fsS https://www.cloudflare.com/ips-v4; echo; curl -fsS https://www.cloudflare.com/ips-v6; echo)"
  printf '%s\n' "$ranges" | grep -E '^[0-9a-fA-F:.]+/[0-9]+$' > /etc/apache2/cloudflare-ips.txt.new
  [ -s /etc/apache2/cloudflare-ips.txt.new ] || { echo "Could not read Cloudflare's ranges." >&2; exit 1; }
  mv /etc/apache2/cloudflare-ips.txt.new /etc/apache2/cloudflare-ips.txt
  echo "Wrote /etc/apache2/cloudflare-ips.txt."
fi

# The certificate: the host's shared one, which must cover every name the vhost serves.
pem="/etc/letsencrypt/live/$CERT/fullchain.pem"
if [ ! -r "$pem" ]; then
  note "issue the certificate $CERT for $NAMES with the host's certbot DNS setup, then run this again"
else
  for name in $NAMES; do
    openssl x509 -in "$pem" -noout -checkhost "$name" | grep -q 'does match' || note "the certificate $CERT does not cover $name"
  done
fi

# DNS: each name is a proxied record in Cloudflare pointing at this host.
for name in $NAMES; do
  getent ahosts "$name" >/dev/null || note "add $name in Cloudflare's DNS, proxied, pointing at this host"
done

if [ "$todo" -eq 0 ]; then
  echo "The host is ready: the first app deploy installs the unit, the vhost and the backups."
else
  echo "Do what each line above marked - says, then run this again."
fi
```

What it leaves to do by hand, when it says so:

1. **The images.** Copy the repo's `deploy/images/` into `<opt dir>/data/images/`, owned by `<service user>` (`pscp -r`, then `chown -R`).
2. **The sounds,** as [The data folder](#the-data-folder) says.
3. **The env file's values,** typed on the host (see [The env file](#the-env-file)); the script makes the session secret itself. `APPSUITE_OWNERS` is the username the first admin will have, chosen now.
4. **The certificate,** when the host's shared one is missing or doesn't cover the three names: issue or expand it with the host's certbot DNS setup (it renews by DNS, so certbot never edits a vhost).
5. **DNS:** each of the three names is a proxied record in Cloudflare, pointing at the host.

Then fill in `scripts/deploy.config.ps1` and run `task deploy`: the first app deploy installs and enables the unit, the vhost and the backup timer, installs the binary and starts it. The deploy needs no unit or vhost on the host before it.

**The first admin.** A new database has no account, so make the one `APPSUITE_OWNERS` names, on the host, as the service's account (so the database's files stay its own):

```bash
sudo -u <service user> <opt dir>/app-suite admin -db <opt dir>/data/database.sqlite <username>
```

It prints a password once. Sign in at `/admin/login` with it and change it at once (the topbar's Change Password). `-password-stdin` reads the password from standard input instead, and prints none. The command runs the migrations first and waits its turn beside the running service; it exits 2 on a usage error.

## The env file

The unit holds no secret. The service's secrets are the variables of `<env file>`, which the unit loads with `EnvironmentFile=` (no leading `-`, so a missing file stops the start rather than starting the app with the Turnstile bot check off). The file is `root:root` mode `0600`, in a folder only root writes, and the deploy stops before it builds or changes anything if the file is missing or any other mode. It prints the variables' names that it finds there, never their values, and warns when `APPSUITE_TURNSTILE_SECRET` or `APPSUITE_OWNERS` is not among them.

```text
APPSUITE_SESSION_SECRET=<a long random string>
APPSUITE_OWNERS=<the owner accounts' usernames, comma-separated>
APPSUITE_TURNSTILE_SECRET=<Cloudflare Turnstile secret key>
APPSUITE_TURNSTILE_SITEKEY=<Cloudflare Turnstile public site key>
APPSUITE_DISCORD_APPLICATION_ID=<the Discord application's id, its OAuth client id>
APPSUITE_DISCORD_CLIENT_SECRET=<the Discord application's OAuth client secret>
```

Every variable the app reads carries the `APPSUITE_` prefix, the Discord ones included; a name without it is not read. Signing in with Discord is on when both `APPSUITE_DISCORD_*` variables are set, and the startup log says which (`Discord sign-in enabled`, or a warning that it is off). The application must list the sign-in's redirect URL, `https://apps.senpan.cafe/api/v1/auth/discord/callback`, under OAuth2 -> Redirects in the Discord Developer Portal, exactly; a dev server behind Vite's proxy needs `http://localhost:8080/api/v1/auth/discord/callback` there too. The file may also hold the application's `APPSUITE_DISCORD_PUBLIC_KEY` and the bot's `APPSUITE_DISCORD_BOT_TOKEN`, which nothing reads yet.

`APPSUITE_OWNERS` names the owner accounts (the `-owners` flag is the same), exactly as they are spelled: nobody deactivates, demotes, renames or deletes one, or sets its password from the Users page, itself included, and only an owner makes a Discord link to itself. It is a name, not a secret, but it lives here with the rest of the service's settings. The startup log names the owners, or warns that none are named.

The host setup makes the file with a fresh session secret and the other names empty. To make or edit it by hand, as root, so the values are typed there and nowhere else (`openssl rand -hex 32` makes a session secret): `install -d -m 755 /etc/<service>`, then `( umask 077; cat > <env file> )`, paste the lines and end with Ctrl-D, then `chown root:root <env file> && chmod 600 <env file>`. Two optional variables go in the same file: `APPSUITE_CORS_ORIGINS`, for cross-origin browser clients (normally unset: the app is same-origin), and `APPSUITE_CSP_REPORT_ONLY=1`, which sends the Content-Security-Policy as report-only, so a page production blocks works again while the cause is fixed (see [Production-only checks](#production-only-checks)). A change to the file takes effect on `systemctl restart <service>`.

## Running as a non-root user

The service runs as its own locked system account, the `ServiceUser` setting, in the unit's `User=` and `Group=`. Nothing in the server needs root: it binds a loopback port (`-addr 127.0.0.1:<AppPort>`) and only writes inside its data folder, so a dedicated account shrinks the blast radius of any compromise. The binary belongs to that account too, and the daily backups run as it.

`deploy/senpan.service` is the unit. Every app deploy installs it as `/etc/systemd/system/<service>.service` where it differs, keeps the previous as `<opt dir>/<service>.service.old` beside the binary, and puts it back if the new build does not come up. Nobody edits the unit on the host: a change goes into the template and ships with the next deploy. A drop-in in `<service>.service.d/` overrides the template, so the deploy names any it finds.

Everything the process writes is in the data folder, `<opt dir>/data`, owned by `<service user>`, and `ReadWritePaths=<opt dir>/data` is the only writable path the unit grants (the log, in `/var/log/<service>`, is `LogsDirectory`'s). The binary in `<opt dir>` is the service's too, but read-only to the running process. Verify:

```bash
systemctl show <service> -p User -p Group -p ReadWritePaths   # <service user> / <service user> / <opt dir>/data
ps -o user= -C app-suite                                       # not root
# Then upload an image, a font (fonts/.woff2/ gets its copy) and a Carrd file from the
# admin UI, and check that /var/log/<service>/<service>.log keeps growing.
```

**The sandbox.** The unit carries the shared hardening: `NoNewPrivileges`, `ProtectSystem=strict` + `ProtectHome`, `PrivateTmp`/`PrivateDevices`, `ProtectKernel*`/`ProtectControlGroups`, `RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX`, `RestrictNamespaces`, `LockPersonality`, `SystemCallFilter=@system-service` + `SystemCallArchitectures=native`, and `MemoryDenyWriteExecute=yes` (W^X). A path written outside `ReadWritePaths` fails with `EACCES` under `ProtectSystem=strict`.

W^X is safe because the binary is pure Go (no cgo, `CGO_ENABLED=0`) and so is its SQLite driver: `modernc.org/sqlite` is SQLite's C translated to Go ahead of time, compiled into the binary like the rest of it, with no run-time code generation. If the backend ever takes a driver that generates code at run time (a WebAssembly build of SQLite under a JIT, say), `MemoryDenyWriteExecute=yes` crashes the service with `SIGSYS` on the first query and must go. Check it with `systemctl show <service> -p MemoryDenyWriteExecute -p SystemCallArchitectures`.

## Credential rotation

- **Admin and staff passwords.** Rotate the admin password to a fresh high-entropy value through the app's change-password flow, and any staff accounts that predate a known exposure. A password change or reset signs that account's other sessions out (each session records the account's password epoch) and revokes its personal access token. An owner changes its own password with Change Password; nobody else can set it.
- **When nobody can sign in** (the owner's password is lost, or every admin is locked out): run the [host command](#host-setup) for the account, `sudo -u <service user> <opt dir>/app-suite admin -db <opt dir>/data/database.sqlite <username>`. On an existing account it activates it, makes it an admin and sets the new password it prints, signing out every session it had and revoking its access token; its permissions, passkeys and Discord link stay.
- **Sessions** are kept in the database (the `sessions` table, under a hash of each token), so a restart signs nobody out. To sign every browser out at once: `systemctl stop <service> && sqlite3 <opt dir>/data/database.sqlite 'DELETE FROM sessions' && systemctl start <service>`. The FFXIV plugin signs in with its personal access token, not a session, so it is unaffected.
- **`APPSUITE_SESSION_SECRET` - keep it set and unchanged.** This version reads nothing from it, and the `-secret` flag is gone (a unit still passing it fails to start). The Discord bot release seals the bot's token under a key derived from it, so changing or removing it after that release would leave the stored token unreadable.
- **`APPSUITE_TURNSTILE_SECRET` / `APPSUITE_TURNSTILE_SITEKEY`.** The Cloudflare Turnstile pair (also settable as `-turnstile-secret`/`-turnstile-sitekey`) turns on the bot check on the public POST paths: sign-in, registration, raffle sign-up, custom card requests, stamp-rally sign-up and ema submissions. With the secret empty the check is off entirely (the server logs `Turnstile not configured` at start and the page skips the widget), so a bad or rotated secret degrades to no bot check, not a lockout. The secret is sensitive; the site key is public (served at `GET /api/v1/config`). Both are variables of [the env file](#the-env-file), never an `Environment=` line of the unit (visible through `systemctl show` and `/proc/<pid>/environ`). Rotate the secret in the Cloudflare dashboard, update the env file and restart the service.

## Font host (protected serving)

Uploaded fonts are licensed assets, so they are **never served as files**: they live in the data folder's `fonts/`, which no vhost names and no route of the app reads. The server streams them itself through obfuscated token URLs that rotate every 1-2 weeks, gated by the requesting site's `Origin` against **that font's own allowlist** (Atelier -> Font Upload -> Edit -> Allowed sites). Chrome and Firefox require CORS on cross-origin `@font-face`, so a site that isn't on a font's list cannot render it. Safari is the exception: it fetches fonts in `no-cors` mode, sending **no** `Origin`, so for an Origin-less request the gate checks the site in the `Referer` (the embedding page) against the same allowlist. Requests naming no site at all (pasting the URL into a browser, `wget`, casual scrapers) get a 403, and `kit.css` only emits each requesting site's allowed fonts. The SPA itself loads fonts same-origin (`/api/v1/fonts/pub/f/<token>`), so the app and its font picker always work; the allowlists only govern external sites.

External (Carrd) sites embed one stylesheet whose URL never changes:

```html
<link rel="stylesheet" href="https://fonts.senpan.cafe/kit.css">
```

and then use `font-family: '<CSS Name>'` (shown per font on the Font Upload tab; the file name without its extension unless changed). The kit's tokenized font URLs refresh by themselves on rotation. `fonts.senpan.cafe` is the vhost's proxy onto `/api/v1/fonts/pub/`, so `https://fonts.senpan.cafe/kit.css` is `/api/v1/fonts/pub/kit.css` and `https://fonts.senpan.cafe/f/<token>` is `/api/v1/fonts/pub/f/<token>`.

Files sharing a base name (`Jasper.otf` and `Jasper.woff2`) are format variants of **one font**. A font with no uploaded WOFF2 gets one **converted** (stored in the hidden `fonts/.woff2/` folder; a start-up backfill converts fonts that lack one and sweeps stale copies). The WOFF2 is served by default; the uploads stay on disk untouched, and the served version is selectable per font in the Edit dialog.

Checking it, from any machine:

```bash
# The gated responses must carry Cache-Control: private, or Cloudflare (which ignores
# Vary) can serve one site's kit to another.
curl -sI https://fonts.senpan.cafe/kit.css | grep -i cache-control                      # -> private
curl -s -H 'Referer: https://<allowed-site>/' https://fonts.senpan.cafe/kit.css          # -> that site's fonts
curl -s -o /dev/null -w '%{http_code}\n' -H 'Origin: https://<allowed-site>' https://fonts.senpan.cafe/f/<token>   # -> 200
curl -s -o /dev/null -w '%{http_code}\n' -H 'Referer: https://<allowed-site>/' https://fonts.senpan.cafe/f/<token> # -> 200 (Safari)
curl -s -o /dev/null -w '%{http_code}\n' -H 'Origin: https://elsewhere.example' https://fonts.senpan.cafe/f/<token> # -> 403
curl -s -o /dev/null -w '%{http_code}\n' https://fonts.senpan.cafe/f/<token>             # -> 403 (no site named)
curl -s -o /dev/null -w '%{http_code}\n' 'https://apps.senpan.cafe/fonts/<File>.ttf'     # -> 404 (never a file)
```

## Carrd image host

The **Atelier -> Carrd Upload** admin tab makes per-project folders under the data folder's `carrd/` (each of which may hold nested folders) and writes uploaded pictures, `.mp3` and `.mp4` files to them. They are served at `https://carrd.senpan.cafe/<project>/.../<file>`, the vhost's proxy onto the app's `/carrd/`, and embedded by external Carrd sites, so every file goes out with `Access-Control-Allow-Origin: *` and a day's cache; audio and video answer range requests. A project's `.carrd.json` (its title) is never served, and neither is a folder listing. A file uploaded under a name already there replaces it, so a site referencing its address picks up the new one (within the day's cache, or at once after a Cloudflare purge).

## Plugin repository

The FFXIV plugin's custom Dalamud repository is `https://apps.senpan.cafe/plugin/pluginmaster.json`, whose download links are `https://apps.senpan.cafe/plugin/SenpanCompanionAdmin/latest.zip?v=<version>`. The app serves those two files from the data folder's `plugin/`, read on every request and sent `no-store`: Cloudflare would otherwise keep an old `latest.zip` while the index already names the new version, and Dalamud refuses a package whose version differs from the index's. The `?v=` on the links is the second guard. `task deploy:plugin` publishes the folder (see [Each deploy](#each-deploy)).

## Backups

What lives only on the host is the database and the data folder's uploads; everything else is the release (the binary, the unit, the vhost), which the repo and its tags hold, or is rebuilt (`fonts/.woff2/` at start, `plugin/` by the plugin deploy, `google-fonts/` whenever a font is asked for again). The deploy installs every backup timer and checks it after the smoke check: shared-tooling's `Install-BackupTimer`, whose units and job scripts the deploy rewrites, so change them in `scripts/deploy.ps1`, never on the host. A new or changed timer runs once while the deploy waits; if that run fails, the previous units come back (or none are left) and the deploy fails, saying the site is live but its backup is not. The copies stay on the droplet; the copies that leave it are the dev-machine pulls of each deploy, below.

### The database

- **Units:** `<service>-backup.service` (oneshot) and `<service>-backup.timer`, with the job `/usr/local/libexec/shared-deploy/<service>-backup`.
- **When:** daily at 10:00 UTC, spread over up to 15 minutes; a missed run happens at the next boot.
- **As whom:** the service's own account, which owns the database.
- **What:** a `VACUUM INTO` copy of the live database, taken while the app keeps running (WAL included), checked with `PRAGMA integrity_check`, compressed with gzip and checked with `gzip -t` before it takes the day's name. No uncompressed copy is left behind.
- **Where:** `<opt dir>/data/backups/` (created `0700`, owned by the service's account), one file per UTC day, `<service>-YYYY-MM-DD.sqlite.gz`. A second run the same day replaces that day's copy. **28 days** are kept; the job removes only its own dated names.
- **Sandbox:** the shared backup units' hardening (`ProtectSystem=strict`, `UMask=0077`, no capabilities and the rest); the job may write only to the database's folder.
- **Off the droplet:** each app deploy pulls a fresh online copy into `devdata/database.sqlite` on the dev machine (step 7 of [Each deploy](#each-deploy)).

The backups never collide with the deploy's own files: the pre-deploy snapshots are `<db>.pre-<yyyyMMdd-HHmmss>.gz` in the database's own folder, pruned to the newest three by exactly that pattern, and the dev pull's `<db>.pull` lasts only for the pull. Neither pattern can match a dated name in `backups/`, and the backup's pruning cannot match theirs.

### The uploads

The data folder's `images/`, `fonts/` (less its `.woff2/` copies), `carrd/` and `sounds/` are host-only: no repo, deploy or database holds them. This project archives a host-only folder on the host up to **250 MB**, and copies a larger one to the dev machine instead (`Sync-LiveFolder`).

Measured on 2026-11-15 (`du`): `images/` 203 MB (111 files), `fonts/` 7.6 MB without its 4 MB of WOFF2 copies (73 files with them), `carrd/` 16.6 MB (79 files), `sounds/` 0.2 MB (4 files): about 233 MB together, so they are archived on the host. Measure again as festivals add pictures; once they pass 250 MB, the deploy moves to `Sync-LiveFolder` for them.

- **Units:** `<service>-uploads-backup.service` and `<service>-uploads-backup.timer`, with the job `/usr/local/libexec/shared-deploy/<service>-uploads-backup`, run as the service's account at the database backup's time.
- **What:** one `.tar.gz` of the data folder, its top entry `data/`, leaving out the live database and its `-wal`, `-shm`, snapshots and pull (`database.sqlite*`), `backups/`, `plugin/` and `plugin.old/` (the plugin deploy rebuilds them) and `fonts/.woff2/` (the app rebuilds it at start) and `google-fonts/` (the app fetches a family again when it is missing). Checked with gzip and a full `tar` listing before it takes the day's name.
- **Where:** `<opt dir>/uploads-backups/` (`0700`, the service's account's), one archive per UTC day, `<service>-uploads-YYYY-MM-DD.tar.gz`. **7 days** are kept, not 28: pictures barely compress, so each archive is about the folders' size, and seven are about 1.6 GB of the droplet's disk.
- **Off the droplet:** each app deploy (unless `-NoDbPull`) also pulls a fresh archive of each of the four folders into `devdata/uploads/<folder>/` on the dev machine, three of each kept (`Copy-LiveFolder`).

## Restore steps

Each starts by stopping the site's writes: `systemctl stop <service>`.

**The database, from a daily copy.** Decompressing in place, as the service's account, keeps the file's owner and mode:

```bash
systemctl stop <service> && rm -f <db>-wal <db>-shm && runuser -u <service user> -- sh -c 'gunzip -c <opt dir>/data/backups/<service>-YYYY-MM-DD.sqlite.gz > <db>' && systemctl start <service>
```

**The database, from a deploy's snapshot** (`<db>.pre-<time>.gz`, the service account's alone): the same, run as root, since `>` onto the existing file keeps its owner; [Rolling back](#rolling-back) has the line.

**The plugin repository** is the newest plugin release: `task deploy:plugin` publishes it again (with no index on the host to compare, the host's own version check stands).

**The sounds** are copied in again from a machine that has them ([The data folder](#the-data-folder)).

**The uploads,** from a day's archive. The archive's top entry is `data/`, holding only the four folders, so it unpacks over `<opt dir>`; the folders it replaces are moved aside first, and unpacking as root keeps every file's owner:

```bash
systemctl stop <service>
cd <opt dir>/data && for f in images fonts carrd sounds; do mv "$f" "$f.before-restore"; done
tar -xzf <opt dir>/uploads-backups/<service>-uploads-YYYY-MM-DD.tar.gz -C <opt dir>
systemctl start <service>   # the app makes fonts/.woff2/ again at start
```

Once the site checks out, remove the `*.before-restore` folders. One folder alone comes back the same way, naming it after the archive (`tar -xzf <archive> -C <opt dir> data/images`). The dev machine's archives in `devdata/uploads/<folder>/` (each one folder, its top entry the folder's name) unpack the same way once uploaded to the host (`tar -xzf <archive> -C <opt dir>/data`), then `chown -R <service user>:<service user>` on what they wrote.

The checks after a restore: `systemctl status <service>`, `curl -s http://127.0.0.1:<AppPort>/healthz` on the host (200, with the version), then sign in and open the pages whose data came back.

## Restore drill

No drill is run on the host (the owner's decision of 2026-11-15); these are the steps for when one is wanted. Last run on the host: never.

1. On the host, as root, restore the newest daily copy into a scratch file, never over the live database: `gunzip -c "$(ls -1t <opt dir>/data/backups/<service>-*.sqlite.gz | head -n 1)" > /tmp/drill.sqlite`.
2. `sqlite3 /tmp/drill.sqlite 'PRAGMA integrity_check'` answers `ok`, and `sqlite3 /tmp/drill.sqlite 'PRAGMA user_version'` names the live schema (`curl -s http://127.0.0.1:<AppPort>/healthz`).
3. Compare row counts with the live database, read-only: for each of `users`, `cards`, `raffles`, `raffle_entries`, `winners_log`, `announcements`, `festival_maps` and `stamp_rally_cards`, `sqlite3 /tmp/drill.sqlite "SELECT count(*) FROM <table>"` against `sqlite3 -readonly <db> "SELECT count(*) FROM <table>"`. A difference is only what changed since the copy's day.
4. Unpack the newest uploads archive into a scratch folder (`mkdir /tmp/drill-uploads && tar -xzf "$(ls -1t <opt dir>/uploads-backups/*.tar.gz | head -n 1)" -C /tmp/drill-uploads`), and compare `find /tmp/drill-uploads/data/<folder> -type f | wc -l` with the live folder's for `images`, `carrd` and `sounds`, and with `find <opt dir>/data/fonts -path '*/.woff2' -prune -o -type f -print | wc -l` for `fonts`. A difference is only what changed since the archive's day.
5. Remove the scratch files (`rm -rf /tmp/drill.sqlite /tmp/drill-uploads`), and write the date here.

## Production-only checks

What only the live site can show, checked after every deploy in a fresh browser tab with the console open:

- **The policy.** The home page, a raffle, a festival map, a board at `/play/<id>`, and in the admin a page with the text editor, System -> Themes (the header font) and System -> Images: no Content-Security-Policy refusal in the console. `curl -sI https://apps.senpan.cafe/` shows one `Strict-Transport-Security` and a `Content-Security-Policy` naming a `nonce-`, with no `'unsafe-inline'`. If production refuses something the build didn't, set `APPSUITE_CSP_REPORT_ONLY=1` in [the env file](#the-env-file), restart the service, then fix the cause and take the line out.
- **Signing in:** with a password (the Turnstile widget shows and passes), with a passkey (WebAuthn's relying party is the live host) and with Discord (the redirect URL is the live one).
- **The visitor's address:** System -> Logs names your own address on your requests, not a Cloudflare one.
- **The live streams through Cloudflare:** open a board, draw a number on the Game page, and see it arrive after the draw delay.
- **Fonts:** a Google font picked as the header font (System -> Settings) loads, and so does one picked in the ema designer; the service fetches each from Google once (`fonts.googleapis.com`, `fonts.gstatic.com`, outbound HTTPS) and the browser asks only the site.
- **Writes in the sandbox:** upload a picture in System -> Images, and see `/var/log/<service>/<service>.log` grow.
- **The map embed:** a page on a site listed under System -> Settings ("Sites that may show a festival map in a frame") shows a festival map's embed, and `curl -sI https://apps.senpan.cafe/embed/festival-maps/<id>` names those sites in `frame-ancestors` with no `X-Frame-Options`.
- **The outside names:** the font checks under [Font host](#font-host-protected-serving), a Carrd file at `carrd.senpan.cafe` with `Access-Control-Allow-Origin: *`, and the plugin index naming the version Dalamud offers.

## Logs

- **The app:** structured JSON to stdout, which journald keeps (`journalctl -u <service>`), and to a rotating file, `/var/log/<service>/<service>.log` (the unit's `-log-file`, in the folder its `LogsDirectory=` makes writable under `ProtectSystem=strict`; without that the app falls back to stdout alone and logs a `file logging disabled` warning). The file rotates daily at local midnight, its backups compressed with zstd and kept 14 days or 30 days or 100 MB, whichever ends first. The admin **System -> Logs** page reads the file (`GET /api/v1/logs`) and live-tails every new line over the staff event stream, so most reading needs no SSH; on the host, `jlv /var/log/<service>/<service>.log` (the Linux `.deb` from <https://github.com/hedhyw/json-log-viewer/releases>) reads it too.
- **The backups:** `journalctl -u <service>-backup.service -n 20`, and `systemctl list-timers '<service>-backup.timer'` for the next run.
- **Apache:** the vhost logs to Apache's own `/var/log/apache2/error.log` and `access.log`.

**Who made each request.** Every request line names its actor in an `auth` field (`session`, `token`, `bot` or `anon`), with `user` (the account) and `bot` (a crawler) when they apply. Admin requests resolve through the session and the plugin's through its personal access token, with nothing to configure; the System -> Logs page shows them in a User column. Capability tokens in paths, queries and the `Referer` are cut to a short correlation hash before logging, so draw, stamp and font links never appear in the log.

**Naming verified crawlers.** An anonymous request is labelled `bot` only when Cloudflare tells the origin it is a verified good bot (Googlebot, Bingbot, GPTBot and the like). Cloudflare sends no such signal by default; turn it on one of two ways:

- **Any plan:** a Request Header Transform Rule (the zone's Rules -> Transform Rules -> Modify Request Header -> Create rule), filtered on the `cf.client.bot` field (Verified Bot equals True), setting the static header `X-Verified-Bot` to `true`. The app then names the bot from its User-Agent.
- **Enterprise with Bot Management:** the "Add bot protection headers" managed transform; the app also reads its `cf-verified-bot` and `cf-verified-bot-category` headers.

It is a logging hint, never a security decision: like any header, a client that reaches the origin without going through Cloudflare can send it.

## Upgrading to 5.7.1

5.7.1 reads Discord sign-in's settings as `APPSUITE_DISCORD_APPLICATION_ID` and `APPSUITE_DISCORD_CLIENT_SECRET`, like every other variable in [the env file](#the-env-file), and no longer reads `DISCORD_APPLICATION_ID` or `DISCORD_CLIENT_SECRET`. Before the deploy, check the env file holds the prefixed names (`grep -c '^APPSUITE_DISCORD_APPLICATION_ID=.' <env file>` and the same for the secret, each 1), and rename any line still without the prefix. **The live env file already holds them:** they were renamed there before 5.7.0, which read the old names and so started with Discord sign-in off. After the deploy, `journalctl -u <service> -b | grep "Discord sign-in"` says it is enabled, and the sign-in page shows **Sign in with Discord** again.

## Upgrading to 5.5.0

5.5.0 sends HSTS and restores the visitor's address in Apache, enforces a Content-Security-Policy with a nonce, caps the service's memory, backs up the uploads, takes its port from the deploy config alone, and names its owner accounts in the env file in place of the protected `admin` account. In order:

1. **On the host, as root,** add `APPSUITE_OWNERS=<the live admin's username>` (several, comma-separated) and `APPSUITE_CSP_REPORT_ONLY=1` to [the env file](#the-env-file): the first keeps the live admin from being removed by another, as the `admin` account was, and the second has the new policy first only report what it would refuse; then turn on `mod_remoteip` and write Cloudflare's ranges: run the [host setup](#host-setup) script, which does both and changes nothing else on a host that has the rest. Without the module the deploy's `apachectl configtest` refuses the vhost and the run stops with nothing changed. **Done on the live host on 2026-11-16:** `APPSUITE_OWNERS=admin` and `APPSUITE_CSP_REPORT_ONLY=1` added (the env file's previous copy kept beside it as `.bak-20261116-221959`, `/etc/apache2` archived in `/root/apache2-before-senpan-5.5.0-20261116-221959.tar.gz`), and the host setup script found the host ready (`mod_remoteip` and Cloudflare's ranges were already there).
2. **In `scripts/deploy.config.ps1`,** set `$DeployAppPort` to the port the app listens on now (the `-addr` in `systemctl show <service> -p ExecStart`): the deploy no longer assumes one, and stops listing it until it is set. **Done on 2026-11-16.**
3. **Commit and push** to `main`; when CI is green it tags `v5.5.0`. Then **`task deploy`**. The unit now passes `-log-file` itself, to the same `/var/log/<service>/<service>.log`, and caps the service at `MemoryMax=384M`. After the database's backup the deploy installs the uploads' timer and waits for its first archive (about 230 MB, under a minute), then pulls each upload folder into `devdata/uploads/` (about 230 MB down; `-NoDbPull` skips it).
4. **The [production-only checks](#production-only-checks),** the visitor's address above all; `journalctl -u <service> | grep "owner accounts"` names the owners. Migration 78 clears the password of an account named `admin` that still has the password `admin`: if that was the way in, make it again with the [host command](#host-setup). Then `systemctl list-timers '<service>-uploads-backup.timer'` and `ls -l <opt dir>/uploads-backups/` on the host.
5. **The policy, report-only:** open the pages the checks name, and the admin pages that show pictures (Raffles, Announcements, Affiliates, each Book Club), in a fresh tab with the console open, and read every `[Report Only]` line. A picture whose address is on another host (a book-club cover pasted as a URL, an old image link) would be refused once the policy is enforced: upload it to the image library and pick it there, or note it for a code change. When nothing is reported, take `APPSUITE_CSP_REPORT_ONLY=1` out of the env file and `systemctl restart <service>`. **Done on 2026-11-17 (UTC):** 5.5.0 deployed; the public and admin pages reported nothing but six book-club covers linked from other sites, four of which were uploaded to the site and the items pointed at them (the other two, expired Discord attachments, need new covers); then the policy was enforced (the env file's previous copy kept as `.bak-20261117-020838`).

## Upgrading to 5.4.0

5.4.0 adds signing in with Discord, which reads `DISCORD_APPLICATION_ID` and `DISCORD_CLIENT_SECRET` from [the env file](#the-env-file) (on the live droplet since 2026-11-13; prefixed with `APPSUITE_` since 5.7.1). Before the deploy, in the Discord Developer Portal, add `https://apps.senpan.cafe/api/v1/auth/discord/callback` to the application's OAuth2 -> Redirects. Without it Discord refuses every sign-in with an invalid `redirect_uri`. The migration (v76) adds two tables and changes no row. After the deploy, `journalctl -u <service> | grep "Discord sign-in"` says it is enabled, and the sign-in page shows **Sign in with Discord**.

## Upgrading to 5.3.0

5.3.0's deploy installs the service's unit itself and the unit reads the service's secrets from [the env file](#the-env-file). The live unit held them as `Environment=` lines, so the file had to exist before the deploy. With `scripts/deploy.config.ps1` filled in (`ServiceUser` is new), in order:

1. **The env file** is on the live droplet since 2026-11-11, made from the unit's `Environment=` lines: `root:root` mode `0600`, holding the variables the unit set. On a host without it, make it as root; the shell copies the values and shows none of them:

   ```bash
   install -d -m 755 /etc/<service>
   ( umask 077; sed -n 's/^Environment=//p' /etc/systemd/system/<service>.service > <env file> )
   chown root:root <env file> && chmod 600 <env file>
   sed 's/=.*//' <env file>   # the variables' names, no values
   ```

   The names should be the variables the unit sets: `APPSUITE_SESSION_SECRET` and the `APPSUITE_TURNSTILE_SECRET` and `APPSUITE_TURNSTILE_SITEKEY` pair. A name with a quote before it, or two names on one line, means the unit quoted or joined its assignments; edit the file until each line reads `NAME=value`.
2. **Commit and push** to `main`. When CI is green it tags `v5.3.0`.
3. **Run `task deploy`.** Before it builds anything it stops if the env file is missing or not `root:root` mode `0600`, if the service's account or the data folder is missing, or if the unit still passes `-webroot`. Otherwise it installs the unit, the vhost and the binary (disabling any other enabled site that serves the vhost's names), and puts all three back, with the database snapshot, if the new build does not answer. The plugin step skips: its version is unchanged.
4. **Check on the host:**

   ```bash
   systemctl show <service> -p Environment -p EnvironmentFiles -p ExecStart -p ReadWritePaths   # no Environment=, the env file, -addr 127.0.0.1:<AppPort>, <opt dir>/data
   journalctl -u <service> -g Turnstile --no-pager | tail -n 3                                  # "bot check enabled", not "not configured"
   ```

   Then sign in at `/admin/login` (the Turnstile widget shows and the sign-in works) and upload a picture in System -> Images, so a write into the narrowed `ReadWritePaths` is proven.
5. **Clean up a few days later**, once 5.3.0 has stayed, on the host. `<opt dir>/<service>.service.old` is the previous unit with its secrets inline and readable by every local account; the next deploy removes it by itself when the unit has not changed, or remove it now with `rm`. The unit sets `LogsDirectory=` itself, so the drop-in the deploy warns about is redundant: `rm /etc/systemd/system/<service>.service.d/logdir.conf && rmdir /etc/systemd/system/<service>.service.d && systemctl daemon-reload`.

## Upgrading to 5.2.0

5.2.0 moved everything out of Apache's document root into the data folder (`<opt dir>/data`): `images/`, `fonts/`, `carrd/`, `plugin/` and, from the web root's `dist/sounds/`, `sounds/`. In the same first deploy it changed the unit's `-webroot` to `-data` and swapped the six apps, fonts and carrd site files for the one vhost. Every public URL stayed as it was, `fonts.senpan.cafe/kit.css` and its token links included. The move was `scripts/deploy.ps1` as of the `v5.2.0` tag; later deploys stop on a host whose unit still passes `-webroot`.

Once 5.2.0 has stayed, the old document root is read by nothing. Remove its `dist/`, `dist.old/`, `.htaccess`, `.htaccess.old`, `.deploy-backup/` and `plugin.old/`, move `themes.json` and `insert_styles.php` (`scripts/themes/`) to wherever they are run, and remove the folder; delete the six disabled site files from `sites-available`; remove the inert `fonts/.htaccess`, `carrd/.htaccess` and `plugin/.htaccess` from the data folder; and, when nothing needs the way back, the move's backups folder (`~/senpan-before-data-move-<UTC>/`) and `~/.shared-deploy/senpan-data-move/`.

## Each deploy

From the repo root, on Windows. Only releases ship: the app's newest `v<version>` tag CI made on main, the plugin's newest `Plugin-v<version>` (the tags are fetched first). Each is checked out into a folder of its own under `%TEMP%` and built there, with `npm ci` from its own lockfile, so commits after it, a shared-tooling update or anything uncommitted are never part of it. `task deploy` deploys the app, then the plugin with `-IfNewer`, which skips it when its version is already published. `task deploy:backend` (the app) and `task deploy:plugin` deploy one each, with flags after `--`. The SPA is inside the binary, so there is no `deploy:frontend`. The script itself:

```powershell
.\scripts\deploy.ps1                  # the app's newest release (skipped when the host already runs it)
.\scripts\deploy.ps1 -Force           # the same release again
.\scripts\deploy.ps1 -Target plugin   # the plugin's newest release, to the Dalamud custom repo
```

The script is built on shared-tooling's SharedDeploy module (`shared/deploy/pwsh/`, whose README describes each function), so it takes the shape every deploy there takes: one archive, one SSH connection held for the whole run, one remote run.

**Settings.** No host value is in the script or in the unit and vhost templates. Copy `scripts/deploy.config.example.ps1` to `scripts/deploy.config.ps1` (untracked) and fill in the eight required `$Deploy*` values. Each can instead be passed as a parameter or set through its `$env:DEPLOY_*` variable (`DEPLOY_VPS_HOST`, `DEPLOY_VPS_USER`, `DEPLOY_KEY_PATH`, `DEPLOY_SERVICE_NAME`, `DEPLOY_SERVICE_USER`, `DEPLOY_REMOTE_OPT_DIR`, `DEPLOY_CERT_NAME`, `DEPLOY_APP_PORT`); the script fails fast, listing any that are unset, before it builds or connects. `ServiceUser` is the account the service runs as, never root: the unit's `User=` and `Group=` and the owner of the binary. `CertName` is the certificate the vhost uses, its folder's name under `/etc/letsencrypt/live`, and `AppPort` the port the app listens on, on `127.0.0.1`; a plugin deploy needs neither. `EnvFile` (`DEPLOY_ENV_FILE`) is optional and is `/etc/<service>/<service>.env` unset. `HostKey` (`DEPLOY_HOST_KEY`) is the optional SSH host key fingerprint, passed to plink as `-hostkey`. The site's address, which the smoke check fetches, is public already (`plugins/pluginmaster.json` links to it), so it defaults to `https://apps.senpan.cafe`; `-SiteUrl`, `DEPLOY_SITE_URL` or `$DeploySiteUrl` overrides it. `-ConfigPath` points the script at another settings file. The live database and the data folder need no setting: they are `<opt dir>/data/database.sqlite` and `<opt dir>/data`, the `-db` and `-data` the unit template gives the app.

**What an app deploy does:**

1. **Check the host**, before anything is built or changed. The host says which release it last took (`~/.shared-deploy/<service>`): when that is this release, the run ends there, unless `-Force`. Otherwise one run asks the host that the service's account and the data folder are there, that [the env file](#the-env-file) is `root:root` mode `0600`, and that the unit does not pass `-webroot`. A failed check ends the run with its reason and nothing changed. The deploy prints the env file's variable names, never a value, warns when `APPSUITE_TURNSTILE_SECRET` (the sign-in bot check would be off) or `APPSUITE_OWNERS` (any admin could remove any other) is not among them, and names any drop-in on the unit.
2. **Build the release, in its own checkout.** The front end first (`npm run build` in `frontend/`: vue-tsc and vite, into `backend/internal/web/dist/`), then a static `linux/amd64` binary that embeds it (`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o app-suite .` in `backend/`). The release's `deploy/senpan.service` and `deploy/apache-vhost.conf` are filled in from the settings (the unit: the service's name, account, opt dir, address and env file; the vhost: the address and `CertName`); a template that lacks a placeholder, or has one nothing fills, stops the run. A broken build changes nothing. The binary, unit and vhost go into **one** `.tar.gz`: a truncated archive refuses to unpack, where a half-copied folder can look whole.
3. **Upload** the archive as one stream through plink, into the SSH user's home.
4. **One remote run.** The host checks the archive's SHA-256 and file count, and only then: installs the vhost as `/etc/apache2/sites-available/<service>.conf` where it differs, enabled and with any other enabled site serving its names disabled, after trying it on a copy of Apache's config and checking it with `apachectl configtest` (a refusal puts everything back and stops the run with nothing else changed); stops the service, keeps `app-suite.old`, snapshots the database with `sqlite3 .backup`, checked with `PRAGMA integrity_check`, compressed with gzip and checked with `gzip -t`, to `<db>.pre-<yyyyMMdd-HHmmss>.gz` (UTC; written as the database's own account and readable by it alone; the newest three are kept), installs the unit as `/etc/systemd/system/<service>.service` where it differs (the previous kept as `<opt dir>/<service>.service.old`, systemd reloaded, the service enabled), installs the binary, owned by the account the unit runs as, starts the service and waits up to 30 s for `/healthz` to answer on the host. If it does not, the previous binary, unit and vhost go back with the snapshot and restart: the new binary may already have migrated the database, and the previous one refuses a newer schema (`database schema vN is newer than this binary`). Once it answers, Apache reloads if the vhost changed. Brief downtime (a few seconds) while the service is stopped. The data folder is never touched.
5. **Smoke check from this machine, through Cloudflare**, each URL tried three times: `<SiteUrl>/healthz` and `<SiteUrl>/api/v1/version` must name this release's version; `<SiteUrl>/` must name the build's entry script, which must load too; `/images/pwa-512x512.png` and `/plugin/pluginmaster.json` must answer. If any fails, the previous binary, unit and vhost go back with the pre-deploy snapshot, so it starts on the schema it knows. Whatever was written in the seconds the new release ran is lost with it (decided 2026-11-06). Once the checks pass, the host records the release.
6. **The daily backups,** once the smoke check passes: see [Backups](#backups). If one cannot be installed, the run fails saying the site is live but its backup is not.
7. **The dev-machine pulls** (unless `-NoDbPull`): an online `sqlite3 .backup` on the host (the service keeps running), downloaded and checked against the host's SHA-256 into `devdata/database.sqlite`. The previous local copy is kept as `database.sqlite.bak-<time>` (the newest three), stale `-wal`/`-shm` files are removed, and the host's copy is deleted. While a local dev server holds the database open, the pull is skipped with a warning. Then a fresh archive of each upload folder (`images`, `fonts`, `carrd`, `sounds`) is made on the host, downloaded and checked into `devdata/uploads/<folder>/`, three of each kept, and the host's copy removed. These copies hold **live** data (accounts, password hashes, tokens, webhook URLs, every upload): keep them local.

The checkout under `%TEMP%` is removed at the end, whether the run passed or not.

**What a plugin deploy does.** It checks out the plugin's newest release, reads the published `<SiteUrl>/plugin/pluginmaster.json` and stops before building when the release's version is not higher, since Dalamud updates to nothing else (with `-IfNewer`, the same version ends the run without error; if the index can't be read, the host's own check still stands). Then `dotnet build -c Release` from clean (its `bin/` and `obj/` deleted first, so the package can never lag the manifest), and its repository laid out: `SenpanCompanionAdmin/latest.zip` and `plugins/pluginmaster.json` with `?v=<version>` on its download links. The built version must equal `pluginmaster.json`'s `AssemblyVersion`. The host is checked as in step 1, without the env file; then the whole `plugin/` folder of the data folder is swapped in, owned by the service's account (the previous kept as `plugin.old`), and the published index must then carry the new `?v=`, or the previous folder goes back.

**Options.** `-Force` deploys the app's release even when the host already runs it. `-NoBackup` removes `app-suite.old`, the previous unit and vhost and `plugin.old` once the host run succeeds; the smoke check then has nothing to go back to, so a failure only stops the run. The database snapshot is still taken. `-NoDbPull` skips step 7.

**The connection.** The droplet runs `ufw limit ssh`, which refuses an address after six connections in 30 s. The whole run is **one** SSH connection, held by a background `plink -share`, which every command rides. An upload or download of 1 MB or more (the archive, the database pull, the uploads archive) takes a connection of its own, many times faster than a channel of the shared one, and the module opens at most four connections in any 30 s. If the held one drops, the module reconnects (one new connection) and retries the step once, resuming where it stopped; only a refused reconnect waits 35 s.

**Requirements.**

- PowerShell 7.4 or later on Windows, with Windows' own `tar.exe` (shipped since Windows 10 1803), git, Node and Go.
- PuTTY's `plink` 0.85 or later (connection sharing), on PATH or in `C:\Program Files\PuTTY`. plink runs in batch mode, so a passphrase-protected `.ppk` must be loaded into Pageant first: run `pageant.exe "<KeyPath>"` once and enter the passphrase (it stays unlocked for the session; a `shell:startup` shortcut loads it at sign-in). The first connection from PuTTY also needs the host key cached once (`plink -i "<KeyPath>" <user>@<host>`, accept the fingerprint), or the `HostKey` setting.
- On the host: bash, systemd, GNU coreutils, `getent`, `curl` (the on-host checks), `awk` and `apachectl` (the vhost check) and `sqlite3` (the snapshot, the backup and the pull); Apache with the modules [the vhost](#the-vhost) needs. The host setup installs what is missing.
- For the plugin: the .NET 10 SDK.

**Manual fallback.**

1. Locally, at the release's tag: `cd frontend && npm ci && npm run build`, then build the binary in `backend/` as in step 2.
2. On the host: stop the service, install the binary over `app-suite`, start it, and check `/healthz`. When `deploy/senpan.service` changed, fill in its placeholders (the service's name, account, opt dir, address and env file), copy it over `/etc/systemd/system/<service>.service` before the start, and `systemctl daemon-reload`. When `deploy/apache-vhost.conf` changed, fill in its `__APP__` with the app's address and `__CERT__` with the certificate's name, copy it over `/etc/apache2/sites-available/<service>.conf`, and `apachectl configtest && systemctl reload apache2`.

## Rolling back

The deploy rolls itself back when the release doesn't come up or fails the smoke check. When that rollback didn't run (a problem found later) or didn't finish, put the previous release back by hand, on the host, as root, in the opt dir; in reverse order of the deploy, the plugin first:

- **The plugin**, in the data folder: `mv plugin plugin.failed && mv plugin.old plugin && rm -rf plugin.failed`.
- **The vhost:** `mv -f /etc/apache2/sites-available/<service>.conf.old /etc/apache2/sites-available/<service>.conf && apachectl configtest && systemctl reload apache2`, and `a2ensite` any site the deploy disabled, which it named.
- **The unit and the binary:** `systemctl stop <service> && mv -f <opt dir>/<service>.service.old /etc/systemd/system/<service>.service && systemctl daemon-reload && install -m 755 -o <service user> -g <service user> app-suite.old app-suite && systemctl reset-failed <service> && systemctl start <service>`. Leave the `mv` out when the deploy did not change the unit, which leaves no `.old`.
- **The database,** when the previous binary refuses it (`database schema vN is newer than this binary`): decompress the newest snapshot over it in place, which keeps the file's owner: `systemctl stop <service> && rm -f <db>-wal <db>-shm && gunzip -c "$(ls -1t <db>.pre-*.gz | head -n 1)" > <db> && systemctl start <service>`. Whatever was written since the snapshot is lost with it.

Then check `curl -s http://127.0.0.1:<AppPort>/healthz` on the host and the [production-only checks](#production-only-checks). A rollback by hand leaves the host's record naming the release it rolled back from, so the next deploy skips that release unless `-Force`.

## Notes

- Image URLs are root-absolute (`/images/...`), served by the app from the data folder's `images/`: the static art and the uploads alike.
- Existing databases are migrated (schema v10) to rewrite old `assets/images/raffles/...` prize paths to `images/raffles/...` on first start.
- **PWA:** the build also emits `sw.js`, `registerSW.js` and `manifest.webmanifest` at its root; the app serves them at the site's root, `sw.js` and `registerSW.js` with `no-store` and the manifest with `no-cache`, never the year-long immutable cache its hashed `assets/` get, so the app can update (`backend/internal/web`). The PWA icons (`pwa-192x192.png`, `pwa-512x512.png`, `pwa-maskable-512x512.png`) and the iOS `apple-touch-icon.png` are served from the data folder's `images/`; they ship in `deploy/images/`.
