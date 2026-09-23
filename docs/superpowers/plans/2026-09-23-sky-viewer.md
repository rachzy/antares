# Sky Viewer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the Hello World landing page with a full-page Aladin Lite sky viewer that lets a user click anywhere on the sky and see the RA/Dec of that point.

**Architecture:** A new standalone `SkyViewer` component (`apps/www/src/app/sky-viewer/`) wraps the `aladin-lite` npm package behind a minimal hand-written ambient type declaration, initializes it in an `afterNextRender` hook against a full-viewport div, and exposes the clicked coordinate as a signal rendered in a small overlay panel. `app.routes.ts` is rewired so the root path (`''`) lazy-loads `SkyViewer`, and `App`/`app.html` shrink to a bare router outlet.

**Tech Stack:** Angular 22 (zoneless, standalone components), `aladin-lite` (WebGL/WASM HiPS viewer, LGPL-3.0-or-later), Tailwind CSS v4 utility classes, vitest (jsdom environment) via `@analogjs/vite-plugin-angular`.

**Spec:** `docs/superpowers/specs/2026-09-23-sky-viewer-design.md`

## Global Constraints

- Standalone components only, `an-` selector prefix (`@angular-eslint/component-selector` enforces kebab-case element selectors with prefix `an`).
- No zone.js — this app is zoneless; reactivity goes through signals, and tests must call `fixture.whenStable()` / `fixture.detectChanges()` explicitly rather than relying on zone-triggered change detection.
- Style with Tailwind utility classes only; no new CSS beyond an empty `.css` file per component, matching the existing `app.css` pattern.
- `aladin-lite` ships no TypeScript types (`npm view aladin-lite@3.8.2 types` returns nothing) — a local ambient `declare module 'aladin-lite'` covering only the surface this app uses is required; do not attempt to fully type the library.
- Test environment is jsdom (`apps/www/vite.config.ts`), which has no WebGL/WASM support, so `aladin-lite` must be mocked in every spec that touches `SkyViewer`, never imported for real.
- Avoid excessive in-file documentation — one sentence at most, only where the WHY is non-obvious (per `CLAUDE.md`).
- Never push commits — commit locally after each step, do not push.
- TDD: write the failing test before the implementation for every behavioral step.

## Review Focus

- A click lands outside the projected sky disk (`pix2world` returns `null`/`undefined`) — the panel must not appear or show stale content, and nothing should throw.
- Aladin Lite fails to initialize (rejected `A.init`, or `A.aladin(...)` throwing) — the user must see a clear fallback message, not a blank screen or a console-only failure.
- The component is destroyed while its click listener is still attached — the listener must be removed and must not fire or throw afterward.
- After rewiring the app shell, the root route (`''`) must actually resolve to `SkyViewer`, not silently keep serving the old Hello World content.
- The exact default survey/config (`P/DSS2/color`, `fov: 60`, `cooFrame: 'equatorial'`) must reach `A.aladin(...)` — a silent fallback to the library's own defaults would defeat the spec's explicit choice.

---

### Task 1: `aladin-lite` dependency, ambient types, and SkyViewer initialization

**Files:**
- Modify: `package.json` (root) — add `aladin-lite` dependency
- Create: `apps/www/src/types/aladin-lite.d.ts`
- Create: `apps/www/src/app/sky-viewer/sky-viewer.ts`
- Create: `apps/www/src/app/sky-viewer/sky-viewer.html`
- Create: `apps/www/src/app/sky-viewer/sky-viewer.css`
- Test: `apps/www/src/app/sky-viewer/sky-viewer.spec.ts`

**Interfaces:**
- Produces: `SkyViewer` (selector `an-sky-viewer`), a standalone component with protected signals `selectedCoord: Signal<{ ra: number; dec: number } | null>` and `loadError: Signal<boolean>`, and a protected `skyDiv = viewChild.required<ElementRef<HTMLDivElement>>('skyDiv')`. Consumed by Task 4's route config as `import('./sky-viewer/sky-viewer').then((m) => m.SkyViewer)`.
- Produces (types): `apps/www/src/types/aladin-lite.d.ts` declares `AladinInstance` (with `pix2world(x: number, y: number, frame?: string): [number, number] | null | undefined`), `AladinOptions` (`{ survey?: string; fov?: number; cooFrame?: string }`), and `AladinStatic` (`{ init: Promise<void>; aladin(element: HTMLElement, options?: AladinOptions): AladinInstance }`) as the default export. Task 2 imports `AladinInstance` from `'aladin-lite'`.

