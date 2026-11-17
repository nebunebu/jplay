# TODO

- [x] Cache fetched data under `~/.cache` — done: defaults to
      `$XDG_CACHE_HOME/jarch/` (see AUDIT.md); configurable via jplay's
      `jarch_cache_dir`.
- [x] Season id vs. episode number is clunky for navigation — done:
      `jplay --fetch game <season_id>/<episode>` (e.g. `42/9600`, `pcj/49`)
      resolves the `game_id` from the season page; the TUI browser already
      navigates by season → episode.
- [ ] Extend the tournament round vocabulary (`roundRE` in `jarch/common.go`)
      as `no round keyword recognised` warnings turn up.
