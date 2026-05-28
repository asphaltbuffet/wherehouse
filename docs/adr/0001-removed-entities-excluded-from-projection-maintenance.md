# Removed entities are excluded from projection maintenance

Removed entities (`status = 'removed'`) are excluded from all store read queries and from path-propagation updates triggered by rename or reparent events. `entities_current` is only kept consistent for non-removed entities; removed rows may hold stale `full_path_display`/`full_path_canonical` values after an ancestor is renamed or moved.

We chose this because removed entities are invisible to the user and to all application queries, so the cost of keeping their projection current has no benefit. If a "restore" feature is ever added, the restore handler must recompute and emit an `EntityPathChangedEvent` for the restored entity at that point, ensuring correct paths are written before the entity becomes visible again.
