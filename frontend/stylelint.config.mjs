// The shared CSS rules (`@jwill89/lint-web`), in the .vue files' style blocks as
// well. Prettier formats; this catches what formatting cannot (an unknown property, a
// rule shadowed by one before it, an @import browsers would ignore). The project's own
// CSS conventions - naming, the scoped-CSS ratchet, the theme tokens - are
// scripts/check-conventions.mjs. A rule is turned off here with its reason, or at the
// one line it does not fit with the reason beside it.
export default {
  extends: ['@jwill89/lint-web/stylelint'],
  rules: {
    // app.css imports its parts by string, which Vite resolves and bundles.
    'import-notation': 'string',
    // Class names are kebab-case: a part joined with `-`, a modifier after `--`, and
    // a reusable component's own classes BEM, `block__element--modifier` (which
    // blocks may take `__` is checked by scripts/check-conventions.mjs). ProseMirror,
    // whose classes the stylesheets reskin, keeps its own names.
    'selector-class-pattern': [
      '^(?:ProseMirror(?:-[a-z]+)*|[a-z][a-z0-9]*(?:-[a-z0-9]+)*(?:__[a-z0-9]+(?:-[a-z0-9]+)*)?(?:--[a-z0-9]+(?:-[a-z0-9]+)*)?)$',
      {
        message:
          'Name classes in kebab-case, a modifier after --, and block__element--modifier only for a reusable component (AGENTS.md)',
      },
    ],
    // Off: the stylesheets are ordered by object and domain on purpose (app.css sets
    // the cascade between files), and this rule pairs selectors from unrelated parts
    // of the app that merely end on the same element - a login form's `input` and a
    // raffle form's `input`. Satisfying it would mean ordering by specificity rather
    // than by what each rule describes.
    'no-descending-specificity': null,
    // A typeface's name is written as the foundry writes it, in a `font-family`
    // declaration as well as in a `--font-*` token (the shared rule covers only the
    // tokens).
    'value-keyword-case': ['lower', { ignoreProperties: ['/^--font-/', 'font-family'] }],
  },
}
