package processing

import (
	"git.sr.ht/~mariusor/lw"
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

	var err error
	if vocab.IntransitiveActivityTypes.Match(it.GetType()) {
		err = vocab.OnIntransitiveActivity(it, p.dereferenceIntransitiveActivityProperties(receivedIn))
	} else {
		err = vocab.OnActivity(it, p.dereferenceActivityProperties(receivedIn))
	}
	if err != nil {
		return it, err
	}

	if err := p.ValidateServerActivity(it, author, receivedIn); err != nil {
		return it, err
	}

	// NOTE(marius): the separation between transitive and intransitive activities overlaps the separation we're
	// using in the processingClientActivity function between the ActivityStreams motivations separation.
	// This means that 'it' should probably be treated as a vocab.Item until the last possible moment.
	if vocab.IntransitiveActivityTypes.Match(it.GetType()) {
		err = vocab.OnIntransitiveActivity(it, func(act *vocab.IntransitiveActivity) error {
			if err := p.saveRemoteIntransitiveActivity(act); err != nil {
				p.l.WithContext(lw.Ctx{"err": err.Error()}).Warnf("unable to save remote activity and objects locally")
			}
			it, err = processServerIntransitiveActivity(p, act, receivedIn)
			return err
		})
	} else {
		err = vocab.OnActivity(it, func(act *vocab.Activity) error {
			if err := p.saveRemoteActivity(act); err != nil {
				p.l.WithContext(lw.Ctx{"err": err.Error()}).Warnf("unable to save remote activity and objects locally")
			}
			it, err = processServerActivity(p, act, receivedIn)
			return err
		})
	}
	if err != nil {
		return it, err
	}

	firstDelivery := true
	if existing, _ := p.s.Load(it.GetLink()); !vocab.IsNil(existing) {
		firstDelivery = false
	}

	if it, err = p.s.Save(vocab.FlattenProperties(it)); err != nil {
		return it, err
	}

	return it, p.ProcessServerInboxDelivery(it, receivedIn, firstDelivery)
}

// ProcessServerInboxDelivery processes an incoming activity received in an actor's Inbox collection.
// It propagates the activity to all local actors, and if among them there are collections, they get
// dereferenced and their members local *and* remote get forwarded a copy of the activity.
func (p P) ProcessServerInboxDelivery(it vocab.Item, receivedIn vocab.IRI, firstDelivery bool) error {
	recipients := make(vocab.ItemCollection, 0)
	_ = recipients.Append(p.BuildInboxRecipientsList(it, receivedIn)...)

	fromLocalCollections := make(vocab.ItemCollection, 0)
	_ = fromLocalCollections.Append(p.BuildLocalCollectionsRecipients(it, receivedIn)...)

	toForward := make(vocab.ItemCollection, 0, len(recipients))
	for _, lr := range fromLocalCollections {
		if p.IsLocal(lr.GetLink()) {
			_ = recipients.Append(lr.GetLink())
		} else {
			_ = toForward.Append(lr.GetLink())
		}
	}

	sync := func() {
		if err := p.AddToLocalCollections(it, recipients...); err != nil {
			p.l.Warnf("errors when disseminating to local actors: %s", err)
		}

		if err := p.ForwardFromInbox(it, toForward, firstDelivery); err != nil {
			p.l.Warnf("errors when forwarding to remote actors: %s", err)
		}
	}

	if p.async {
		// TODO(marius): Find another mechanism for running this asynchronously.
		go sync()
	} else {
		sync()
	}
	return nil
}

// ForwardFromInbox
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
// * The values of to, cc, and/or audience contain a Collection owned by the server: this is done in the
// ProcessServerInboxDelivery where we fetch the recipients corresponding to local collections
// * The values of inReplyTo, object, target and/or tag are objects owned by the server: this is verified in ObjectShouldBeInboxForwarded
// * The server SHOULD recurse through these values to look for linked objects owned by the server, and SHOULD set a
// maximum limit for recursion (ie. the point at which the thread is so deep the recipients followers may not mind if
// they are no longer getting updates that don't directly involve the recipient). The server MUST only target the values
// of to, cc, and/or audience on the original object being forwarded, and not pick up any new addressees whilst
// recursing through the linked objects (in case these addressees were purposefully amended by or via the client).
//
// The server MAY filter its delivery targets according to implementation-specific rules (for example, spam filtering).
func (p P) ForwardFromInbox(it vocab.Item, remoteRecipients vocab.ItemCollection, firstDelivery bool) error {
	if len(remoteRecipients) == 0 {
		return nil
	}

	if !firstDelivery {
		return nil
	}
	if !p.ObjectShouldBeInboxForwarded(it, 3) {
		return nil
	}
	return p.disseminateToRemoteCollections(it, remoteRecipients.IRIs()...)
}

