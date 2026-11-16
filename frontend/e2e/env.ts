import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'

/**
 * Where the end-to-end run serves the suite: the app as production runs it, the Go
 * binary serving the API and the SPA it embeds. It is built from this checkout (after
 * the front end's build, which it embeds) into the OS temp folder and runs there on a
 * throwaway database and data folder, on its own port, with every Discord post held back.
 */
export const PORT = 8184
export const SITE_URL = `http://127.0.0.1:${String(PORT)}`

export const ROOT = resolve(import.meta.dirname, '../..')
export const DATA_DIR = join(tmpdir(), 'senpan-e2e')
export const DATABASE = join(DATA_DIR, 'database.sqlite')
/** The server's own data folder (`-data`): the uploads and the plugin repository it serves. */
export const SERVER_DATA = join(DATA_DIR, 'data')
export const LOG_FILE = join(DATA_DIR, 'senpan.log')
export const SERVER = join(DATA_DIR, process.platform === 'win32' ? 'app-suite.exe' : 'app-suite')

/**
 * The run's staff account, which signs in to seed the rest: an owner and an admin, made on
 * the throwaway database with the host's own command (`app-suite admin -password-stdin`),
 * as a host's first admin is made.
 */
export const STAFF = { username: 'e2e-owner', password: 'e2e owner password' }

/**
 * The flags `task dev:backend` runs the API with, every path the run's own. The server's
 * defaults point at the production layout, and devdata/ is a copy of the live data, so
 * each one is given. `-webhook-dry-run` logs every Discord post instead of sending it.
 */
export const serverFlags: Record<string, string> = {
  '-addr': `127.0.0.1:${String(PORT)}`,
  '-db': DATABASE,
  '-data': SERVER_DATA,
  '-log-file': LOG_FILE,
}

/**
 * The environment the API starts with, over the one it is started from: each setting
 * main.go falls back to an environment variable for is set here, so nothing in the
 * shell can turn on Turnstile, open CORS or redirect a webhook to a real channel.
 */
export const serverEnv: Record<string, string> = {
  APPSUITE_TURNSTILE_SECRET: '',
  APPSUITE_TURNSTILE_SITEKEY: '',
  APPSUITE_CORS_ORIGINS: '',
  APPSUITE_WEBHOOK_OVERRIDE: '',
  APPSUITE_WEBHOOK_DRY_RUN: '1',
  APPSUITE_OWNERS: STAFF.username,
  // The policy is enforced, so the run's pages are held to it.
  APPSUITE_CSP_REPORT_ONLY: '',
}
