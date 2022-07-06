package processing

import (
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

type S2SProcessor interface {
	ProcessServerActivity(vocab.Item) (vocab.Item, error)
}

// ProcessServerActivity processes an Activity received in a server to server request
func (p defaultProcessor) ProcessServerActivity(it vocab.Item) (vocab.Item, error) {
	if it == nil {
		return nil, errors.Newf("Unable to process nil activity")
	}

	if _, err := p.s.Save(it); err != nil {
		return it, err
	}

	vocab.OnActivity(it, func(act *vocab.Activity) error {
		var err error
		// TODO(marius): this does not work correctly if act.Object is an ItemCollection
		obType := act.Object.GetType()
		// First we process the activity to effect whatever changes we need to on the activity properties.
		if vocab.ContentManagementActivityTypes.Contains(act.Type) && obType != vocab.RelationshipType {
			act, err = ContentManagementActivity(p.s, act, vocab.Inbox)
		} else if vocab.CollectionManagementActivityTypes.Contains(act.Type) {
			act, err = CollectionManagementActivity(p.s, act)
		} else if vocab.ReactionsActivityTypes.Contains(act.Type) {
			act, err = ReactionsActivity(p, act)
		} else if vocab.EventRSVPActivityTypes.Contains(act.Type) {
			act, err = EventRSVPActivity(p.s, act)
		} else if vocab.GroupManagementActivityTypes.Contains(act.Type) {
			act, err = GroupManagementActivity(p.s, act)
		} else if vocab.ContentExperienceActivityTypes.Contains(act.Type) {
			act, err = ContentExperienceActivity(p.s, act)
		} else if vocab.GeoSocialEventsActivityTypes.Contains(act.Type) {
			act, err = GeoSocialEventsActivity(p.s, act)
		} else if vocab.NotificationActivityTypes.Contains(act.Type) {
			act, err = NotificationActivity(p.s, act)
		} else if vocab.RelationshipManagementActivityTypes.Contains(act.Type) {
			act, err = RelationshipManagementActivity(p, act, vocab.Inbox)
		} else if vocab.NegatingActivityTypes.Contains(act.Type) {
			act, err = NegatingActivity(p.s, act)
		} else if vocab.OffersActivityTypes.Contains(act.Type) {
			act, err = OffersActivity(p.s, act)
		}
		if err != nil {
			return err
		}
		if act.Tag != nil {
			// Try to save tags as set on the activity
			createNewTags(p.s, act.Tag)
		}
		return nil
	})

	if colSaver, ok := p.s.(CollectionStore); ok {
		if _, err := AddToCollections(p, colSaver, it); err != nil {
			return it, err
		}
	}
	return it, nil
}
