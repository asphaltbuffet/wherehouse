# Changelog

## [0.2.0] - 2026-05-24

### Added

- Add `serve` command to expose a local web UI for browsing entities ([`9c4ad3e`](https://github.com/asphaltbuffet/wherehouse/commit/9c4ad3e))
- Add `rename` command to rename items and locations ([`b122e03`](https://github.com/asphaltbuffet/wherehouse/commit/b122e03))
- Add `status` command to manage entity lifecycle (active/inactive) ([`04a1b34`](https://github.com/asphaltbuffet/wherehouse/commit/04a1b34))
- Add `internal/web` package with HTTP handlers, routing, and embedded assets ([`0a19ac9`](https://github.com/asphaltbuffet/wherehouse/commit/0a19ac9))
- Add `internal/eventbus` package for event dispatch and path propagation ([`afa93d2`](https://github.com/asphaltbuffet/wherehouse/commit/afa93d2))
- Add `internal/inventory` package with unified `Entity`/`Event` domain types ([`28001313`](https://github.com/asphaltbuffet/wherehouse/commit/28001313))
- Add `internal/entitypath` package for colon-separated path parsing ([`c6b093d`](https://github.com/asphaltbuffet/wherehouse/commit/c6b093d))
- Add `internal/store` package replacing legacy database layer ([`974a7d7`](https://github.com/asphaltbuffet/wherehouse/commit/974a7d7))
- Add `internal/app` business logic layer (`App` struct, path-based operations) ([`c8c0b41`](https://github.com/asphaltbuffet/wherehouse/commit/c8c0b41))

### Changed

- Unify items and locations into a single **entity** model ([`3947f709`](https://github.com/asphaltbuffet/wherehouse/commit/3947f709))
- Migrate all commands (`add`, `move`, `remove`, `list`, `scry`, `history`, `status`, `rename`) to the new `*app.App` layer ([`6eac1560`](https://github.com/asphaltbuffet/wherehouse/commit/6eac1560))
- Squash historical migrations 001–006 into a single initial schema ([`349f5926`](https://github.com/asphaltbuffet/wherehouse/commit/349f5926))
- Refactor all commands to eliminate singleton constructors and reduce duplication ([`38e47f8b`](https://github.com/asphaltbuffet/wherehouse/commit/38e47f8b))

### Fixed

- Fix Nix flake fileset to include `internal/web/assets` for `go:embed` ([`6e43a21`](https://github.com/asphaltbuffet/wherehouse/commit/6e43a21))
- Fix root-entity detection in web UI to use `FullPathDisplay` instead of `CanonicalName` ([`83e3f2a`](https://github.com/asphaltbuffet/wherehouse/commit/83e3f2a))
- Fix eventbus path-propagation guard, `LIKE` escaping, and error wrapping ([`5312677`](https://github.com/asphaltbuffet/wherehouse/commit/5312677))

### Removed

- Remove `cmd/migrate` command and legacy `internal/database` package ([`ce6b821`](https://github.com/asphaltbuffet/wherehouse/commit/ce6b821))
- Remove `lost`, `found`, and `loan` commands (superseded by `status`) ([`9e5f47c`](https://github.com/asphaltbuffet/wherehouse/commit/9e5f47c))

## [0.1.0] - 2026-02-25

_First release._

### Added

- Add tab completion for location flags (`--in`, `--to`) in `add`, `found`, and `move` ([`8f355be`](https://github.com/asphaltbuffet/wherehouse/commit/8f355be))
- Add `remove` command to retire items from tracking ([`7c4ff22`](https://github.com/asphaltbuffet/wherehouse/commit/7c4ff22))
- Add `found` command to mark a missing item as located ([`c3ec199`](https://github.com/asphaltbuffet/wherehouse/commit/c3ec199))
- Add `list` command to display all tracked items and their current locations ([`85a5532`](https://github.com/asphaltbuffet/wherehouse/commit/85a5532))
- Add `initialize` command to set up a new wherehouse database ([`adee3ae`](https://github.com/asphaltbuffet/wherehouse/commit/adee3ae))
- Add `move` command to relocate items between locations ([`53a8ca7`](https://github.com/asphaltbuffet/wherehouse/commit/53a8ca7))
- Add `scry` command to find likely locations for missing items ([`04cd914`](https://github.com/asphaltbuffet/wherehouse/commit/04cd914))
- Add `loan` command to record items lent to others ([`fc671b5`](https://github.com/asphaltbuffet/wherehouse/commit/fc671b5))
- Add `lost` command to mark items as missing ([`7939e3e`](https://github.com/asphaltbuffet/wherehouse/commit/7939e3e))
- Add `history` command to view an item's location history ([`928b96c`](https://github.com/asphaltbuffet/wherehouse/commit/928b96c))
- Add `find` command to look up where an item is stored ([`982000f`](https://github.com/asphaltbuffet/wherehouse/commit/982000f))
- Add `add` command to register new items at a location ([`d0a7191`](https://github.com/asphaltbuffet/wherehouse/commit/d0a7191))
- Add `--json` global flag for machine-readable output on all commands ([`456c4e6`](https://github.com/asphaltbuffet/wherehouse/commit/456c4e6))
- Add XDG-compliant configuration and log file locations ([`760f2db`](https://github.com/asphaltbuffet/wherehouse/commit/760f2db))

### Changed

- Use NanoID instead of UUID for item identifiers (shorter, more readable) ([`4772974`](https://github.com/asphaltbuffet/wherehouse/commit/4772974))
- Unify output styling across all commands ([`0e24808`](https://github.com/asphaltbuffet/wherehouse/commit/0e24808))

[0.2.0]: https://github.com/asphaltbuffet/wherehouse/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/asphaltbuffet/wherehouse/compare/v0.0.0...v0.1.0
