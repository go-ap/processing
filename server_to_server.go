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
func (p P) ProcessServerActivity(it vocab.Item, receivedIn vocab.IRI) (vocab.Item, error) {
	if it == nil {
		return nil, errors.Newf("Unable to process nil activity")
	}

	err := saveRemoteActivityAndObject(p.s, it)
	if err != nil {
		return it, err
	}
	recipients, err := p.BuildRecipientsList(it, receivedIn)
	if err != nil {
		return it, err
	}
	localCollections, err := p.BuildAdditionalCollections(it)
	if err != nil {
		errFn("error: %s", err)
	}
	recipients = append(recipients, localCollections...)

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
	return it, p.AddToLocalCollections(it, recipients)
}

func saveRemoteActivityAndObject(s WriteStore, act vocab.Item) error {
	err := vocab.OnActivity(act, func(act *vocab.Activity) error {
		_, err := s.Save(act.Object)
		return err
	})
	if err != nil {
		return err
	}
	_, err = s.Save(vocab.FlattenProperties(act))
	return err
}

func processServerIntransitiveActivity(p P, it vocab.Item, receivedIn vocab.IRI) (vocab.Item, error) {
	return it, errors.NotImplementedf("processing intransitive activities is not yet finished")
}

func processServerActivity(p P, act *vocab.Activity, receivedIn vocab.IRI) (vocab.Item, error) {
	var err error
	typ := act.GetType()
	switch {
	case vocab.DeleteType == typ:
		act, err = DeleteActivity(p.s, act)
	case vocab.ReactionsActivityTypes.Contains(typ):
		act, err = ReactionsActivity(p, act, receivedIn)
	}
	return act, err
}
