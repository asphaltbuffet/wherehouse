# Wherehouse - Personal Item Location Tracker

## Product Overview

Wherehouse is a CLI/TUI application designed to solve the universal problem of misplaced items. It answers questions like "Where did I put my 10mm socket wrench?", "Who borrowed my pencil?", and "What is missing?" by maintaining a structured inventory of personal items and their locations.

### Core Value Proposition

- **Never lose track of items**: Record and locate any physical item in your home or workspace
- **Understand movement patterns**: Track how items move, who uses them, and when they need to be returned
- **Project-aware tracking**: Associate items with active projects and get reminders when projects complete
- **Smart suggestions**: When items aren't where expected, receive helpful suggestions based on history and context

### Target Users

Individuals who:
- Work with tools and equipment (workshop owners, makers, technicians)
- Frequently lend items to family members or colleagues
- Manage collections or inventories
- Want better organization without complex systems
- Need to track items across projects

## Features and Functionality

### Core Features (v1)

#### 1. Item Management
- **Record Items**: Create item entries with minimum required fields (name + location)
- **Item Attributes**:
  - Name (required)
  - Current location (required)
  - Description (optional)
  - Tags/categories (optional)
  - Current user/owner (attributed automatically, overridable)
  - Creation timestamp
  - Last modified timestamp

#### 2. Location Hierarchy
- **Hierarchical Structure**: Organize locations in a tree structure
  - Root level: Major areas (e.g., "Garage", "Office", "Kitchen")
  - Nested levels: Progressively specific locations (e.g., "Garage > Workbench > Drawer 2")
- **Flexible Placement**: Items typically placed at leaf nodes but may exist at any level
- **Special Locations**:
  - "Missing" - A special root-level location for items whose whereabouts are unknown

#### 3. Movement Tracking
- **Complete History**: Every item movement is recorded with:
  - From location
  - To location
  - Timestamp
  - User who performed the move
  - Movement type (see below)
  - Optional notes
  - Associated project (optional)

- **Movement Types**:
  - **temporary_use**: Item borrowed/moved temporarily, expected to return
  - **rehome**: Permanent relocation of item's default location

#### 4. Project Management
- **First-Class Projects**: Projects are distinct entities with:
  - Name
  - Description
  - State: `active` or `completed`
  - Creation date
  - Completion date (when applicable)
  - Associated user

- **Project Lifecycle**:
  - Create new projects
  - Associate item movements with projects (for temporary_use)
  - Mark projects as completed
  - View "items to return" when project completes
  - No automatic item movement on completion (manual user action)

#### 5. Missing Item Management
- **Explicit Missing State**:
  - Move items to "Missing" location when lost
  - Track when items went missing and who reported it
  - Move items out of Missing when found

#### 6. Search and Discovery
- **Find Items**:
  - Search by name (fuzzy matching)
  - Filter by location
  - Filter by tag/category
  - Filter by current user
  - View items by project

- **Suggestions Engine (v1 - Heuristics Only)**:
  - When item not in expected location, suggest:
    - Last known location
    - Locations with similar items
    - Items currently with active projects
    - Common movement patterns
  - No ML or advanced prediction in v1

#### 7. User Attribution
- **Multi-User Tracking**:
  - Attribution only (no permissions/access control)
  - Default user derived from OS username
  - Override user per command
  - Track which user moved items
  - Track which user has items
  - Track which user created projects

### Interface

#### CLI Commands (Examples)
```bash
# Item management
wherehouse item add "10mm socket" --location "Garage/Toolbox"
wherehouse item move "10mm socket" "Workshop/Bench" --type temporary_use --project "Engine Rebuild"
wherehouse item find "socket"
wherehouse item show "10mm socket"

# Location management
wherehouse location add "Garage/Toolbox/Drawer 1"
wherehouse location list
wherehouse location show "Garage"

# Project management
wherehouse project create "Engine Rebuild"
wherehouse project list --active
wherehouse project complete "Engine Rebuild"
wherehouse project items "Engine Rebuild"

# Missing items
wherehouse item missing "10mm socket"
wherehouse item found "10mm socket" --location "Garage/Toolbox"
wherehouse missing list

# Search and suggestions
wherehouse search "wrench"
wherehouse suggest "10mm socket"
```

