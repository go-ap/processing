package processing

import (
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// C2SProcessor
type C2SProcessor interface {
	ProcessClientActivity(vocab.Item) (vocab.Item, error)
}

// ProcessClientActivity processes an Activity received in a client to server request
func (p defaultProcessor) ProcessClientActivity(it vocab.Item) (vocab.Item, error) {
	if it == nil {
		return nil, errors.Newf("Unable to process nil activity")
	}
	if vocab.IntransitiveActivityTypes.Contains(it.GetType()) {
		return processClientIntransitiveActivity(p, it)
	}
	return it, vocab.OnActivity(it, func(act *vocab.Activity) error {
		var err error
		it, err = processClientActivity(p, act)
		return err
	})
}

func processClientIntransitiveActivity(p defaultProcessor, it vocab.Item) (vocab.Item, error) {
	if len(it.GetLink()) == 0 {
		if err := SetID(it, nil, nil); err != nil {
			return it, err
		}
	}
	typ := it.GetType()
	if vocab.QuestionActivityTypes.Contains(typ) {
		err := vocab.OnQuestion(it, func(q *vocab.Question) error {
			var err error
			q, err = QuestionActivity(p.s, q)
			return err
		})
		if err != nil {
			return it, err
		}
	}
	err := vocab.OnIntransitiveActivity(it, func(act *vocab.IntransitiveActivity) error {
		var err error
		if vocab.GeoSocialEventsActivityTypes.Contains(typ) {
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

	if it, err = p.s.Save(vocab.FlattenProperties(it)); err != nil {
		return it, err
	}
	if colSaver, ok := p.s.(CollectionStore); ok {
		if it, err = AddToCollections(p, colSaver, it); err != nil {
			p.infoFn("error: %s", err)
		}
	}
	return it, nil
}

func processClientActivity(p defaultProcessor, act *vocab.Activity) (*vocab.Activity, error) {
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
	if vocab.ContentManagementActivityTypes.Contains(act.Type) && obType != vocab.RelationshipType {
		act, err = ContentManagementActivity(p.s, act, vocab.Outbox)
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
		act, err = RelationshipManagementActivity(p, act, vocab.Outbox)
	} else if vocab.NegatingActivityTypes.Contains(act.Type) {
		act, err = NegatingActivity(p.s, act)
	} else if vocab.OffersActivityTypes.Contains(act.Type) {
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

	var it vocab.Item
	if act.Content != nil || act.Summary != nil {
		// For activities that have a content value, we create the collections that allow actors to interact
		// with them as they are a regular object.
		vocab.OnObject(act, addNewObjectCollections)
	}

	// Making a local copy of the activity in order to not lose information that could be required
	// later in the call system.
	toSave := *act

	it, err = p.s.Save(vocab.FlattenProperties(&toSave))
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
