# Shaula: resolve a star from sky coordinates

## Goal

`ResolveTarget` accepts a sky position (`ra_deg`, `dec_deg`) instead of a name and returns the same exact `catalog_id` and `designation` it returns for a name, whether or not the star was resolved before. The sky viewer can then send coordinates for stars nobody has named.

## Decisions

- **Logic lives in the Shaula library** (`~/Documents/GitHub/shaula`), not the service wrapper. The service pins the library by git tag, so a new tag is released and the pin bumped.
- **One RPC.** `ResolveTarget` is extended; no second RPC.
- **Matching:** the nearest star with light curves inside a fixed radius (4 arcseconds, about one Kepler pixel, a module constant). No radius on the wire. No match is `NOT_FOUND`. Ambiguity is not reported.
- **Coordinates are never a cache key.** The resolved `catalog_id` stays the key; returned coordinates are for display.
- **Out of scope:** a caller-supplied radius, listing ambiguous candidates, any change to extraction.

## Library

`resolve(query=None, mission="Kepler", *, ra_deg=None, dec_deg=None)`.

- Exactly one mode: a `query`, or both `ra_deg` and `dec_deg`. Anything else is `ValueError`.
- Position mode runs `lk.search_lightcurve(SkyCoord(ra, dec, unit="deg"), radius=..., ...)` through the existing `search_default_product`, so resolve and extract look at the same product class (author and exposure-time filters included).
- The nearest row wins. Rows are ordered by the search's distance column, not by `table[0]`.
- Validation: both finite, RA in [0, 360), Dec in [-90, 90]; otherwise `ValueError`. No row within the radius is `TargetNotFound`.
- `display_name` is the designation in position mode.
- `extract` is unchanged.
- Tests are offline, through the `_search_target` seam: nearest-of-several, none in range, invalid ranges, both-modes and neither-mode errors, and `ra_deg=0.0` / `dec_deg=0.0` treated as real coordinates.
- A new release tag follows.

## Proto and service (`apps/shaula`)

- `ResolveTargetRequest` gains `optional double ra_deg = 3` and `optional double dec_deg = 4`. Explicit presence is needed because 0.0 is a valid coordinate.
- The service forwards them. A query together with a position, a single coordinate, or neither maps to `INVALID_ARGUMENT`, as do out-of-range values. No match is `NOT_FOUND`.
- Proto comments are updated. The `[tool.uv.sources]` tag is bumped and `uv lock` re-run. `npx nx run proto:generate` is re-run so the Go stubs carry the new fields.
- Service tests are written first and cover each case above.

## Go plan amendment (after Shaula lands)

- Design decision 11 and `POST /targets/resolve` in `plans/go_api.md` change: the API forwards `ra_deg`/`dec_deg` to Shaula and drops the 3-arcsecond local lookup and the table behind it. Tasks 2, 3, 7 and 10 are checked for references to it.
- The resolve response and the cache key are unchanged.

## To verify during implementation

- lightkurve's cone search returns a usable distance column for Kepler, K2 and TESS.
- The archive accepts a position search together with the author and exposure-time filters the library already applies.
- If either fails, stop and revisit the matching rule before continuing.
