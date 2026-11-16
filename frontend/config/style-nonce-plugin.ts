import type { Plugin } from 'vite'

// @ckpack/vue-color adds its stylesheet as a <style> element when its module loads,
// through one small helper (rollup-plugin-postcss's style-inject). The page's
// Content-Security-Policy admits a <style> element only when it carries the page's
// nonce, so the build rewrites that helper to give the element the nonce the app reads
// from its csp-nonce meta (src/lib/csp.ts). The dev server sends no policy and serves
// the helper as it is.
const STYLE_INJECT = /[\\/]@ckpack[\\/]vue-color[\\/]libs[\\/]style-inject\.[^\\/]+\.js$/
const CREATE_STYLE = "var style = document.createElement('style');"

export function vueColorStyleNonce(): Plugin {
  return {
    name: 'vue-color-style-nonce',
    apply: 'build',
    transform(code, id) {
      if (!STYLE_INJECT.test(id.split('?', 1)[0] ?? id)) {
        return null
      }
      if (!code.includes(CREATE_STYLE)) {
        this.error(
          `${id} no longer creates its <style> element as "${CREATE_STYLE}", so its ` +
            'stylesheet would be refused by the page policy without a nonce',
        )
      }
      return {
        code:
          "import { cspNonce } from '@/lib/csp';\n" +
          code.replace(CREATE_STYLE, `${CREATE_STYLE} style.nonce = cspNonce;`),
        map: null,
      }
    },
  }
}