#### TUI Interface
- Interactive text-based UI for browsing and managing items
- Tree view of location hierarchy
- Item list with filters
- Project dashboard
- Movement history view

## User Roles and Permissions

### v1: Attribution Only

**Single User Model (Simplified)**:
- No permissions system
- No access control
- All users have full access to all data
- User attribution for tracking purposes only

**User Identification**:
- Default user: Derived from OS username (`$USER` or `%USERNAME%`)
- Override: `--user` flag on commands
- Use cases:
  - Track who borrowed items
  - Track who moved items
  - Track who created projects
  - Attribution in movement history

**Future Considerations** (Out of scope for v1):
- Multi-user permissions
- Read-only users
- Admin capabilities
- User groups

## Data Models

### Entities and Relationships

#### Item
```
Item {
  id: UUID (primary key)
  name: String (required)
  description: String (optional)
  current_location_id: UUID (foreign key -> Location)
  tags: []String (optional)
  created_by: String (username)
  created_at: Timestamp
  updated_at: Timestamp
}
```

**Relationships**:
- `current_location_id` → Location (many-to-one)
- Has many MovementHistory entries
- Referenced by Project through MovementHistory

#### Location
```
Location {
  id: UUID (primary key)
  name: String (required)
  parent_location_id: UUID (foreign key -> Location, nullable)
  full_path: String (computed, for display)
  created_at: Timestamp
  updated_at: Timestamp
}
```

**Special Cases**:
- Root locations have `parent_location_id = NULL`
- "Missing" is a special root-level location

**Relationships**:
- `parent_location_id` → Location (self-referential, tree structure)
- Has many Items (current location)
- Has many MovementHistory entries (from/to locations)

#### MovementHistory
```
MovementHistory {
  id: UUID (primary key)
  item_id: UUID (foreign key -> Item, required)
  from_location_id: UUID (foreign key -> Location, required)
  to_location_id: UUID (foreign key -> Location, required)
  movement_type: Enum[temporary_use, rehome] (required)
  project_id: UUID (foreign key -> Project, optional)
  moved_by: String (username, required)
  moved_at: Timestamp (required)
  notes: String (optional)
}
```

**Relationships**:
- `item_id` → Item (many-to-one)
- `from_location_id` → Location (many-to-one)
- `to_location_id` → Location (many-to-one)
- `project_id` → Project (many-to-one, optional)

#### Project
```
Project {
  id: UUID (primary key)
  name: String (required)
  description: String (optional)
  state: Enum[active, completed] (required)
  created_by: String (username)
  created_at: Timestamp
  completed_at: Timestamp (nullable)
}
```

**Relationships**:
- Referenced by MovementHistory entries
- Can compute "items to return" by querying MovementHistory

### Database Schema Considerations

- **Primary Keys**: UUIDs for all entities
- **Indexes**:
  - Item name (for search)
  - Item current_location_id
  - Location parent_location_id (for tree traversal)
  - MovementHistory item_id (for history queries)
  - MovementHistory project_id (for project items)
  - Project state (for active project queries)

- **Constraints**:
  - Location paths must not create cycles
  - MovementHistory from/to locations must be different
  - Item must reference valid location

## Business Rules

### Item Rules

1. **Item Creation**:
   - Minimum required: name + location
   - Default user: OS username
   - Auto-generate UUID
   - Auto-set created_at, updated_at

2. **Item Naming**:
   - Names need not be unique (can have multiple "screwdriver" items)
   - Search should handle duplicates gracefully

3. **Item Placement**:
   - Items typically placed at leaf nodes
   - Items may be placed at intermediate nodes if intentional
   - System should allow but may suggest leaf placement

### Location Rules

1. **Location Structure**:
   - Hierarchical tree structure
   - Unlimited depth supported
   - Root locations have no parent

2. **Special Locations**:
   - "Missing" is a reserved root-level location
   - Created automatically if not exists
   - Cannot be deleted or renamed

3. **Location Deletion**:
   - Cannot delete locations containing items
   - Must move items first
   - May delete locations with sub-locations if all empty

