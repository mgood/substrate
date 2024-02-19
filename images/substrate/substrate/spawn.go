package substrate

import (
	"context"
	"time"

	"github.com/ajbouh/substrate/images/substrate/activityspec"
	"github.com/ajbouh/substrate/images/substrate/fs"

	ulid "github.com/oklog/ulid/v2"
)

func (s *Substrate) WriteSpawn(
	ctx context.Context,
	req *activityspec.ServiceSpawnRequest,
	views *activityspec.ServiceSpawnResolution,
	res *activityspec.ActivitySpawnResponse,
) error {
	var err error

	var spaces = []*Space{}
	entropy := ulid.DefaultEntropy()
	now := time.Now()
	nowTs := ulid.Timestamp(now)

	visitSpace := func(viewName string, multi bool, view *substratefs.SpaceView) error {
		spaceID := view.Tip.SpaceID.String()

		if view.Creation != nil {
			var forkedFromRef *string
			var forkedFromID *string
			baseRef := view.Creation.Base
			if baseRef != nil {
				base := baseRef.String()
				baseID := baseRef.TipRef.SpaceID.String()
				forkedFromRef = &base
				forkedFromID = &baseID
			}
			spaces = append(spaces, &Space{
				Owner:         req.User,
				Alias:         spaceID, // initial alias is just the ID itself
				ID:            spaceID,
				ForkedFromRef: forkedFromRef,
				ForkedFromID:  forkedFromID,
				CreatedAt:     now,
			})
		}

		err := s.WriteCollectionMembership(ctx, &CollectionMembership{
			Owner:       "system",
			Name:        "spawn",
			SpaceID:     spaceID,
			ServiceSpec: req.ServiceName,
			CreatedAt:   now,
			IsPublic:    true,
			Attributes:  map[string]any{},
		})
		return err
	}

	for viewName, view := range views.Parameters {
		switch {
		case view.Space != nil:
			err = visitSpace(viewName, false, view.Space)
			if err != nil {
				return err
			}
		case view.Spaces != nil:
			for _, v := range *view.Spaces {
				err = visitSpace(viewName, true, &v)
				if err != nil {
					return err
				}
			}
		}
	}

	eventULID := ulid.MustNew(nowTs, entropy)
	eventID := "ev-" + eventULID.String()
	viewspecReq, _ := req.Format()
	err = s.WriteEvent(ctx, &Event{
		ID:        eventID,
		Type:      "spawn",
		Timestamp: now,
		// Parameters:       req.ActivitySpec.Parameters,
		ActivitySpec: viewspecReq,
		User:         req.User,
		Service:      req.ServiceName,
		Response:     res,
	})
	if err != nil {
		return err
	}

	for _, sp := range spaces {
		err = s.WriteSpace(ctx, sp)
		if err != nil {
			return err
		}
	}

	viewspec, _ := views.Format()

	if !req.Ephemeral {
		err = s.WriteActivity(ctx, &Activity{
			ActivitySpec: viewspec,
			CreatedAt:    now,
			Service:      req.ServiceName,
		})
		if err != nil {
			return err
		}
	}

	return nil
}
