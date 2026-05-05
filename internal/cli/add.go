package cli

import (
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/nanoid"
)

// AddItem adds new entities to the database.
func (wh *App) AddItem(e entitypath.Path, t database.EntityType) (*database.Entity, error) {
	// split location from item
	loc := e.Dir()
	name := e.Base()

	if name == "" {
		return nil, fmt.Errorf("cannot determine item name from %q", e)
	}

	parentID, err := wh.GetID(loc)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve parent %q: %w", loc, err)
	}

	entityID, err := nanoid.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create id for %q: %w", e, err)
	}

	payload := map[string]any{
		"entity_id":    entityID,
		"display_name": name,
		"entity_type":  t.String(),
		"parent_id":    parentID,
	}

	_, err = wh.appendEvent(database.EntityCreatedEvent, payload)
	if err != nil {
		return nil, fmt.Errorf("adding entity %q: %w", e, err)
	}

	return wh.database.GetEntity(wh.ctx, entityID)
}

// GetID returns the nanoid of an entity. Returns an error if there is no match or if there are multiple results.
func (wh *App) GetID(p entitypath.Path) (string, error) {
	c := database.CanonicalizeString(p.String())

	matches, err := wh.database.GetEntitiesByCanonicalName(wh.ctx, c)
	if err != nil {
		return "", err
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("could not find %q", c)
	case 1:
		return matches[0].EntityID, nil
	default:
		return "", fmt.Errorf("ambiguous entity %q: %d matches", c, len(matches))
	}
}

func (wh *App) appendEvent(ee database.EventType, payload any) (int64, error) {
	return wh.database.AppendEvent(wh.ctx, ee, wh.actor, payload, "")
}