- [ ] **Step 1: Install the dependency**

Run: `npm install aladin-lite`

This adds `aladin-lite` (currently `^3.8.2`, the latest non-beta release) to the root `package.json` dependencies.

- [ ] **Step 2: Write the ambient type declaration**

Create `apps/www/src/types/aladin-lite.d.ts`:

```ts
declare module 'aladin-lite' {
  export interface AladinInstance {
    pix2world(x: number, y: number, frame?: string): [number, number] | null | undefined;
  }

  export interface AladinOptions {
    survey?: string;
    fov?: number;
    cooFrame?: string;
  }

  export interface AladinStatic {
    init: Promise<void>;
    aladin(element: HTMLElement, options?: AladinOptions): AladinInstance;
  }

  const A: AladinStatic;
  export default A;
}
```

- [ ] **Step 3: Write the failing test**

Create `apps/www/src/app/sky-viewer/sky-viewer.spec.ts`:

```ts
import { TestBed } from '@angular/core/testing';
import { SkyViewer } from './sky-viewer';

const aladinMock = vi.fn();
const pix2worldMock = vi.fn();
let initPromise: Promise<void>;

vi.mock('aladin-lite', () => ({
  default: {
    get init() {
      return initPromise;
    },
    aladin: (element: HTMLElement, options: unknown) => {
      aladinMock(element, options);
      return { pix2world: pix2worldMock };
    },
  },
}));

describe('SkyViewer', () => {
  beforeEach(async () => {
    aladinMock.mockClear();
    pix2worldMock.mockReset();
    initPromise = Promise.resolve();

    await TestBed.configureTestingModule({
      imports: [SkyViewer],
    }).compileComponents();
  });

  it('initializes Aladin with the default survey', async () => {
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();

    await vi.waitFor(() => {
      expect(aladinMock).toHaveBeenCalledWith(expect.any(HTMLDivElement), {
        survey: 'P/DSS2/color',
        fov: 60,
        cooFrame: 'equatorial',
      });
    });
  });
});
```

- [ ] **Step 4: Run the test and verify it fails**

Run: `npx nx test www`
Expected: FAIL — `sky-viewer.ts` (and `sky-viewer.html`/`.css`) do not exist yet, so the import fails to resolve.

- [ ] **Step 5: Write the minimal implementation**

Create `apps/www/src/app/sky-viewer/sky-viewer.css` (empty file, matching `app.css`).

Create `apps/www/src/app/sky-viewer/sky-viewer.html`:

```html
<div #skyDiv class="h-screen w-screen"></div>
```

Create `apps/www/src/app/sky-viewer/sky-viewer.ts`:

```ts
import {
  afterNextRender,
  Component,
  ElementRef,
  signal,
  viewChild,
} from '@angular/core';

@Component({
  selector: 'an-sky-viewer',
  templateUrl: './sky-viewer.html',
  styleUrl: './sky-viewer.css',
})
export class SkyViewer {
  protected readonly skyDiv = viewChild.required<ElementRef<HTMLDivElement>>('skyDiv');
  protected readonly selectedCoord = signal<{ ra: number; dec: number } | null>(null);
  protected readonly loadError = signal(false);

  constructor() {
    afterNextRender(() => void this.initAladin());
  }

  private async initAladin(): Promise<void> {
    const { default: A } = await import('aladin-lite');
    await A.init;
    A.aladin(this.skyDiv().nativeElement, {
      survey: 'P/DSS2/color',
      fov: 60,
      cooFrame: 'equatorial',
    });
  }
}
```

- [ ] **Step 6: Run the test and verify it passes**