4. **Location Paths**:
   - Display format: "Parent > Child > Grandchild"
   - Must be acyclic (no location can be its own ancestor)

### Movement Rules

1. **Movement Recording**:
   - All movements recorded in history
   - Cannot delete movement history
   - Original state preserved for audit trail

2. **Movement Types**:
   - **temporary_use**:
     - Expected to return to original location
     - Should be associated with a project (recommended)
     - Completion of project shows these items
   - **rehome**:
     - Permanent relocation
     - Updates item's "home" location concept
     - Not shown in "items to return" lists

3. **Current Location Update**:
   - Item's current_location_id updated immediately on move
   - Movement history entry created simultaneously
   - Both operations atomic (transaction)

### Project Rules

1. **Project Lifecycle**:
   - Projects start in `active` state
   - Can transition to `completed` state
   - Cannot transition back to `active` (one-way)
   - Completion is manual action by user

2. **Project Completion**:
   - When marked complete:
     - Set completed_at timestamp
     - Display "items to return" list (items with temporary_use movements)
     - Do NOT automatically move items
     - User must manually return items

3. **Project Association**:
   - Movements can reference projects (optional)
   - Multiple movements can reference same project
   - Projects can be completed even with unreturned items
   - Completed projects remain queryable

### Missing Item Rules

1. **Missing State**:
   - Moving to "Missing" is a movement type `temporary_use` or `rehome`
   - Missing items shown in special "missing items" list
   - Can move out of Missing when found

2. **Suggestions for Missing**:
   - When item marked missing, record last known location
   - Suggestion engine uses last known location
   - May suggest checking locations with similar items

### Search and Suggestion Rules

1. **Search Behavior**:
   - Name search: fuzzy matching, case-insensitive
   - Location filter: exact match or hierarchical match
   - Tag filter: exact match, multiple tags = OR

2. **Suggestion Engine (v1 Heuristics)**:
   - Triggered when item not in expected location
   - Suggestions based on:
     1. Last known location (from movement history)
     2. Locations containing items with similar names/tags
     3. Active projects with temporary_use movements
     4. Most frequent movement patterns for that item
   - Return max 5 suggestions, ranked by confidence
   - No ML/AI in v1, pure heuristics

### Multi-User Rules

1. **User Attribution**:
   - All actions attributed to a user
   - Default user from OS username
   - Override with --user flag

2. **No Permissions**:
   - All users can perform all actions
   - No read/write restrictions
   - No user authentication

## Storage and Persistence

### Database: SQLite

**Requirements**:
- Single database file
- Path configurable via config file or environment variable
- Default path: `~/.wherehouse/wherehouse.db`
- Must support network storage (NFS, SMB compatible)

**Rationale**:
- SQLite is serverless, requires no setup
- Single file is easy to backup and move
- Supports concurrent reads (important for TUI)
- Sufficient for personal use (single user, moderate item count)
- Works on network storage with caveats (locking)

**Performance Considerations**:
- Expect up to 10,000 items (typical user)
- Indexes on search fields
- Movement history may grow large (acceptable for SQLite)

### Configuration

**Config File Location**:
- `~/.wherehouse/config.toml` or `~/.config/wherehouse/config.toml`
- Allow override with `--config` flag

**Configuration Options**:
```toml
[database]
path = "~/.wherehouse/wherehouse.db"

[user]
default_username = "auto"  # "auto" = use OS username

[ui]
default_view = "tui"  # or "cli"

[suggestions]
max_suggestions = 5
```

## Technology Stack

### Language
- **Go**: For CLI/TUI application
  - Cross-platform
  - Single binary distribution
  - Excellent CLI library ecosystem
  - Strong SQLite support

