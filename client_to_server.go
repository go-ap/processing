package processing

import (
	"time"

	"git.sr.ht/~mariusor/lw"
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
// servers and servers sending responses to clients.
func (p *P) ProcessClientActivity(it vocab.Item, author vocab.Actor, receivedIn vocab.IRI) (vocab.Item, error) {
	if vocab.IsNil(it) {
		return nil, InvalidActivity("is nil")
	}

	if err := p.ValidateClientActivity(it, author, receivedIn); err != nil {
		return it, err
	}
	// NOTE(marius): the separation between transitive and intransitive activities overlaps the separation we're
	//  using in the processingClientActivity function between the ActivityStreams use case motivation separation.
	//  https://www.w3.org/TR/activitystreams-vocabulary/
	//  This means that 'it' should probably be treated as a vocab.Item until the last possible moment.
	switch {
	case vocab.IntransitiveActivityTypes.Match(it.GetType()):
		return p.processClientIntransitiveActivity(it, receivedIn)
	default:
		return it, vocab.OnActivity(it, func(act *vocab.Activity) error {
			var err error
			it, err = p.processClientActivity(act, receivedIn)
			return err
		})
	}
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
func (p *P) ProcessOutboxDelivery(it vocab.Item, receivedIn vocab.IRI) error {
	recipients := p.BuildOutboxRecipientsList(it, receivedIn)
	if len(recipients) == 0 {
		return nil
	}

	// NOTE(marius): this accumulates the InReplyTo targets for the current activity
	replyToCollections := p.BuildReplyToCollections(it)
	if err := p.AddToLocalCollections(it, append(recipients, replyToCollections...)...); err != nil {
		p.l.WithContext(lw.Ctx{"err": err}).Errorf("unable to add to local collections")
	}
	if err := p.AddToRemoteCollections(it, recipients...); err != nil {
		p.l.WithContext(lw.Ctx{"err": err}).Errorf("unable to add to remote collections")
	}

	return nil
}

func (p *P) processClientIntransitiveActivity(act vocab.Item, receivedIn vocab.IRI) (vocab.Item, error) {
	if len(act.GetLink()) == 0 {
		if err := SetIDIfMissing(act, nil, p.createIDFn); err != nil {
			return act, err
		}
	}
	typ := act.GetType()
	if vocab.QuestionActivityTypes.Match(typ) {
		err := vocab.OnQuestion(act, func(q *vocab.Question) error {
			var err error
			q, err = p.QuestionActivity(q)
			return err
		})
		if err != nil {
			return act, err
		}
	}
	err := vocab.OnIntransitiveActivity(act, func(act *vocab.IntransitiveActivity) error {
		var err error
		if vocab.GeoSocialEventsActivityTypes.Match(typ) {
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
		return act, err
	}

	sync := func() {
		if err = p.ProcessOutboxDelivery(act, receivedIn); err != nil {
			p.l.WithContext(lw.Ctx{"err": err}).Errorf("unable to add recipients to remote collection")
		}
	}

	if _, err = p.s.Save(vocab.FlattenProperties(act)); err != nil {
		return act, err
	}

	if p.async {
		go sync()
	} else {
		sync()
	}

	return act, nil
}

func (p *P) processClientActivity(act *vocab.Activity, receivedIn vocab.IRI) (vocab.Item, error) {
	if len(act.GetLink()) == 0 {
		if err := SetIDIfMissing(act, nil, p.createIDFn); err != nil {
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
	case vocab.ContentManagementActivityTypes.Match(typ) && !vocab.RelationshipType.Match(act.Object.GetType()):
		act, err = ContentManagementActivityFromClient(p, act)
	case vocab.CollectionManagementActivityTypes.Match(typ):
		act, err = p.CollectionManagementActivity(act)
	case vocab.ReactionsActivityTypes.Match(typ):
		act, err = p.ReactionsActivity(act, receivedIn)
	case vocab.EventRSVPActivityTypes.Match(typ):
		act, err = EventRSVPActivity(p.s, act)
	case vocab.GroupManagementActivityTypes.Match(typ):
		act, err = GroupManagementActivity(p.s, act)
	case vocab.ContentExperienceActivityTypes.Match(typ):
		act, err = ContentExperienceActivity(p.s, act)
	case vocab.GeoSocialEventsActivityTypes.Match(typ):
		// NOTE(marius): this is most likely wrong, as Arrive and Travel are Intransitive Activities
		act, err = GeoSocialEventsActivity(p.s, act)
	case vocab.NotificationActivityTypes.Match(typ):
		act, err = p.NotificationActivity(act)
	case vocab.RelationshipManagementActivityTypes.Match(typ):
		act, err = p.RelationshipManagementActivity(act, receivedIn)
	case vocab.NegatingActivityTypes.Match(typ):
		act, err = p.NegatingActivity(act)
	case vocab.OffersActivityTypes.Match(typ):
		act, err = OffersActivity(p.s, act)
	}
	if err != nil {
		return act, err
	}

	if act.Published.IsZero() {
		act.Published = time.Now().Round(time.Millisecond).UTC()
	}

	sync := func() {
		if err = p.ProcessOutboxDelivery(act, receivedIn); err != nil {
			p.l.WithContext(lw.Ctx{"err": err}).Errorf("unable to add recipients to remote collection")
		}
	}
	// Making a local copy of the activity in order to not lose information that could be required
	// later in the call system.
	toSave := *act
	if _, err := p.s.Save(vocab.FlattenProperties(&toSave)); err != nil {
		return act, err
	}

	if p.async {
		// TODO(marius): Find another mechanism for running this asynchronously.
		go sync()
	} else {
		sync()
	}
	return act, nil
}

// SharedInboxes returns the list of inbox collections the processing uses to disseminate
// to local actors that use it as such (actor's endpoints.sharedInbox corresponds).
func (p *P) SharedInboxes() vocab.ItemCollection {
	result := make(vocab.ItemCollection, 0, len(p.baseIRI))
	for _, iri := range validateLocalIRI(p.s, p.baseIRI...) {
		_ = result.Append(vocab.Inbox.IRI(iri))
	}
	return result
}

// BuildOutboxRecipientsList builds the recipients list of the received 'it' Activity is addressed to:
//   - the author's Outbox
//   - the recipients' Inboxes
func (p *P) BuildOutboxRecipientsList(it vocab.Item, receivedIn vocab.IRI) vocab.ItemCollection {
	act, err := vocab.ToIntransitiveActivity(it)
	if err != nil {
		return nil
	}
	if vocab.IsNil(act) {
		return nil
	}
	loader := p.s

	actor := act.Actor
	allRecipients := make(vocab.ItemCollection, 0)

	// NOTE(marius): append the "receivedIn" collection to the list of recipients
	//  We do this, because it could be missing from the Activity's recipients fields (to, bto, cc, bcc)
	_ = allRecipients.Append(receivedIn)

	_ = vocab.OnItem(actor, func(actor vocab.Item) error {
		// NOTE(marius): this is needed only for client to server interactions
		if vocab.IsNil(actor) || p.IsLocal(actor) {
			return nil
		}
		// NOTE(marius): this most likely overlaps with the logic above,
		//  of adding the receivedIn collection to the recipients list.
		if actIRI := actor.GetLink(); !vocab.PublicNS.Equal(actIRI) {
			_ = allRecipients.Append(vocab.Outbox.IRI(actIRI))
		}
		return nil
	})

	actorHasBlocked := p.actorHasBlockedFn(actor)

	if hasRecipients, ok := it.(vocab.HasRecipients); ok {
		activityRecipients := hasRecipients.Recipients()

		for _, rec := range activityRecipients {
			recIRI := rec.GetLink()

			if vocab.PublicNS.Equal(recIRI) {
				// NOTE(marius): if the activity is Public, we store it in the local shared inboxes
				//  See [disseminateToLocalCollections] for the part where we disseminate to the actors
				//  that use them.
				_ = allRecipients.Append(p.SharedInboxes()...)
				continue
			}

			if actorHasBlocked(recIRI) {
				// NOTE(marius): if the activity actor has blocked the recipient, we skip
				p.l.WithContext(lw.Ctx{"actor": actor.GetID(), "rec": recIRI}).Tracef("Skipping blocked recipient")
				continue
			}

			if !p.IsLocalIRI(recIRI) {
				_ = allRecipients.Append(vocab.Inbox.IRI(recIRI))
				continue
			}

			recipient, err := loader.Load(recIRI)
			if err != nil || vocab.IsNil(recipient) {
				continue
			}

			_ = vocab.OnItem(recipient, func(rec vocab.Item) error {
				recipientHasBlocked := p.actorHasBlockedFn(rec)
				if recipientHasBlocked(act.Actor) {
					p.l.WithContext(lw.Ctx{"actor": act.Actor.GetID(), "rec": recIRI}).Tracef("Skipping blocked actor blocked by recipient")
					return nil
				}
				if !vocab.ActorTypes.Match(rec.GetType()) {
					return nil
				}
				return vocab.OnActor(rec, func(act *vocab.Actor) error {
					if (act.Endpoints != nil && !vocab.IsNil(act.Endpoints.SharedInbox)) && !p.IsLocalIRI(act.ID) {
						_ = allRecipients.Append(act.Endpoints.SharedInbox.GetLink())
					} else {
						_ = allRecipients.Append(vocab.Inbox.Of(rec))
					}
					return nil
				})
			})
		}
	}

	return vocab.ItemCollectionDeduplication(&allRecipients)
}
