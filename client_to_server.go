package processing

import (
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// C2SProcessor
type C2SProcessor interface {
	ProcessClientActivity(vocab.Item, vocab.IRI) (vocab.Item, error)
}

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
func (p P) ProcessClientActivity(it vocab.Item, receivedIn vocab.IRI) (vocab.Item, error) {
	if it == nil {
		return nil, errors.Newf("Unable to process nil activity")
	}
	// NOTE(marius): the separation between transitive and intransitive activities overlaps the separation we're
	// using in the processingClientActivity function between the ActivityStreams motivations separation.
	// This means that 'it' should probably be treated as a vocab.Item until the last possible moment.
	if vocab.IntransitiveActivityTypes.Contains(it.GetType()) {
		return processClientIntransitiveActivity(p, it, receivedIn)
	}
	return it, vocab.OnActivity(it, func(act *vocab.Activity) error {
		var err error
		it, err = processClientActivity(p, act, receivedIn)
		return err
	})
}

func processClientIntransitiveActivity(p P, it vocab.Item, receivedIn vocab.IRI) (vocab.Item, error) {
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
	recipients, err := p.BuildRecipientsList(it, receivedIn)
	if err != nil {
		infoFn("error: %s", err)
	}
	if err := p.AddToLocalCollections(it, recipients); err != nil {
		errFn("error: %s", err)
	}
	if err := p.AddToRemoteCollections(it, recipients); err != nil {
		errFn("error: %s", err)
	}
	return it, nil
}

func processClientActivity(p P, act *vocab.Activity, receivedIn vocab.IRI) (vocab.Item, error) {
	if len(act.GetLink()) == 0 {
		if err := SetID(act, nil, nil); err != nil {
			return act, err
		}
	}
	if act.Object == nil {
		return act, errors.BadRequestf("Invalid %s: object is nil", act.Type)
	}

	var err error
	typ := act.GetType()
	// TODO(marius): this does not work correctly if act.Object is an ItemCollection
	//  First we process the activity to effect whatever changes we need to on the activity properties.
	switch {
	case vocab.ContentManagementActivityTypes.Contains(typ) && act.Object.GetType() != vocab.RelationshipType:
		act, err = ContentManagementActivity(p.s, act, receivedIn)
	case vocab.CollectionManagementActivityTypes.Contains(typ):
		act, err = CollectionManagementActivity(p.s, act)
	case vocab.ReactionsActivityTypes.Contains(typ):
		act, err = ReactionsActivity(p, act, receivedIn)
	case vocab.EventRSVPActivityTypes.Contains(typ):
		act, err = EventRSVPActivity(p.s, act)
	case vocab.GroupManagementActivityTypes.Contains(typ):
		act, err = GroupManagementActivity(p.s, act)
	case vocab.ContentExperienceActivityTypes.Contains(typ):
		act, err = ContentExperienceActivity(p.s, act)
	case vocab.GeoSocialEventsActivityTypes.Contains(typ):
		act, err = GeoSocialEventsActivity(p.s, act)
	case vocab.NotificationActivityTypes.Contains(typ):
		act, err = NotificationActivity(p.s, act)
	case vocab.RelationshipManagementActivityTypes.Contains(typ):
		act, err = RelationshipManagementActivity(p, act, receivedIn)
	case vocab.NegatingActivityTypes.Contains(typ):
		act, err = NegatingActivity(p.s, act)
	case vocab.OffersActivityTypes.Contains(typ):
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

	recipients, err := p.BuildRecipientsList(act, receivedIn)
	if err != nil {
		return act, err
	}
	localCollections, err := p.BuildAdditionalCollections(act)
	if err != nil {
		errFn("error: %s", err)
	}
	recipients = append(recipients, localCollections...)

	// Making a local copy of the activity in order to not lose information that could be required
	// later in the call system.
	toSave := *act
	it, err = p.s.Save(vocab.FlattenProperties(&toSave))
	if err != nil {
		return act, err
	}
	if err := p.AddToLocalCollections(it, recipients); err != nil {
		errFn("error: %s", err)
	}
	if err := p.AddToRemoteCollections(it, recipients); err != nil {
		errFn("error: %s", err)
	}
	return act, nil
}