### Key Libraries
- **CLI Framework**: [cobra](https://github.com/spf13/cobra) - industry standard for Go CLIs
- **TUI Framework**: [bubbletea](https://github.com/charmbracelet/bubbletea) - modern, composable TUI
- **Database**: [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) or [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)
- **UUID**: [google/uuid](https://github.com/google/uuid)
- **Configuration**: [viper](https://github.com/spf13/viper)

### Project Structure
```
wherehouse/
├── cmd/              # CLI commands (cobra)
├── internal/         # Internal packages
│   ├── models/       # Data models
│   ├── database/     # SQLite interactions
│   ├── service/      # Business logic
│   ├── tui/          # TUI components
│   └── suggestions/  # Suggestion engine
├── pkg/              # Public packages (if any)
├── migrations/       # Database migrations
├── config/           # Configuration handling
└── main.go          # Entry point
```

## User Experience Principles

1. **Quick Entry**: Common operations should require minimal keystrokes
   - `wh add item "name" loc` (short aliases)
   - Interactive prompts for missing required fields

2. **Forgiving Search**: Fuzzy matching, typo tolerance
   - "screw" matches "screwdriver"
   - "garag" matches "Garage"

3. **Helpful Defaults**: Sensible defaults reduce cognitive load
   - Current user from OS username
   - Common movement type: temporary_use
   - Suggest leaf locations for new items

4. **Transparent History**: Users can always see why/when items moved
   - Movement history always accessible
   - Show username and timestamp

5. **No Data Loss**: Never delete data, only mark as missing/completed
   - Movement history permanent
   - Projects remain after completion
   - Items never deleted, only moved to Missing

## Explicitly Out of Scope (v1)

The following features are explicitly deferred to future versions:

### Not in v1
- ❌ Permissions and access control
- ❌ Cloud accounts or synchronization
- ❌ Image attachments for items
- ❌ Barcode/QR code scanning
- ❌ Mobile application
- ❌ AI/ML beyond simple heuristics
- ❌ Web interface
- ❌ Import/export (beyond database backup)
- ❌ Notifications or reminders
- ❌ Multi-language support
- ❌ Item value tracking
- ❌ Borrowing/lending workflows (formal)
- ❌ Integration with external systems

## Success Criteria

Version 1.0 is successful when:

1. ✅ User can add items with location in <10 seconds
2. ✅ User can find any item in <5 seconds
3. ✅ User can move items and see movement history
4. ✅ User can create projects and associate item movements
5. ✅ User can complete projects and see items to return
6. ✅ User can mark items missing and get suggestions
7. ✅ Database persists reliably on local and network storage
8. ✅ TUI is responsive and intuitive for common operations
9. ✅ CLI commands are well-documented and consistent
10. ✅ Application works on Linux, macOS, and Windows

## Future Considerations (Post-v1)

Potential features for future versions:

- **v1.1**: Import/export functionality (CSV, JSON)
- **v1.2**: Basic reporting (most moved items, most used projects)
- **v2.0**: Web interface for remote access
- **v2.1**: Image attachments
- **v2.2**: Barcode scanning integration
- **v3.0**: Mobile companion app
- **v3.1**: Cloud sync (optional)

## Development Phases

### Phase 1: Core Foundation
- Database schema and migrations
- Basic models and data layer
- CLI framework setup

### Phase 2: Core Commands
- Item CRUD operations
- Location CRUD operations
- Basic search functionality

### Phase 3: Movement Tracking
- Movement history recording
- Movement type support
- History queries

### Phase 4: Projects
- Project CRUD operations
- Project-movement associations
- Project completion flow

### Phase 5: Suggestions
- Missing item support
- Basic suggestion heuristics
- Suggestion ranking

### Phase 6: TUI
- Interactive TUI interface
- Tree views for locations
- List views for items
- Project dashboard

### Phase 7: Polish
- Configuration system
- Documentation
- Testing
- Release packaging

## Questions for Future Clarification

These questions can be resolved during implementation:

1. **Location Deletion**: Should we allow "archiving" locations instead of deletion?
2. **Item Quantities**: Should items support quantity tracking (e.g., "screws")?
3. **Bulk Operations**: Should we support moving multiple items at once?
4. **Undo**: Should movement operations be undoable?
5. **Tags vs Categories**: Should we have both, or just tags?
6. **Export**: What formats for backup/export (JSON, CSV, SQL dump)?
7. **Search Ranking**: How should search results be ranked?
8. **Performance**: At what item count should we optimize/paginate?

---

**Document Version**: 1.0
**Last Updated**: 2026-02-19
**Status**: Initial Design
