package processing

import (
	"time"

	vocab "github.com/go-ap/activitypub"
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
func (p P) ProcessClientActivity(it vocab.Item, author vocab.Actor, receivedIn vocab.IRI) (vocab.Item, error) {
	if vocab.IsNil(it) {
		return nil, InvalidActivity("is nil")
	}

	if err := p.ValidateClientActivity(it, author, receivedIn); err != nil {
		return it, err
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

// ProcessOutboxDelivery
//
// # Outbox Delivery Requirements for Server to Server
//
// https://www.w3.org/TR/activitypub/#outbox-delivery
//
// When objects are received in the outbox (for servers which support both Client to Server interactions and
// Server to Server Interactions), the server MUST target and deliver to:
//
// The to, bto, cc, bcc or audience fields if their values are individuals or Collections owned by the actor.
// These fields will have been populated appropriately by the client which posted the Activity to the outbox.
//
// Additional recommendation from the ActivityPub mailing list: Activities addressed to `Public` usually appear
// only in the inboxes of actors that follow the activity's `actor` property.
func (p P) ProcessOutboxDelivery(it vocab.Item, receivedIn vocab.IRI) error {
	recipients, err := p.BuildOutboxRecipientsList(it, receivedIn)
	if err != nil {
		p.l.Warnf("%+s", err)
		return nil
	}
	if err := p.AddToLocalCollections(it, recipients...); err != nil {
		p.l.Errorf("%+s", err)
	}
	if err := p.AddToRemoteCollections(it, recipients...); err != nil {
		p.l.Errorf("%+s", err)
	}

	return nil
}

func processClientIntransitiveActivity(p P, it vocab.Item, receivedIn vocab.IRI) (vocab.Item, error) {
	if len(it.GetLink()) == 0 {
		if err := SetIDIfMissing(it, receivedIn, nil); err != nil {
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

	return it, p.ProcessOutboxDelivery(it, receivedIn)
}

func processClientActivity(p P, act *vocab.Activity, receivedIn vocab.IRI) (vocab.Item, error) {
	if len(act.GetLink()) == 0 {
		if err := SetIDIfMissing(act, receivedIn, nil); err != nil {
			return act, err
		}
	}
	if vocab.IsNil(act.Object) {
		return act, InvalidActivityObject("is nil")
	}

	var err error
	typ := act.GetType()
	// TODO(marius): this does not work correctly if act.Object is an ItemCollection
	//  First we process the activity to effect whatever changes we need to on the activity properties.
	switch {
	case vocab.ContentManagementActivityTypes.Contains(typ) && act.Object.GetType() != vocab.RelationshipType:
		act, err = ContentManagementActivityFromClient(p, act)
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

	if act.Published.IsZero() {
		act.Published = time.Now().UTC()
	}

	var it vocab.Item
	if act.Content != nil || act.Summary != nil {
		// For activities that have a content value, we create the collections that allow actors to interact
		// with them as they are a regular object.
		_ = vocab.OnObject(act, addNewObjectCollections)
	}

	recipients, err := p.BuildOutboxRecipientsList(act, receivedIn)
	if err != nil {
		return act, err
	}
	activityReplyToCollections, _ := p.BuildReplyToCollections(act)

	// Making a local copy of the activity in order to not lose information that could be required
	// later in the call system.
	toSave := *act
	it, err = p.s.Save(vocab.FlattenProperties(&toSave))
	if err != nil {
		return act, err
	}
	// Additional recommendation from the ActivityPub mailing list:
	// Activities addressed to `Public` usually appear only in the inboxes of actors that follow the activity's `actor`
	// property.
	if err = p.AddToLocalCollections(it, append(recipients, activityReplyToCollections...)...); err != nil {
		p.l.Errorf("%+s", err)
	}
	if err = p.AddToRemoteCollections(it, recipients...); err != nil {
		p.l.Errorf("%+s", err)
	}

	return act, nil
}

// BuildOutboxRecipientsList builds the recipients list of the received 'it' Activity is addressed to:
//   - the author's Outbox
//   - the recipients' Inboxes
func (p P) BuildOutboxRecipientsList(it vocab.Item, receivedIn vocab.IRI) (vocab.ItemCollection, error) {
	act, err := vocab.ToActivity(it)
	if err != nil {
		return nil, err
	}
	if vocab.IsNil(act) {
		return nil, InvalidActivity("is nil")
	}
	loader := p.s

	allRecipients := make(vocab.ItemCollection, 0)
	if !vocab.IsNil(act.Actor) && p.IsLocal(act.Actor) {
		// NOTE(marius): this is needed only for client to server interactions
		actIRI := act.Actor.GetLink()
		outbox := vocab.Outbox.IRI(actIRI)

		if !actIRI.Equals(vocab.PublicNS, true) {
			allRecipients.Append(outbox)
		}
	}

	for _, rec := range act.Recipients() {
		recIRI := rec.GetLink()
		if recIRI == vocab.PublicNS {
			// NOTE(marius): if the activity is addressed to the Public NS, we store it to the local service's inbox
			// TODO(marius): this basically needs to add the shared inbox of the Service corresponding to our server
			if len(p.baseIRI) > 0 {
				allRecipients.Append(vocab.Inbox.IRI(p.baseIRI[0]))
			}
			continue
		}
		if vocab.ValidCollectionIRI(recIRI) {
			members, err := loader.Load(recIRI)
			if err != nil || vocab.IsNil(members) {
				continue
			}
			vocab.OnCollectionIntf(members, func(col vocab.CollectionInterface) error {
				for _, m := range col.Collection() {
					if !vocab.ActorTypes.Contains(m.GetType()) || (p.IsLocalIRI(m.GetLink()) && isBlocked(loader, m, act.Actor)) {
						continue
					}
					vocab.OnActor(m, func(act *vocab.Actor) error {
						if act.Endpoints != nil && !vocab.IsNil(act.Endpoints.SharedInbox) {
							allRecipients.Append(act.Endpoints.SharedInbox.GetLink())
						} else {
							allRecipients.Append(vocab.Inbox.IRI(m))
						}
						return nil
					})
				}
				return nil
			})
		} else {
			if p.IsLocalIRI(recIRI) && isBlocked(loader, recIRI, act.Actor) {
				continue
			}
			// TODO(marius): add check if IRI represents an actor (or rely on the collection saver to break if not)
			allRecipients.Append(vocab.Inbox.IRI(recIRI))
		}
	}
	// NOTE(marius): append the receivedIn collection to the list of recipients
	// We do this, because it could be missing from the Activity's recipients fields (to, bto, cc, bcc)
	allRecipients.Append(receivedIn)

	return vocab.ItemCollectionDeduplication(&allRecipients), nil
}
