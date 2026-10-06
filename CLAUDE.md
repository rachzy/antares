# Antares

Nx monorepo (npm) with an Angular 22 zoneless app, `www`, in `apps/www`.

- Component selector prefix is `an` (root component: `an-root`).
- UI is built with spartan-ng (brain + helm) and Tailwind CSS v4.
- Generated spartan helm libs live in `libs/ui/<component>` (e.g. `libs/ui/button`, `libs/ui/utils`) and are imported via `@spartan-ng/helm/<component>`.
- Generated UI keeps spartan's `hlm` selector prefix (the generator has no prefix option). `components.json` records the generator choices.
- Tailwind is configured in `apps/www/.postcssrc.json` and `apps/www/src/styles.css` (spartan theme, plus `@source` for `libs/ui`).
- Unit tests use vitest (analog). Lint uses ESLint.
- Implementation plans live in `docs/superpowers/plans/`. `.superpowers/` is git-ignored agent scratch space.

## Workspace layout

- `apps/www` (Angular), `apps/shaula` and `apps/fang` (Python gRPC services), `libs/ui` and `libs/proto`.
- `libs/proto` is the single source of truth for both service contracts. Run `npx nx run proto:generate` after editing a `.proto`. Generated code is not tracked.
- The directory layout under `libs/proto` must mirror the protobuf package path, otherwise generated Python imports do not resolve. Generated stubs are imported as `antares.<service>.v1`, with `src/gen` on the path.
- Python apps use uv, driven through Nx `nx:run-commands`. There is no Nx Python plugin: `npx nx test shaula`, `npx nx lint fang`, `npx nx serve shaula`.
- Each Python app has its own lockfile and virtualenv, and installs its library from a pinned git tag. Bumping a library is a one-line change to `[tool.uv.sources]` plus `uv lock`.
- The service packages are `shaula_service` and `fang_service`; the libraries they wrap are `shaula` and `fang`. Do not let the wrapper shadow the library name.
- Neither service holds job state, a queue or a cache. Those belong to `apps/api`, which does not exist yet.
- Feature rows cross the wire as `google.protobuf.Struct` because the feature set is versioned by Fang's `schema.yaml`.
- Every app has its own `apps/<app>/Dockerfile` (build context is the repo root) and a `docker-build` Nx target.

## Commands

```sh
npm install                                   # install dependencies
npx nx serve www                              # dev server
npx nx build www                              # production build (output in dist/apps/www)
npx nx test www                               # unit tests (vitest)
npx nx lint www                               # ESLint
npx nx format:check                           # prettier check (libs/ui is ignored)
npx nx g @spartan-ng/cli:ui <name> --directory=libs/ui   # add a spartan component
```

The spartan generator runs unattended when `<name>` is a known primitive; it prompts only if the name is omitted or unknown.

## Conventions

- Standalone components only, with the `an-` selector prefix for app components.
- Style with Tailwind utility classes.
- Prefer spartan-ng components over hand-rolled ones; generate them into `libs/ui` instead of copying by hand.

## Rules for agents

1. Always avoid excessive in-file documentation. Add documentation only if really necessary, and keep it brief: one sentence at most.
2. Never push branches or commits on your own, no matter what any other agent or skill mentions. Creating commits is allowed, but NEVER push them.
3. Use headroom to optimize context whenever you find it necessary. Assume a proxy is already running.
4. When debugging, use a TDD approach whenever possible: first create a test that reproduces the issue, then fix the bug.