// ObjectShouldBeInboxForwarded checks if the last remaining rules for forwarding from an inbox are fulfilled.
//
// * The values of inReplyTo, object, target and/or tag are objects owned by the server.
// * The server SHOULD recurse through these values to look for linked objects owned by the server, and SHOULD set a
// maximum limit for recursion (ie. the point at which the thread is so deep the recipients followers may not mind if
// they are no longer getting updates that don't directly involve the recipient). The server MUST only target the values
// of to, cc, and/or audience on the original object being forwarded, and not pick up any new addressees whilst
// recursing through the linked objects (in case these addressees were purposefully amended by or via the client).
//
// The server MAY filter its delivery targets according to implementation-specific rules (for example, spam filtering).
func (p P) ObjectShouldBeInboxForwarded(it vocab.Item, maxDepth int) bool {
	if vocab.IsNil(it) {
		return false
	}
	if maxDepth <= 0 {
		return false
	}

	shouldForward := false

	typ := it.GetType()
	switch {
	case vocab.IsIRI(it):
		return p.IsLocal(it)
	case vocab.IsIRIs(it):
		_ = vocab.OnIRIs(it, func(is *vocab.IRIs) error {
			for _, iri := range *is {
				if shouldForward = p.IsLocalIRI(iri); shouldForward {
					break
				}
			}
			return nil
		})
	case vocab.IsItemCollection(it):
		_ = vocab.OnCollectionIntf(it, func(col vocab.CollectionInterface) error {
			for _, ob := range col.Collection() {
				if shouldForward = p.ObjectShouldBeInboxForwarded(ob, maxDepth-1); shouldForward {
					break
				}
			}
			return nil
		})
	case vocab.IntransitiveActivityTypes.Match(typ):
		_ = vocab.OnIntransitiveActivity(it, func(act *vocab.IntransitiveActivity) error {
			if shouldForward = p.IsLocal(act); shouldForward {
				return nil
			}
			if shouldForward = p.ObjectShouldBeInboxForwarded(act.Target, maxDepth-1); shouldForward {
				return nil
			}
			return nil
		})
	case vocab.ActivityTypes.Match(typ):
		_ = vocab.OnActivity(it, func(act *vocab.Activity) error {
			if shouldForward = p.IsLocal(act); shouldForward {
				return nil
			}
			if shouldForward = p.ObjectShouldBeInboxForwarded(act.Object, maxDepth-1); shouldForward {
				return nil
			}
			if shouldForward = p.ObjectShouldBeInboxForwarded(act.Target, maxDepth-1); shouldForward {
				return nil
			}
			return nil
		})
	default:
		_ = vocab.OnObject(it, func(ob *vocab.Object) error {
			if shouldForward = p.IsLocal(ob); shouldForward {
				return nil
			}
			if shouldForward = p.ObjectShouldBeInboxForwarded(ob.InReplyTo, maxDepth-1); shouldForward {
				return nil
			}
			if shouldForward = p.ObjectShouldBeInboxForwarded(ob.Tag, maxDepth-1); shouldForward {
				return nil
			}
			return nil
		})
	}

	return shouldForward
}

func (p P) localSaveIfMissing(it vocab.Item) error {
	_, err := p.s.Load(it.GetLink())
	if err == nil {
		return nil
	}
	if !errors.IsNotFound(err) {
		return err
	}
	_, err = p.s.Save(it)
	return err
}

func (p P) saveRemoteIntransitiveActivity(act *vocab.IntransitiveActivity) error {
	if !vocab.IsNil(act.Actor) && !p.IsLocalIRI(act.Actor.GetLink()) {
		if err := p.localSaveIfMissing(act.Actor); err != nil {
			return err
		}
	}
	if !vocab.IsNil(act.Target) && !p.IsLocalIRI(act.Target.GetLink()) {
		if err := p.localSaveIfMissing(act.Target); err != nil {
			return err
		}
	}
	return nil
}

func (p P) saveRemoteActivity(act *vocab.Activity) error {
	if !vocab.IsNil(act.Object) && !p.IsLocalIRI(act.Object.GetLink()) {
		if err := p.localSaveIfMissing(act.Object); err != nil {
			return err
		}
	}
	return vocab.OnIntransitiveActivity(act, func(act *vocab.IntransitiveActivity) error {
		return p.saveRemoteIntransitiveActivity(act)
	})
}

