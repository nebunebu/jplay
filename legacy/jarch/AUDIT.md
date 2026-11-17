# Audit: staleness TTL for index pages

Date: 2026-07-22

## Problem

Reported via `jplay`: the archive browser "is not updating for new episodes."

Root cause is in `jarch`, not `jplay`. Every page went through the same
cache-first rule — fetch only when the cache file was missing or `--refresh`
was passed — with no expiry. That is correct for `game`/`player` pages, which
are immutable archival records, but wrong for the two *index* pages:

- `listseasons.php` (the `seasons` list) — gains new seasons over time.
- `showseason.php?season=<id>` (the `season` episode list) — the current
  season gains a new episode every weekday the show airs.

Once fetched, these were cached forever. On this machine `listseasons.php`
was 19 days stale and the in-progress season's page ~17 days stale, so
`jplay` (which shells out to `jarch seasons` / `jarch season <id>`, never
with `--refresh`) could never surface newly aired episodes or seasons.

## Change

`bin/jarch` now applies a staleness TTL to the `seasons` and `season`
subcommands only. On a cache hit, if the cached file is older than
`JARCH_INDEX_TTL` seconds it is re-fetched (reusing the existing fetch path).

- Default TTL: `43200` (12h) — one weekday episode airs per day, so 12h is
  ample freshness while staying within the README's low-volume/anti-scraping
  posture (at most ~2 index re-fetches/day in normal use).
- `JARCH_INDEX_TTL=0` restores the old cache-forever behaviour.
- The TTL is skipped entirely when `JARCH_DIR` is set, keeping offline
  fixture reproduction network-free.
- `game` and `player` remain cache-forever (immutable) — unaffected.

No `jplay` change was needed: its `jarch seasons` / `jarch season` calls now
self-refresh on the next launch after the TTL lapses. (`jplay` does not set
`JARCH_DIR` or a subprocess env, so it inherits the real XDG cache path and
the TTL applies.)

## Files touched

| File | Change |
|---|---|
| `bin/jarch` | TTL staleness check for `seasons`/`season` before the fetch gate |
| `README.md` | Documented `JARCH_INDEX_TTL` |
| `AUDIT.md` | This section |

## Verification

With a fake `curl` on `PATH` (no real network) and cache files planted at
chosen ages under a temp `XDG_CACHE_HOME`:

1. Fresh `season` cache (age 60s) → cache hit, no fetch.
2. Stale `season` cache (age 20h) → re-fetch attempted.
3. Stale `season` cache with `JARCH_INDEX_TTL=0` → cache hit, no fetch.
4. Stale `listseasons` cache (age 20h) → re-fetch attempted.
5. Old `game` cache (age 100 days) → cache hit, no fetch (immutable).

`bash -n bin/jarch` passes.

## Residual risks / notes

- Past (completed) seasons are also index pages, so their cached page will be
  re-fetched at most once per TTL window even though it will not have
  changed. This is a negligible, bounded number of extra low-volume fetches
  and is the price of not parsing the page to decide whether a season is
  still in progress.
- `stat -c %Y` / `date +%s` are GNU/coreutils-specific, consistent with the
  script's existing use of `readlink -f`.

---

# Audit: XDG cache directory for `jarch`

Date: 2026-07-03

## Problem

`bin/jarch` defaulted its cache/output directory to the current working
directory (`CACHE_DIR="${JARCH_DIR:-.}"`). Every invocation littered the
directory it was run from with cached HTML (`listseasons.php`,
`showgame.php?game_id=…`, `showseason.php?season=…`) alongside the JSON
outputs — the repo root itself had accumulated three such files.

## Change

Cache and output locations are now separate:

- **Cached HTML** defaults to `$XDG_CACHE_HOME/jarch/`, falling back to
  `~/.cache/jarch/` when `XDG_CACHE_HOME` is unset. The directory is created
  (`mkdir -p`) on the fetch path only, so a pure cache-hit parse never
  creates directories.
- **JSON outputs** default to the current directory (unchanged behaviour —
  outputs are deliverables, not cache).
- **`JARCH_DIR`**, when set, still overrides *both* directories at once.
  This preserves the spec-fixture workflow documented in the README
  (`JARCH_DIR=spec jarch game 6789 --stdout` reads fixtures from and writes
  next to `spec/`).
- Cache filenames are unchanged (they mirror the site URL path + query), so
  existing cache files are drop-in valid in the new location.

Note: the XDG Base Directory spec's variable is `XDG_CACHE_HOME` (there is
no `XDG_CACHE_DIR` in the spec); the script uses the spec name.

## Files touched

| File | Change |
|---|---|
| `bin/jarch` | Split `CACHE_DIR`/`OUT_DIR`; XDG default; `mkdir -p` before fetch; updated `--help` text |
| `README.md` | Documented new cache location and `JARCH_DIR` override semantics |
| `TODO.md` | Ticked the "cache to ~/.cache" item |
| `AUDIT.md` | This file |

## Data migration

The three stale cache files in the repo root were **moved** (not copied) to
`~/.cache/jarch/`:

- `listseasons.php`
- `showseason.php?season=38`
- `showgame.php?game_id=7154`

The JSON outputs (`listseasons.json`, `season-38.json`, `7154.json`) were
left in the repo root, as outputs continue to belong to the working
directory.

## Verification

All runs used `JARCH_FETCH_DELAY=0` and completed without any network fetch
(cache hits against the migrated files):

1. `jarch seasons`, `jarch game 7154`, and `jarch season 38` run from a
   scratch directory each read HTML from `~/.cache/jarch/`, wrote JSON to
   the scratch cwd, and produced output **byte-identical** (via `jq -S`
   normalised diff) to the pre-change JSON in the repo root.
2. `JARCH_DIR=~/.cache/jarch jarch game 7154 --stdout` confirmed the
   override path still resolves cache correctly and matches the same output.
3. No stray temp files or directories were left in `~/.cache/jarch/` after
   the runs.

## Residual risks / notes

- First fetch on a fresh machine now depends on `$HOME` being set (standard
  in any login environment); `set -u` will fail loudly rather than silently
  mis-cache if it is not.
- The `player` subcommand was not exercised (no cached fixture available);
  it shares the identical cache/fetch/output code path with the verified
  subcommands.
- Packaging (`flake.nix`) needed no change — the script computes paths at
  runtime.