Run: `npx nx test www`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add package.json package-lock.json apps/www/src/types/aladin-lite.d.ts apps/www/src/app/sky-viewer/
git commit -m "feat(www): initialize Aladin Lite sky viewer"
```

---

### Task 2: Click-to-coordinate handling, overlay panel, and destroy cleanup

**Files:**
- Modify: `apps/www/src/app/sky-viewer/sky-viewer.ts`
- Modify: `apps/www/src/app/sky-viewer/sky-viewer.html`
- Modify (test): `apps/www/src/app/sky-viewer/sky-viewer.spec.ts`

**Interfaces:**
- Consumes: `AladinInstance` from `apps/www/src/types/aladin-lite.d.ts` (Task 1); `SkyViewer`'s existing `skyDiv`, `selectedCoord` signals (Task 1).
- Produces: `SkyViewer` now attaches a `click` listener to the `skyDiv` element on init and removes it via `DestroyRef.onDestroy` on teardown; `selectedCoord` is updated from `aladin.pix2world(event.offsetX, event.offsetY)`.

- [ ] **Step 1: Write the failing tests**

Replace the contents of `apps/www/src/app/sky-viewer/sky-viewer.spec.ts` with:

```ts
import { TestBed } from '@angular/core/testing';
import { SkyViewer } from './sky-viewer';

const aladinMock = vi.fn();
const pix2worldMock = vi.fn();
let initPromise: Promise<void>;

vi.mock('aladin-lite', () => ({
  default: {
    get init() {
      return initPromise;
    },
    aladin: (element: HTMLElement, options: unknown) => {
      aladinMock(element, options);
      return { pix2world: pix2worldMock };
    },
  },
}));

