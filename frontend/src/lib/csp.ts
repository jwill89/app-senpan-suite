/**
 * The nonce the server stamps into index.html on every response (vite.config.ts places
 * the placeholder in a `<meta property="csp-nonce">`). Anything that adds a `<style>` or
 * `<script>` element at run time sets it, or the page's Content-Security-Policy refuses
 * the element. Empty under the Vite dev server, which sends no policy.
 */
export const cspNonce =
  document.querySelector<HTMLMetaElement>('meta[property="csp-nonce"]')?.nonce ?? ''
