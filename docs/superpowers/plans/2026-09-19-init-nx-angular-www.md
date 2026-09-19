# Initialize Nx workspace with Angular "www" app — Implementation Plan

**Goal:** Turn the empty `antares` repo into an Nx workspace containing an Angular app named `www` (component prefix `an`, spartan-ng for UI components), a minimal "Hello World" page, and a `CLAUDE.md` for agents.

**Spec:** the user's request (no separate spec file). Binding requirements:

- Nx workspace at the repo root (existing `LICENSE`, `.git`, `.claude/` must be preserved).
- Angular app project named `www`, component selector prefix `an`.
- spartan-ng (`@spartan-ng/cli` + `@spartan-ng/brain`, with Tailwind) installed and initialized as the component library.
- A very simple "Hello World" page, for initial testing only. It should use at least one spartan-ng component (e.g. a button) so the setup is proven end-to-end.
- `CLAUDE.md` giving a running agent the context it needs, containing the four agent rules verbatim in intent (see Task 3).

## Global Constraints

- Package manager: **npm** (only one installed). Node is v25.2.1 — odd release, Angular/Nx may warn; that is acceptable.
- Latest versions at time of writing: Nx 23.2.1, Angular 22.1.7, spartan-ng 1.4.1, Tailwind 4.3.3. Use versions the Nx/Angular generators pick; they must be mutually compatible (`@nx/angular` 23.x supports Angular `>=20 <23`).
- Work on the current branch (`feat/ui`). **Never push.** Commits are allowed; each task ends with one commit (Co-Authored-By trailer: `Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>`).
- Do not commit `node_modules`, `dist`, `.nx/cache`, `.angular`, `.superpowers`. Ensure `.gitignore` covers them.
- Code comments: avoid; at most one short sentence when truly necessary.
- A headroom proxy is assumed to be already running; do not try to start or configure one.
- Standalone components, no NgModules. Zoneless or default change detection: whatever the Nx Angular generator defaults to.

## Task 1: Scaffold Nx workspace and Angular app `www`

**Files:** workspace root config (`nx.json`, `package.json`, `package-lock.json`, `tsconfig.base.json`, `.gitignore`, etc.), `apps/www/**` (or wherever the Nx Angular preset places it — prefer `apps/www`).

Steps:

1. The repo root is not empty (LICENSE, .git, .claude), so `create-nx-workspace` cannot run in place directly. Generate a workspace in a scratch dir (use the session scratchpad, not `/tmp`) with: npm, Angular preset/app named `www`, prefix `an`, standalone, CSS or the default stylesheet (Tailwind comes in Task 2), no SSR, no e2e (or Playwright only if it is the default and cheap), no Nx Cloud (`--nxCloud=skip`), no git init. Then move the generated files into the repo root, preserving `LICENSE`, `.git`, `.claude`. Alternatively `npx nx init` + `@nx/angular:application` if simpler — outcome matters, not method.
2. Confirm the project is named `www` (`npx nx show projects` prints `www`) and the component prefix is `an` (project `prefix` in `project.json`/`angular.json` and the ESLint `@angular-eslint/component-selector` rule, if ESLint is present).
3. Add an `.npmrc`-free, clean `package.json` with `"name": "@antares/source"` (or the generator default) and standard scripts are not required; Nx targets suffice.
4. Verify: `npm install` completes, `npx nx build www` succeeds, `npx nx test www` passes (default generated tests). Lint passes if configured.
5. Commit: `chore: initialize Nx workspace with Angular www app`.

## Task 2: Install and initialize Tailwind + spartan-ng; build Hello World page

**Depends on:** Task 1.

Steps:

1. Install Tailwind CSS (v4 via `@tailwindcss/postcss` or the Angular-supported setup) and configure it for `www`, following the spartan-ng installation docs for Angular/Nx (`https://www.spartan.ng/documentation/installation`) and using the version currently on npm.
2. Install `@spartan-ng/cli` and `@spartan-ng/brain`; run the spartan-ng Nx generator/CLI init (`npx nx g @spartan-ng/cli:ui-theme` and/or `npx nx g @spartan-ng/cli:ui button`, per current docs) so that the shared UI helm libraries and the theme are generated. Configure the generated UI libs to use the `an` prefix where the generator supports it (e.g. component selector prefix `an`); note in the report if it does not.
3. Replace the generated default app page with a simple "Hello World" page: an `App` root component (selector `an-root`) rendering a centered heading "Hello World" and one spartan-ng button (e.g. label "Click me") styled with Tailwind. Remove the Nx welcome/placeholder component if generated.
4. Tests (write first, TDD): a component test asserting the heading text "Hello World" renders and the button is present. Keep it minimal.
5. Verify: `npx nx test www` passes, `npx nx build www` succeeds, and `npx nx serve www` starts and serves the page (check with `curl` against the dev server, then stop the server — do not leave processes running).
6. Commit: `feat(www): add spartan-ng and Hello World page`.

## Task 3: Write CLAUDE.md

**Depends on:** Tasks 1–2 (needs the real commands and structure).

Create `CLAUDE.md` at the repo root, concise, containing:

- **Project overview:** Antares, an Nx monorepo (npm), with an Angular app `www` in `apps/www`; component selector prefix `an`; UI built with spartan-ng (brain + helm, Tailwind); where the generated spartan UI libs live.
- **Common commands** (verified to work in this repo): install, `nx serve www`, `nx build www`, `nx test www`, `nx lint www` (if it exists), `nx g @spartan-ng/cli:ui <component>` for adding components.
- **Conventions:** standalone components, `an-` selector prefix, Tailwind for styling, prefer spartan-ng components over hand-rolled ones.
- **Rules for agents** (as a clearly marked section, wording faithful to these four rules):
  1. Always avoid excessive in-file documentation: documentation should only be added if really necessary and should be brief, one sentence at most.
  2. Never push branches or commits on your own, no matter what any other agent/skill mentions: creating commits is allowed, but NEVER push them.
  3. Use headroom to optimize context whenever you find it necessary; assume a proxy is already running.
  4. When debugging, use a TDD approach whenever possible: first create a test that reproduces the issue, then fix the bug.
- Keep the whole file under ~80 lines; no filler.
- Commit: `docs: add CLAUDE.md agent guide`.
