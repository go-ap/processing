package processing

import (
	"time"

	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
)

// NOTE(marius): this should be moved to the handlers package, where we are actually
//  interested in its functionality
type C2SProcessor interface {
	ProcessClientActivity(pub.Item) (pub.Item, error)
}

// ProcessClientActivity processes an Activity received in a client to server request
func (p defaultProcessor) ProcessClientActivity(it pub.Item) (pub.Item, error) {
	if it == nil {
		return nil, errors.Newf("Unable to process nil activity")
	}
	if pub.IntransitiveActivityTypes.Contains(it.GetType()) {
		return it, pub.OnIntransitiveActivity(it, func(act *pub.IntransitiveActivity) error {
			var err error
			it, err = processClientIntransitiveActivity(p, act)
			return err
		})
	}
	return it, pub.OnActivity(it, func(act *pub.Activity) error {
		var err error
		it, err = processClientActivity(p, act)
		return err
	})
}

func processClientIntransitiveActivity(p defaultProcessor, act *pub.IntransitiveActivity) (*pub.IntransitiveActivity, error) {
	iri := act.GetLink()
	if len(iri) == 0 {
		if err := SetID(act, handlers.Outbox.IRI(act.Actor), act); err != nil {
			return act, nil
		}
	}
	var err error
	if pub.QuestionActivityTypes.Contains(act.Type) {
		act, err = QuestionActivity(p.s, act)
	} else if pub.GeoSocialEventsActivityTypes.Contains(act.Type) {
		act, err = GeoSocialEventsIntransitiveActivity(p.s, act)
	}
	if err != nil {
		return act, err
	}

	if act.Published.IsZero() {
		act.Published = time.Now().UTC()
	}

	var it pub.Item
	it, err = p.s.Save(pub.FlattenProperties(act))
	if err != nil {
		return act, err
	}
	if colSaver, ok := p.s.(s.CollectionStore); ok {
		it, err = AddToCollections(p, colSaver, it)
	}
	return act, nil
}

func processClientActivity(p defaultProcessor, act *pub.Activity) (*pub.Activity, error) {
	if iri := act.GetLink(); len(iri) == 0 {
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
		act, err = ContentManagementActivity(p.s, act, handlers.Outbox)
	} else if pub.CollectionManagementActivityTypes.Contains(act.Type) {
		act, err = CollectionManagementActivity(p.s, act)
	} else if pub.ReactionsActivityTypes.Contains(act.Type) {
		act, err = ReactionsActivity(p.s, act)
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
		act, err = RelationshipManagementActivity(p.s, act)
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

	if colSaver, ok := p.s.(s.CollectionStore); ok {
		if it, err = AddToCollections(p, colSaver, it); err != nil {
			return act, err
		}
	}
	return act, nil
}