describe('SkyViewer', () => {
  beforeEach(async () => {
    aladinMock.mockClear();
    pix2worldMock.mockReset();
    initPromise = Promise.resolve();

    await TestBed.configureTestingModule({
      imports: [SkyViewer],
    }).compileComponents();
  });

  it('initializes Aladin with the default survey', async () => {
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();

    await vi.waitFor(() => {
      expect(aladinMock).toHaveBeenCalledWith(expect.any(HTMLDivElement), {
        survey: 'P/DSS2/color',
        fov: 60,
        cooFrame: 'equatorial',
      });
    });
  });

  it('shows the clicked RA/Dec when the sky is clicked', async () => {
    pix2worldMock.mockReturnValue([83.8221, -5.3911]);
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();
    await vi.waitFor(() => expect(aladinMock).toHaveBeenCalled());

    const skyDiv = fixture.nativeElement.querySelector('div') as HTMLDivElement;
    skyDiv.dispatchEvent(new MouseEvent('click'));
    fixture.detectChanges();
    await fixture.whenStable();

    expect(pix2worldMock).toHaveBeenCalledWith(0, 0);
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent).toContain('RA 83.8221°, Dec -5.3911°');
  });

  it('does not show a panel when the click falls outside the sky', async () => {
    pix2worldMock.mockReturnValue(null);
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();
    await vi.waitFor(() => expect(aladinMock).toHaveBeenCalled());

    const skyDiv = fixture.nativeElement.querySelector('div') as HTMLDivElement;
    skyDiv.dispatchEvent(new MouseEvent('click'));
    fixture.detectChanges();
    await fixture.whenStable();

    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.textContent?.trim()).toBe('');
  });

  it('removes the click listener when the component is destroyed', async () => {
    pix2worldMock.mockReturnValue([1, 2]);
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();
    await vi.waitFor(() => expect(aladinMock).toHaveBeenCalled());

    const skyDiv = fixture.nativeElement.querySelector('div') as HTMLDivElement;
    fixture.destroy();

    expect(() => skyDiv.dispatchEvent(new MouseEvent('click'))).not.toThrow();
    expect(pix2worldMock).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run the tests and verify the new ones fail**

Run: `npx nx test www`
Expected: FAIL on the three new tests — no click listener exists yet, so `pix2worldMock` is never called and no panel text is ever rendered.

- [ ] **Step 3: Write the minimal implementation**

Replace `apps/www/src/app/sky-viewer/sky-viewer.html` with:

```html
<div #skyDiv class="h-screen w-screen"></div>

@if (selectedCoord(); as coord) {
  <div class="absolute bottom-4 left-4 rounded bg-card px-3 py-2 text-sm text-card-foreground shadow">
    RA {{ coord.ra.toFixed(4) }}&deg;, Dec {{ coord.dec.toFixed(4) }}&deg;
  </div>
}
```

Replace `apps/www/src/app/sky-viewer/sky-viewer.ts` with:

```ts
import {
  afterNextRender,
  Component,
  DestroyRef,
  ElementRef,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import type { AladinInstance } from 'aladin-lite';

@Component({
  selector: 'an-sky-viewer',
  templateUrl: './sky-viewer.html',
  styleUrl: './sky-viewer.css',
})
export class SkyViewer {
  protected readonly skyDiv = viewChild.required<ElementRef<HTMLDivElement>>('skyDiv');
  protected readonly selectedCoord = signal<{ ra: number; dec: number } | null>(null);
  protected readonly loadError = signal(false);

  private readonly destroyRef = inject(DestroyRef);

  constructor() {
    afterNextRender(() => void this.initAladin());
  }

  private async initAladin(): Promise<void> {
    const { default: A } = await import('aladin-lite');
    await A.init;
    const aladin = A.aladin(this.skyDiv().nativeElement, {
      survey: 'P/DSS2/color',
      fov: 60,
      cooFrame: 'equatorial',
    });
    this.attachClickHandler(aladin);
  }

  private attachClickHandler(aladin: AladinInstance): void {
    const element = this.skyDiv().nativeElement;
    const onClick = (event: MouseEvent) => {
      const coords = aladin.pix2world(event.offsetX, event.offsetY);
      if (!coords) {
        return;
      }
      const [ra, dec] = coords;
      this.selectedCoord.set({ ra, dec });
    };

    element.addEventListener('click', onClick);
    this.destroyRef.onDestroy(() => element.removeEventListener('click', onClick));
  }
}
```

- [ ] **Step 4: Run the tests and verify they pass**

Run: `npx nx test www`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add apps/www/src/app/sky-viewer/
git commit -m "feat(www): show clicked sky coordinates in an overlay panel"
```

---

### Task 3: Fallback message when Aladin fails to initialize

**Files:**
- Modify: `apps/www/src/app/sky-viewer/sky-viewer.ts`
- Modify: `apps/www/src/app/sky-viewer/sky-viewer.html`
- Modify (test): `apps/www/src/app/sky-viewer/sky-viewer.spec.ts`

**Interfaces:**
- Consumes: `SkyViewer`'s existing `loadError` signal (declared but unused since Task 1).
- Produces: no new public surface — `initAladin` now catches failures and sets `loadError`; later tasks are unaffected.

- [ ] **Step 1: Write the failing test**

Add this test inside the existing `describe('SkyViewer', ...)` block in `apps/www/src/app/sky-viewer/sky-viewer.spec.ts` (after the "removes the click listener..." test):

```ts
  it('shows a fallback message when Aladin fails to initialize', async () => {
    initPromise = Promise.reject(new Error('network error'));
    const fixture = TestBed.createComponent(SkyViewer);
    await fixture.whenStable();

    await vi.waitFor(() => {
      fixture.detectChanges();
      const compiled = fixture.nativeElement as HTMLElement;
      expect(compiled.textContent).toContain('Unable to load the sky viewer.');
    });
  });
```

- [ ] **Step 2: Run the test and verify it fails**

Run: `npx nx test www`
Expected: FAIL — the rejection is currently unhandled (an unhandled promise rejection), and no fallback text is ever rendered.

- [ ] **Step 3: Write the minimal implementation**

Replace `apps/www/src/app/sky-viewer/sky-viewer.html` with:

```html
<div #skyDiv class="h-screen w-screen"></div>

@if (loadError()) {
  <p class="absolute bottom-4 left-4 rounded bg-destructive/10 px-3 py-2 text-sm text-destructive">
    Unable to load the sky viewer.
  </p>
} @else if (selectedCoord(); as coord) {
  <div class="absolute bottom-4 left-4 rounded bg-card px-3 py-2 text-sm text-card-foreground shadow">
    RA {{ coord.ra.toFixed(4) }}&deg;, Dec {{ coord.dec.toFixed(4) }}&deg;
  </div>
}
```

In `apps/www/src/app/sky-viewer/sky-viewer.ts`, replace the `initAladin` method with:

```ts
  private async initAladin(): Promise<void> {
    try {
      const { default: A } = await import('aladin-lite');
      await A.init;
      const aladin = A.aladin(this.skyDiv().nativeElement, {
        survey: 'P/DSS2/color',
        fov: 60,
        cooFrame: 'equatorial',
      });
      this.attachClickHandler(aladin);
    } catch (error) {
      console.error('Failed to initialize the sky viewer', error);
      this.loadError.set(true);
    }
  }
```

- [ ] **Step 4: Run the tests and verify they all pass**

Run: `npx nx test www`
Expected: PASS (all `SkyViewer` tests, including the four from Tasks 1–2)

- [ ] **Step 5: Commit**

```bash
git add apps/www/src/app/sky-viewer/
git commit -m "feat(www): show a fallback message if the sky viewer fails to load"
```

---

### Task 4: Wire SkyViewer into the app shell and retire the Hello World page

**Files:**
- Modify: `apps/www/src/app/app.routes.ts`
- Modify: `apps/www/src/app/app.html`
- Modify: `apps/www/src/app/app.ts`
- Modify: `apps/www/src/app/app.spec.ts`

**Interfaces:**
- Consumes: `SkyViewer` from `apps/www/src/app/sky-viewer/sky-viewer` (Task 1).
- Produces: nothing consumed by later tasks — this is the final integration point for this plan.

- [ ] **Step 1: Write the failing tests**

Replace the contents of `apps/www/src/app/app.spec.ts` with:

```ts
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { App } from './app';
import { appRoutes } from './app.routes';
import { SkyViewer } from './sky-viewer/sky-viewer';

describe('App', () => {
  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [App],
      providers: [provideRouter(appRoutes)],
    }).compileComponents();
  });

  it('should render a router outlet', async () => {
    const fixture = TestBed.createComponent(App);
    await fixture.whenStable();
    const compiled = fixture.nativeElement as HTMLElement;
    expect(compiled.querySelector('router-outlet')).not.toBeNull();
  });
});

