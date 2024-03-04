package processing

import (
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

type S2SProcessor interface {
	ProcessServerActivity(vocab.Item, vocab.IRI) (vocab.Item, error)
}

// ProcessServerActivity processes an Activity received in a server to server request
//
// https://www.w3.org/TR/activitypub/#server-to-server-interactions
//
// Servers communicate with other servers and propagate information across the social graph by posting activities to
// actors' inbox endpoints. An Activity sent over the network SHOULD have an id, unless it is intended to be transient
// (in which case it MAY omit the id).
//
// POST requests (eg. to the inbox) MUST be made with a Content-Type of
// 'application/ld+json; profile="https://www.w3.org/ns/activitystreams"' and GET requests (see also 3.2 Retrieving objects)
// with an Accept header of 'application/ld+json; profile="https://www.w3.org/ns/activitystreams"'.
// Servers SHOULD interpret a Content-Type or Accept header of 'application/activity+json' as equivalent to
// application/ld+json; profile="https://www.w3.org/ns/activitystreams" for server-to-server interactions.
//
// In order to propagate updates throughout the social graph, Activities are sent to the appropriate recipients.
// First, these recipients are determined through following the appropriate links between objects until you reach an
// actor, and then the Activity is inserted into the actor's inbox (delivery). This allows recipient servers to:
//
// conduct any side effects related to the Activity (for example, notification that an actor has liked an object is
// used to update the object's like count);
// deliver the Activity to recipients of the original object, to ensure updates are propagated to the whole social graph
// (see inbox delivery).
//
// Delivery is usually triggered by one of:
//
// * an Activity being created in an actor's outbox with their Followers Collection as the recipient.
// * an Activity being created in an actor's outbox with directly addressed recipients.
// * an Activity being created in an actor's outbox with user-curated collections as recipients.
// * an Activity being created in an actor's outbox or inbox which references another object.
//
// Servers performing delivery to the inbox or sharedInbox properties of actors on other servers MUST provide the
// object property in the activity: Create, Update, Delete, Follow, Add, Remove, Like, Block, Undo.
// Additionally, servers performing server to server delivery of the following activities MUST also provide the target
// property: Add, Remove.
//
// HTTP caching mechanisms [RFC7234] SHOULD be respected when appropriate, both when receiving responses from other
// servers and when sending responses to other servers.
func (p P) ProcessServerActivity(it vocab.Item, author vocab.Actor, receivedIn vocab.IRI) (vocab.Item, error) {
	if vocab.IsNil(it) {
		return nil, errors.Newf("Unable to process nil Activity")
	}

	err := vocab.OnActivity(it, p.dereferenceActivityProperties(receivedIn))
	if err != nil {
		return it, err
	}
	if err := p.ValidateServerActivity(it, author, receivedIn); err != nil {
		return it, err
	}
	if err := saveRemoteActivityAndObjects(p.s, it); err != nil {
		return it, err
	}

	// NOTE(marius): the separation between transitive and intransitive activities overlaps the separation we're
	// using in the processingClientActivity function between the ActivityStreams motivations separation.
	// This means that 'it' should probably be treated as a vocab.Item until the last possible moment.
	if vocab.IntransitiveActivityTypes.Contains(it.GetType()) {
		if it, err = processServerIntransitiveActivity(p, it, receivedIn); err != nil {
			return it, err
		}
	}
	err = vocab.OnActivity(it, func(act *vocab.Activity) error {
		var err error
		it, err = processServerActivity(p, act, receivedIn)
		return err
	})
	if err != nil {
		return it, err
	}

	if it, err = p.s.Save(vocab.FlattenProperties(it)); err != nil {
		return it, err
	}
	return it, p.ProcessServerInboxDelivery(it, receivedIn)
}

// ProcessServerInboxDelivery processes an incoming activity received in an actor's Inbox collection
//
// # Forwarding from Inbox
//
// https://www.w3.org/TR/activitypub/#inbox-forwarding
//
// NOTE: Forwarding to avoid the ghost replies problem
// The following section is to mitigate the "ghost replies" problem which occasionally causes problems on federated
// networks. This problem is best demonstrated with an example.
//
// Alyssa makes a post about her having successfully presented a paper at a conference and sends it to her followers
// collection, which includes her friend Ben. Ben replies to Alyssa's message congratulating her and includes her
// followers collection on the recipients. However, Ben has no access to see the members of Alyssa's followers
// collection, so his server does not forward his messages to their inbox. Without the following mechanism, if Alyssa
// were then to reply to Ben, her followers would see Alyssa replying to Ben without having ever seen Ben interacting.
// This would be very confusing!
//
// When Activities are received in the inbox, the server needs to forward these to recipients that the origin was
// unable to deliver them to. To do this, the server MUST target and deliver to the values of to, cc, and/or audience
// if and only if all of the following are true:
//
// * This is the first time the server has seen this Activity.
// * The values of to, cc, and/or audience contain a Collection owned by the server.
// * The values of inReplyTo, object, target and/or tag are objects owned by the server.
// * The server SHOULD recurse through these values to look for linked objects owned by the server, and SHOULD set a
// maximum limit for recursion (ie. the point at which the thread is so deep the recipients followers may not mind if
// they are no longer getting updates that don't directly involve the recipient). The server MUST only target the values
// of to, cc, and/or audience on the original object being forwarded, and not pick up any new addressees whilst
// recursing through the linked objects (in case these addressees were purposefully amended by or via the client).
//
// The server MAY filter its delivery targets according to implementation-specific rules (for example, spam filtering).
func (p P) ProcessServerInboxDelivery(it vocab.Item, receivedIn vocab.IRI) error {
	recipients, err := p.BuildInboxRecipientsList(it, receivedIn)
	if err != nil {
		return err
	}
	activityReplyToCollections, err := p.BuildReplyToCollections(it)
	if err != nil {
		errFn("unable to load inReplyTo collections for the activity: %s", err)
	}
	recipients = append(recipients, activityReplyToCollections...)
	return p.AddToLocalCollections(it, recipients...)
}

func saveRemoteActivityAndObjects(s WriteStore, act vocab.Item) error {
	err := vocab.OnActivity(act, func(act *vocab.Activity) error {
		if _, err := s.Save(act.Object); err != nil {
			return err
		}
		if _, err := s.Save(act.Actor); err != nil {
			return err
		}
		return nil
	})
	return err
}

func processServerIntransitiveActivity(p P, it vocab.Item, receivedIn vocab.IRI) (vocab.Item, error) {
	return it, errors.NotImplementedf("processing intransitive activities is not yet finished")
}

func processServerActivity(p P, act *vocab.Activity, receivedIn vocab.IRI) (vocab.Item, error) {
	var err error
	typ := act.GetType()
	switch {
	case vocab.CreateType == typ:
		act, err = CreateActivityFromServer(p, act)
	case vocab.DeleteType == typ:
		act, err = DeleteActivity(p.s, act)
	case vocab.ReactionsActivityTypes.Contains(typ):
		act, err = ReactionsActivity(p, act, receivedIn)
	}
	return act, err
}

// BuildInboxRecipientsList builds the recipients list of the received 'it' Activity is addressed to:
//   - the author's Outbox
//   - the recipients' Inboxes
func (p P) BuildInboxRecipientsList(it vocab.Item, receivedIn vocab.IRI) (vocab.ItemCollection, error) {
	allRecipients, err := p.BuildOutboxRecipientsList(it, receivedIn)
	if err != nil {
		return allRecipients, err
	}

	for _, rec := range loadSharedInboxRecipients(p, receivedIn) {
		// NOTE(marius): load all actors that use 'receivedIn' as a sharedInbox
		if allRecipients.Contains(rec.GetLink()) {
			continue
		}
		allRecipients.Append(receivedIn)
	}
	return vocab.ItemCollectionDeduplication(&allRecipients), nil
}
