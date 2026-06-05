/**
 * Resolve a stored prize-image path (e.g. "assets/images/raffles/raffle_1.png")
 * to a browser URL. Uploaded images live in the Apache web root and are served
 * at the site root in production. In dev these 404 (the Go API server doesn't
 * serve them), so callers should hide broken images with an @error handler.
 */
export function raffleImageSrc(path: string): string {
  if (!path) return ''
  // Absolute URLs or already-rooted paths pass through unchanged.
  if (/^https?:\/\//.test(path) || path.startsWith('/')) return path
  return `/${path}`
}
