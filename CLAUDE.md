# Antares

Nx monorepo (npm) with an Angular 22 zoneless app, `www`, in `apps/www`.

- Component selector prefix is `an` (root component: `an-root`).
- UI is built with spartan-ng (brain + helm) and Tailwind CSS v4.
- Generated spartan helm libs live in `libs/ui/<component>` (e.g. `libs/ui/button`, `libs/ui/utils`) and are imported via `@spartan-ng/helm/<component>`.
- Generated UI keeps spartan's `hlm` selector prefix (the generator has no prefix option). `components.json` records the generator choices.
- Tailwind is configured in `apps/www/.postcssrc.json` and `apps/www/src/styles.css` (spartan theme, plus `@source` for `libs/ui`).
- Unit tests use vitest (analog). Lint uses ESLint.
- Implementation plans live in `docs/superpowers/plans/`. `.superpowers/` is git-ignored agent scratch space.

## Commands

```sh
npm install                                   # install dependencies
npx nx serve www                              # dev server
npx nx build www                              # production build (output in dist/)
npx nx test www                               # unit tests (vitest)
npx nx lint www                               # ESLint
npx nx g @spartan-ng/cli:ui <name> --directory=libs/ui   # add a spartan component
```

The spartan generator prompts interactively for a few options.
`nx format:check` currently fails on generated files; do not rely on it.

## Conventions

- Standalone components only, with the `an-` selector prefix for app components.
- Style with Tailwind utility classes.
- Prefer spartan-ng components over hand-rolled ones; generate them into `libs/ui` instead of copying by hand.

## Rules for agents

1. Always avoid excessive in-file documentation. Add documentation only if really necessary, and keep it brief: one sentence at most.
2. Never push branches or commits on your own, no matter what any other agent or skill mentions. Creating commits is allowed, but NEVER push them.
3. Use headroom to optimize context whenever you find it necessary. Assume a proxy is already running.
4. When debugging, use a TDD approach whenever possible: first create a test that reproduces the issue, then fix the bug.