func processServerIntransitiveActivity(p P, it vocab.Item, receivedIn vocab.IRI) (vocab.Item, error) {
	return it, errors.NotImplementedf("processing intransitive activities is not yet finished")
}

func processServerActivity(p P, act *vocab.Activity, receivedIn vocab.IRI) (vocab.Item, error) {
	var err error
	typ := act.GetType()
	switch {
	case vocab.CreateType.Match(typ):
		act, err = CreateActivityFromServer(p, act)
	case vocab.DeleteType.Match(typ):
		act, err = DeleteActivity(p.s, act)
	case vocab.ReactionsActivityTypes.Match(typ):
		act, err = ReactionsActivity(p, act, receivedIn)
	}
	return act, err
}

// BuildInboxRecipientsList builds the recipients list of the received 'it' Activity is addressed to:
//   - the *local* recipients' Inboxes
func (p P) BuildInboxRecipientsList(it vocab.Item, receivedIn vocab.IRI) vocab.ItemCollection {
	act, err := vocab.ToActivity(it)
	if err != nil {
		return nil
	}
	if vocab.IsNil(act) {
		return nil
	}

	loader := p.s

	allRecipients := make(vocab.ItemCollection, 0)
	for _, rec := range act.Recipients() {
		recIRI := rec.GetLink()

		if !p.IsLocalIRI(recIRI) || isBlocked(loader, recIRI, act.Actor) {
			continue
		}

		lr, err := loader.Load(recIRI)
		if err != nil {
			continue
		}

		// NOTE(marius): at this stage we only want the actor recipients
		if vocab.ActorTypes.Match(lr.GetType()) {
			_ = allRecipients.Append(vocab.Inbox.IRI(lr))
		}
	}

	// NOTE(marius): append the receivedIn collection to the list of recipients
	// We do this, because it could be missing from the Activity's recipients fields (to, bto, cc, bcc)
	_ = allRecipients.Append(receivedIn)

	// NOTE(marius): for local dissemination, we need to check if "receivedIn" corresponds to a sharedInbox
	// that is used by actors on the current server.
	// So we load all actors that use 'receivedIn' as a sharedInbox, and append them to the recipients list.
	//
	// This logic might not be entirely sound, as I suspect we should search all local inbox recipients if
	// they are shared collections and dispatch them accordingly.
	// TODO(marius): maybe a better solution would be to have the processor map the shared inboxes and watch for
	//  new activity in them and dispatch those asynchronously.
	for _, rec := range loadSharedInboxRecipients(p, receivedIn) {
		if !allRecipients.Contains(rec.GetLink()) {
			continue
		}
		_ = allRecipients.Append(receivedIn)
	}

	return vocab.ItemCollectionDeduplication(&allRecipients)
}

// BuildLocalCollectionsRecipients builds the recipients list of the received 'it' Activity is addressed to:
//   - any *local* collections
func (p P) BuildLocalCollectionsRecipients(it vocab.Item, receivedIn vocab.IRI) vocab.ItemCollection {
	act, err := vocab.ToActivity(it)
	if err != nil {
		return nil
	}
	if vocab.IsNil(act) {
		return nil
	}

	loader := p.s

	allRecipients := make(vocab.ItemCollection, 0)
	for _, rec := range act.Recipients() {
		recIRI := rec.GetLink()
		if !p.IsLocalIRI(recIRI) || isBlocked(loader, recIRI, act.Actor) {
			continue
		}

		lr, err := loader.Load(recIRI)
		if err != nil {
			continue
		}

		if !vocab.CollectionTypes.Match(lr.GetType()) {
			continue
		}
		_ = vocab.OnCollectionIntf(lr, func(col vocab.CollectionInterface) error {
			for _, m := range col.Collection() {
				// NOTE(marius): we append all valid recipients, local or remote.
				if !vocab.ActorTypes.Match(m.GetType()) || isBlocked(loader, recIRI, act.Actor) {
					continue
				}
				_ = vocab.OnActor(m, func(act *vocab.Actor) error {
					if act.Endpoints != nil && !vocab.IsNil(act.Endpoints.SharedInbox) {
						_ = allRecipients.Append(act.Endpoints.SharedInbox.GetLink())
					} else {
						_ = allRecipients.Append(vocab.Inbox.Of(m))
					}
					return nil
				})
			}
			return nil
		})
	}

	return vocab.ItemCollectionDeduplication(&allRecipients)
}