describe('appRoutes', () => {
  it('routes the empty path to SkyViewer', async () => {
    const [route] = appRoutes;
    expect(route.path).toBe('');
    const loadComponent = route.loadComponent as () => Promise<typeof SkyViewer>;
    await expect(loadComponent()).resolves.toBe(SkyViewer);
  });
});
```

- [ ] **Step 2: Run the tests and verify they fail**

Run: `npx nx test www`
Expected: FAIL — `appRoutes` is currently empty (`[]`), so `appRoutes[0]` is `undefined` and `route.path` throws; the old `App` tests (Hello World heading/button) are gone, so no conflicting failures there.

- [ ] **Step 3: Write the minimal implementation**

Replace `apps/www/src/app/app.routes.ts` with:

```ts
import { Route } from '@angular/router';

export const appRoutes: Route[] = [
  {
    path: '',
    loadComponent: () => import('./sky-viewer/sky-viewer').then((m) => m.SkyViewer),
  },
];
```

Replace `apps/www/src/app/app.html` with:

```html
<router-outlet></router-outlet>
```

Replace `apps/www/src/app/app.ts` with:

```ts
import { Component } from '@angular/core';
import { RouterModule } from '@angular/router';

@Component({
  imports: [RouterModule],
  selector: 'an-root',
  templateUrl: './app.html',
  styleUrl: './app.css',
})
export class App {}
```

- [ ] **Step 4: Run the tests and verify they pass**

Run: `npx nx test www`
Expected: PASS (all `App`, `appRoutes`, and `SkyViewer` tests)

- [ ] **Step 5: Run the full app build to confirm everything still type-checks and bundles**

Run: `npx nx build www`
Expected: build succeeds with no TypeScript errors (confirms the `aladin-lite` ambient types resolve correctly in a production build, not just under vitest).

- [ ] **Step 6: Commit**

```bash
git add apps/www/src/app/app.routes.ts apps/www/src/app/app.html apps/www/src/app/app.ts apps/www/src/app/app.spec.ts
git commit -m "feat(www): make the sky viewer the landing page"
```

---
