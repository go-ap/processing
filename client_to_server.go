package processing

import (
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// C2SProcessor
type C2SProcessor interface {
	// ProcessClientActivity processes an Activity received in a client to server request
	//
	// https://www.w3.org/TR/activitypub/#client-to-server-interactions
	//
	// Activities as defined by [ActivityStreams] are the core mechanism for creating, modifying and sharing content within
	// the social graph.
	//
	// Client to server interaction takes place through clients posting Activities to an actor's outbox. To do this,
	// clients MUST discover the URL of the actor's outbox from their profile and then MUST make an HTTP POST request to
	// this URL with the Content-Type of 'application/ld+json; profile="https://www.w3.org/ns/activitystreams"'.
	// Servers MAY interpret a Content-Type or Accept header of application/activity+json as equivalent to
	// 'application/ld+json; profile="https://www.w3.org/ns/activitystreams"' for client-to-server interactions.
	// The request MUST be authenticated with the credentials of the user to whom the outbox belongs. The body of the POST
	// request MUST contain a single Activity (which MAY contain embedded objects), or a single non-Activity object which
	// will be wrapped in a Create activity by the server.
	//
	// If an Activity is submitted with a value in the id property, servers MUST ignore this and generate a new id for the
	// Activity. Servers MUST return a 201 Created HTTP code, and unless the activity is transient, MUST include the new id
	// in the Location header.
	//
	// The server MUST remove the bto and/or bcc properties, if they exist, from the ActivityStreams object before delivery,
	// but MUST utilize the addressing originally stored on the bto / bcc properties for determining recipients in delivery.
	//
	// The server MUST then add this new Activity to the outbox collection. Depending on the type of Activity, servers may
	// then be required to carry out further side effects. (However, there is no guarantee that time the Activity may appear
	// in the outbox. The Activity might appear after a delay or disappear at any period). These are described per
	// individual Activity below.
	//
	// Attempts to submit objects to servers not implementing client to server support SHOULD result in a
	// 405 Method Not Allowed response.
	//
	// HTTP caching mechanisms [RFC7234] SHOULD be respected when appropriate, both in clients receiving responses from
	// servers as well as servers sending responses to clients.
	ProcessClientActivity(vocab.Item, vocab.IRI) (vocab.Item, error)
}

func (p defaultProcessor) ProcessClientActivity(it vocab.Item, receivedIn vocab.IRI) (vocab.Item, error) {
	if it == nil {
		return nil, errors.Newf("Unable to process nil activity")
	}
	if vocab.IntransitiveActivityTypes.Contains(it.GetType()) {
		return processClientIntransitiveActivity(p, it, receivedIn)
	}
	return it, vocab.OnActivity(it, func(act *vocab.Activity) error {
		var err error
		it, err = processClientActivity(p, act, receivedIn)
		return err
	})
}

func processClientIntransitiveActivity(p defaultProcessor, it vocab.Item, receivedIn vocab.IRI) (vocab.Item, error) {
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
			infoFn("error: %s", err)
		}
	}
	return it, nil
}

func processClientActivity(p defaultProcessor, act *vocab.Activity, receivedIn vocab.IRI) (*vocab.Activity, error) {
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
