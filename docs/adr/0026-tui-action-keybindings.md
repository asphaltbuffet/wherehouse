# TUI action keybindings avoid vim-navigation conflicts

The TUI uses `h`/`j`/`k`/`l` for vim-style navigation (back/down/up/open). When adding action keybindings for `add`, `loan`, `borrow`, `lost`, `return`, `found`, `history`, and `scry`, mnemonic single-key assignments for `loan` (`l`) and `history` (`h`) would directly conflict with DrillIn and DrillOut. We resolved this by using shifted variants (`L` for loan, `H` for history) rather than dropping the vim navigation aliases.

## Keybinding table

| Key | Action | Gate |
|---|---|---|
| `a` | add | selection exists and entity is not `discrete` |
| `L` | loan | selection is `ok` or `missing`, not `locked` |
| `b` | borrow | selection exists |
| `x` | lost | selection is `ok`, not `locked` |
| `r` | return | selection is `loaned` or `borrowed` |
| `f` | found | selection is `missing` |
| `H` | history | selection exists (ungated) |
| `s` | scry | always (no selection required) |
| `d` | toggle detail pane | selection exists |

`/` remains the bubbles/list local filter (current level only). `s` opens the inventory-wide Levenshtein search (`scry`). These are intentionally distinct — `/` filters what is already visible; `s` searches the whole inventory.

## Considered Options

**Drop `h`/`l` nav aliases, assign `h`=history and `l`=loan directly.** Rejected — users who navigate with the vim home row would silently lose DrillIn/DrillOut. Arrow keys remain functional but breaking muscle memory without warning is a poor trade for saving one shift key.
