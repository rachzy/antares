# Sky Viewer — Design Spec

## Purpose

Antares will eventually let a user pinpoint a location in the sky and download lightcurve data for it. This spec covers the first milestone only: a full-page sky visualization, built on Aladin Lite (the CDS/Unistra HiPS viewer), that lets a user pan/zoom a real sky survey and click a point to see its RA/Dec. This is the foundation the lightcurve-lookup feature will later build on (the clicked RA/Dec is what a future lightcurve API call would consume) — that lookup itself is out of scope here.

## Background

`apps/www` is currently a near-empty Angular 22 zoneless app (standalone components, no zone.js, no SSR) with a single Hello World page and no routes wired up. There is no existing sky-viewer flow to extend, and this introduces a new external dependency (`aladin-lite`), so this is new-subsystem work rather than a bounded change.

## Decisions carried from brainstorming

- **Milestone scope:** full-page canvas + click-to-select a point, surfacing the clicked point's RA/Dec (not yet: survey switching UI, object lookup, lightcurve fetch).
- **Code structure:** lives directly in `apps/www/src/app/`, no separate Nx lib (can be extracted later if reuse demands it).
- **Route:** the sky viewer replaces the current Hello World home page (`/`).
- **Default survey:** `P/DSS2/color`.
- **Coordinate display:** decimal degrees (RA, Dec), 4 decimal places.

## Architecture

One new standalone component, `SkyViewer` (selector `an-sky-viewer`), at `apps/www/src/app/sky-viewer/`:
- `sky-viewer.ts`
- `sky-viewer.html`
- `sky-viewer.css`
- `sky-viewer.spec.ts`

This matches the existing no-suffix file naming used by `App` (`app.ts`, not `app.component.ts`).

`app.routes.ts` gets its first route:

```ts
export const appRoutes: Route[] = [
  { path: '', loadComponent: () => import('./sky-viewer/sky-viewer').then((m) => m.SkyViewer) },
];
```

`app.html` is reduced to just `<router-outlet />`. The current static `<main>Hello World...</main>` markup and the `HlmButton` import in `App` are removed — `App` becomes a pure shell around the router outlet.

`aladin-lite` is added as a runtime dependency of `apps/www` (npm package, WebGL/WASM-based, LGPL-3.0-or-later).

## Components

`SkyViewer`'s template is:
- A full-viewport host `<div #skyDiv>` that Aladin Lite renders its canvas into.
- A small overlay panel (absolutely positioned, Tailwind utility classes, consistent with the rest of the app's styling approach), shown only once a point has been clicked.

State lives entirely on the component — no service, no store — since this is a single self-contained view:

```ts
selectedCoord = signal<{ ra: number; dec: number } | null>(null);
loadError = signal(false);
```

## Data flow

1. Constructor calls `afterNextRender(() => this.initAladin())`. `afterNextRender` is the Angular-idiomatic hook for browser-only, DOM-dependent initialization (the modern equivalent of `ngAfterViewInit` for this purpose), and keeps the component SSR-safe if that's ever added later (this app currently has no SSR).
2. `initAladin()`:
   - Dynamically imports `aladin-lite`.
   - Awaits `A.init`.
   - Calls `A.aladin(this.skyDiv().nativeElement, { survey: 'P/DSS2/color', fov: 60, cooFrame: 'equatorial' })`.
   - Wraps all of the above in try/catch (see Error handling).
3. A native `click` listener is attached to the `#skyDiv` element. On click:
   - Reads `event.offsetX` / `event.offsetY` (pixel coordinates relative to the div — this matches the origin `pix2world` documents: top-left corner of the Aladin Lite view).
   - Calls `aladin.pix2world(offsetX, offsetY)`, which returns `[ra, dec]` in degrees (ICRS), or `null`/`undefined` if the click falls outside the projected sky disk.
   - If a result is returned, sets `selectedCoord.set({ ra, dec })`.
4. The template renders the overlay panel when `selectedCoord()` is non-null, formatted as `RA {ra.toFixed(4)}°, Dec {dec.toFixed(4)}°`.

## Error handling

- **Click outside the projected sky disk:** `pix2world` returns `null`/`undefined`. This is a no-op, not an error — `selectedCoord` is left unchanged.
- **Aladin fails to initialize** (tile/WASM fetch failure, no WebGL support, `A.init` rejects, or `A.aladin(...)` throws synchronously): caught, logged via `console.error`, and `loadError.set(true)`. The template shows a plain-text fallback message ("Unable to load the sky viewer.") instead of the coordinate panel in this state.
- **Component teardown:** the click listener is removed via `DestroyRef.onDestroy(...)` so nothing leaks if the component is ever unmounted (e.g. once more routes exist).

## Testing

`aladin-lite` requires a WebGL/WASM context that jsdom (this project's vitest environment, per `apps/www/vite.config.ts`) cannot provide. `sky-viewer.spec.ts` mocks the module wholesale with `vi.mock('aladin-lite', ...)`, exposing:
- A fake `A.init` (a resolved promise).
- A fake `A.aladin()` returning a stub object with a mockable `pix2world`.

Test cases:
1. On render, `A.aladin` is called with `survey: 'P/DSS2/color'` (and the other fixed options).
2. Dispatching a `click` event on the host div with known `offsetX`/`offsetY`, where the stub's `pix2world` returns `[83.8221, -5.3911]`, results in the panel showing `RA 83.8221°, Dec -5.3911°`.
3. `pix2world` returning `null` leaves the panel absent (no crash, no stale content shown).
4. `A.init` rejecting results in the fallback error text being shown instead of a thrown/unhandled rejection.

## Out of scope (future milestones)

- Survey switcher UI / additional HiPS layers.
- Object lookup (Simbad/NED) at the clicked position.
- Actually fetching and displaying lightcurve data for a selected position.
- Extracting `SkyViewer` into a reusable `libs/` package.
