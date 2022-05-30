package processing

import (
	"time"

	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// C2SProcessor
type C2SProcessor interface {
	ProcessClientActivity(pub.Item) (pub.Item, error)
}

// ProcessClientActivity processes an Activity received in a client to server request
func (p defaultProcessor) ProcessClientActivity(it pub.Item) (pub.Item, error) {
	if it == nil {
		return nil, errors.Newf("Unable to process nil activity")
	}
	if pub.IntransitiveActivityTypes.Contains(it.GetType()) {
		return processClientIntransitiveActivity(p, it)
	}
	return it, pub.OnActivity(it, func(act *pub.Activity) error {
		var err error
		it, err = processClientActivity(p, act)
		return err
	})
}

func processClientIntransitiveActivity(p defaultProcessor, it pub.Item) (pub.Item, error) {
	if len(it.GetLink()) == 0 {
		if err := SetID(it, nil, nil); err != nil {
			return it, err
		}
	}
	typ := it.GetType()
	if pub.QuestionActivityTypes.Contains(typ) {
		err := pub.OnQuestion(it, func(q *pub.Question) error {
			var err error
			q, err = QuestionActivity(p.s, q)
			return err
		})
		if err != nil {
			return it, err
		}
	}
	err := pub.OnIntransitiveActivity(it, func(act *pub.IntransitiveActivity) error {
		var err error
		if pub.GeoSocialEventsActivityTypes.Contains(typ) {
			act, err = GeoSocialEventsIntransitiveActivity(p.s, act)
		}
		if err != nil {
			return err
		}
		if act.Published.IsZero() {
			act.Published = time.Now().UTC()
		}
		return nil
	})
	if err != nil {
		return it, err
	}

	if it, err = p.s.Save(pub.FlattenProperties(it)); err != nil {
		return it, err
	}
	if colSaver, ok := p.s.(CollectionStore); ok {
		if it, err = AddToCollections(p, colSaver, it); err != nil {
			p.infoFn("error: %s", err)
		}
	}
	return it, nil
}

func processClientActivity(p defaultProcessor, act *pub.Activity) (*pub.Activity, error) {
	if len(act.GetLink()) == 0 {
		if err := SetID(act, nil, nil); err != nil {
			return act, err
		}
	}
	var err error

	if act.Object == nil {
		return act, errors.BadRequestf("Invalid %s: object is nil", act.Type)
	}

	// TODO(marius): this does not work correctly if act.Object is an ItemCollection
	obType := act.Object.GetType()
	// First we process the activity to effect whatever changes we need to on the activity properties.
	if pub.ContentManagementActivityTypes.Contains(act.Type) && obType != pub.RelationshipType {
		act, err = ContentManagementActivity(p.s, act, pub.Outbox)
	} else if pub.CollectionManagementActivityTypes.Contains(act.Type) {
		act, err = CollectionManagementActivity(p.s, act)
	} else if pub.ReactionsActivityTypes.Contains(act.Type) {
		act, err = ReactionsActivity(p, act)
	} else if pub.EventRSVPActivityTypes.Contains(act.Type) {
		act, err = EventRSVPActivity(p.s, act)
	} else if pub.GroupManagementActivityTypes.Contains(act.Type) {
		act, err = GroupManagementActivity(p.s, act)
	} else if pub.ContentExperienceActivityTypes.Contains(act.Type) {
		act, err = ContentExperienceActivity(p.s, act)
	} else if pub.GeoSocialEventsActivityTypes.Contains(act.Type) {
		act, err = GeoSocialEventsActivity(p.s, act)
	} else if pub.NotificationActivityTypes.Contains(act.Type) {
		act, err = NotificationActivity(p.s, act)
	} else if pub.RelationshipManagementActivityTypes.Contains(act.Type) {
		act, err = RelationshipManagementActivity(p, act)
	} else if pub.NegatingActivityTypes.Contains(act.Type) {
		act, err = NegatingActivity(p.s, act)
	} else if pub.OffersActivityTypes.Contains(act.Type) {
		act, err = OffersActivity(p.s, act)
	}
	if err != nil {
		return act, err
	}
	if act.Tag != nil {
		// Try to save tags as set on the activity
		createNewTags(p.s, act.Tag)
	}

	if act.Published.IsZero() {
		act.Published = time.Now().UTC()
	}

	var it pub.Item
	if act.Content != nil || act.Summary != nil {
		// For activities that have a content value, we create the collections that allow actors to interact
		// with them as they are a regular object.
		pub.OnObject(act, addNewObjectCollections)
	}

	// Making a local copy of the activity in order to not lose information that could be required
	// later in the call system.
	toSave := *act

	it, err = p.s.Save(pub.FlattenProperties(&toSave))
	if err != nil {
		return act, err
	}

	if colSaver, ok := p.s.(CollectionStore); ok {
		if it, err = AddToCollections(p, colSaver, it); err != nil {
			return act, err
		}
	}
	return act, nil
}
