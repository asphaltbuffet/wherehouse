# Changelog

## [0.2.0]

### Added

- Add `load` command to add items/locations in bulk

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
