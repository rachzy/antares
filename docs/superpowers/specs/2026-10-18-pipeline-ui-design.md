# Pipeline UI design

## Intent

Today `apps/www` shows one sky viewer where a click resolves a star and a single "Analyze" button runs the whole API job. The API (`apps/api`) is now a staged pipeline (resolve, download, process, score), so the UI should walk the user through those stages, one page each, with the live job output visible at every stage.

## Goals

- Select a star by pointing a visible crosshair at it and pressing Confirm.
- Show the pipeline as four pages with a shared stepper: Select star, Download light curves, Process data, Score candidates. Steps unlock as the previous one finishes.
- Stream every WebSocket event into a terminal-style log on the Download, Process and Score pages.
- Present the final candidate scores clearly, with detail for people who want to see how the models scored.
- Show API errors in toasts.

## Non-goals

- No new API endpoints or API changes.
- No training, run selection or multi-star batch UI.
- No offline or SSR support beyond what the existing stores already do.

## Anti-goals

- No second look: the pages reuse the existing Antares look (orange `#f54900` on warm black, Fraunces, Inter, IBM Plex Mono from `feat/welcome-page`), not a new theme.
- Pages must not call `ApiService` directly; stores only (existing rule).

## Constraints

- Builds on branch `feat/www-api-integration` (routes, `ApiService`, stores) and needs the welcome page and theme from `feat/welcome-page`, which conflicts with the sky viewer there and must be merged by hand.
- Angular 22 zoneless, standalone components, `an-` prefix, Tailwind, spartan-ng helm components generated into `libs/ui`.
- Commit conventions per `CONTRIBUTING.md`. `plans/` is never committed. No pushes.

## What was decided

### Facts about the API (checked in the merged code)

- The WebSocket relays Shaula progress (`downloading`, `detrending`, `period_search` with `fraction`) and one `predicting` state from the API. Fang's prediction has no progress events. The terminal ends with a `result` or `error` event; Shaula's own `done` is not relayed.
- The end of the light curve download is the first event whose stage is `detrending`.
- No endpoint downloads extracted features. They arrive inline in the job result (`features`), so "Download extracted features" is built in the browser (CSV).
- `POST /targets/resolve` with a position returns the nearest star with light curves within 4 arcseconds, otherwise `404`. A cache hit on `POST /pipeline/extract` returns `200` with the result and no token, so there is no stream to show.
- `POST /analyze` after an extraction reuses the cached features and only runs Fang. A prediction row is the input features plus `prob_<model>` for each base model (lightgbm, catboost, extra_trees), `prob_stack`, `potential_probability`, `decision_threshold`, `prediction` (`POTENTIAL` or `UNLIKELY`), `model_name`, `model_run_id`, `star_id`. The result also carries `model_run_id`, `schema_version` and `cached`.
- Aladin Lite has a built-in reticle (`showReticle`, `reticleColor`, `reticleSize`); a Confirm button is an HTML overlay and can always be visible. Aladin cannot tell a star from empty sky; the API resolve does.

### Pages and routes

`/select`, `/download`, `/process`, `/score`. Each is its own routed page and renders the shared stepper. Locked steps cannot be reached by URL either; they redirect to the furthest unlocked step. The saved subscription token (existing `SubscriptionStore`, `localStorage`) restores the current star and step after a refresh.

### Stepper

A line of stations. Done is an orange outlined check, current is filled orange, locked is dimmed with a tooltip. Mockups: `.superpowers/brainstorm/*/content/pipeline-layout-v3.html`.

### Select page

Full-height sky with a centered orange crosshair. Confirm is always visible but disabled until the field of view is small enough that one star can sit in the ring, with a hint to zoom. Confirm resolves the center position through the target store. A miss shows an error toast. A hit shrinks the sky to the right and shows a left card: star name, a link to `https://www.stellarcatalog.com/stars/<star name>`, and "Extract lightcurve features". The zoom limit (field of view) is chosen by the builder.

### Download and Process pages

Clicking Extract starts the job and moves to Download. The terminal prints every event. When the first `detrending` event arrives the page marks the download finished and Process unlocks; the Process page shows the same log from the `detrending` events on. When the `result` event arrives, two buttons appear: Score candidates (primary, with a tooltip: "Runs the extracted data through our machine learning model and shows how likely each candidate is to be a real exoplanet") and Download extracted features (secondary, CSV in the browser). A cached extraction (`200`, no token) skips straight to the finished state with a single "loaded from cache" line.

### Score page

Score candidates calls analyze and moves to Score. The log sits on top, with three tabs below it:

- **Ranking:** table sorted by `potential_probability`, a bar with a threshold tick and a POTENTIAL or UNLIKELY tag per candidate.
- **Model matrix:** candidates by models (one `prob_*` column each, read dynamically from the row) plus the decision, as a heat-map.
- **Scoring details:** chips for model, threshold, run, schema and cache state; a decision table (score, threshold, margin, decision); a per-model scores table where only the row used for the decision (the stack) has the orange bar and the others are muted; and a candidates by features table (missing values shown as a dash, first 7 features with a Show all toggle).

Mockups: `score-step-v4.html` in the same folder.

### Cross-cutting

- API errors (store `error` signals) show in a spartan-ng toaster. `busy` errors say when to retry.
- spartan-ng tooltip for the locked steps and the Score candidates button.
- Components inject stores only. New state lives in stores: a selected-star store (coordinates, resolved target, step) and the existing pipeline and subscription stores for the jobs.

## Left to the builder

- Exact field-of-view threshold and the zoom hint wording.
- How steps are persisted beyond the subscription token (token plus a persisted target name is enough).
- Splitting the single log between Download and Process by stage (my guess, confirmed by the user in conversation).
- How the stellarcatalog link encodes star names.
- CSV column order and file name.

## Calls made while writing this (not yet confirmed by the user)

- Done steps stay viewable. Download and Process keep their log and the CSV button after scoring, so extract and score results live in separate state instead of one "latest job". Both job tokens are persisted, so a refresh restores whichever steps are done.
- Score unlocks when the user presses Score candidates (analyze starts), not when extraction ends. The button is on the Process page; `/score` redirects to the furthest unlocked step until then. Scoring can be re-run from the Score page.
- Extract and analyze send the same body: the resolved target's `ra_deg` and `dec_deg` with the mission derived from its catalog (KIC is Kepler, EPIC is K2, TIC is TESS), so the cached features are reused and the star analyzed is the star confirmed.
- Failure or a dropped job: a toast plus a Retry button on the page; "Select star" stays reachable. Picking a new star resets the pipeline and asks for confirmation only if a job is running.
- A cache hit (`200`, no token) lands on Process with Download and Process both shown as done.
- A missing or expired token lands on `/select`.
- The welcome page stays at `/`; expanding its sky navigates to `/select`. Merging `feat/welcome-page` (hand-resolving the sky viewer conflict) is the plan's first task.
