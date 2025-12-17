package processing

import (
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// ReactionsActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-respond
//
// The Reactions use case primarily deals with reactions to content.
// This can include activities such as liking or disliking content, ignoring updates,
// flagging content as being inappropriate, accepting or rejecting objects, etc.
func ReactionsActivity(p P, act *vocab.Activity, receivedIn vocab.IRI) (*vocab.Activity, error) {
	var err error
	if act.Object != nil {
		switch act.Type {
		case vocab.DislikeType:
			fallthrough
		case vocab.LikeType:
			act, err = AppreciationActivity(p, act)
		case vocab.RejectType:
			fallthrough
		case vocab.TentativeRejectType:
			// I think nothing happens here.
			act, err = RejectActivity(p.s, act)
		case vocab.TentativeAcceptType:
			fallthrough
		case vocab.AcceptType:
			act, err = AcceptActivity(p, act, receivedIn)
		case vocab.BlockType:
			act, err = BlockActivity(p, act, receivedIn)
		case vocab.FlagType:
			act, err = FlagActivity(p.s, act)
		case vocab.IgnoreType:
			act, err = IgnoreActivity(p, act)
		}
	}

	return act, err
}

// AppreciationActivity
// The Like(and Dislike) activity indicates the actor likes the object.
// The side effect of receiving this in an outbox is that the server SHOULD add the object to the actor's liked Collection.
func AppreciationActivity(p P, act *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(act.Object) {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if vocab.IsNil(act.Actor) {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	good := vocab.ActivityVocabularyTypes{vocab.LikeType, vocab.DislikeType}
	if !good.Contains(act.Type) {
		return act, errors.NotValidf("Activity has wrong type %s, expected %v", act.Type, good)
	}

	saveToCollections := func(actors, objects vocab.ItemCollection) error {
		errs := make([]error, 0)
		colToAdd := make(map[vocab.IRI][]vocab.IRI)
		for _, object := range objects {
			for _, actor := range actors {
				liked := vocab.Liked.IRI(actor)
				colToAdd[liked] = append(colToAdd[liked], object.GetLink())
			}
			likes := vocab.Likes.IRI(object)
			colToAdd[likes] = append(colToAdd[likes], act.GetLink())
		}
		for col, iris := range colToAdd {
			for _, iri := range iris {
				if err := p.AddItemToCollection(col, iri); err != nil {
					errs = append(errs, errors.Annotatef(err, "Unable to save %s to collection %s", iris, col))
				}
			}
		}
		return errors.Join(errs...)
	}
	var actors, objects vocab.ItemCollection
	if vocab.IsItemCollection(act.Actor) {
		_ = vocab.OnItemCollection(act.Actor, func(c *vocab.ItemCollection) error {
			actors = *c
			return nil
		})
	} else {
		actors = make(vocab.ItemCollection, 1)
		actors[0] = act.Actor
	}
	if vocab.IsItemCollection(act.Object) {
		_ = vocab.OnItemCollection(act.Object, func(c *vocab.ItemCollection) error {
			objects = *c
			return nil
		})
	} else {
		objects = make(vocab.ItemCollection, 1)
		objects[0] = act.Object
	}

	// NOTE(marius): we're only saving to the Liked and Likes collections for Likes in order to conform to the spec.
	if act.GetType() == vocab.LikeType {
		// TODO(marius): do something sensible with these errors, they shouldn't stop execution,
		//               but they are still good to know
		_ = saveToCollections(actors, objects)
	}
	return act, nil
}

func firstOrItem(it vocab.Item) vocab.Item {
	if vocab.IsNil(it) {
		return it
	}
	if vocab.IsItemCollection(it) {
		_ = vocab.OnItemCollection(it, func(col *vocab.ItemCollection) error {
			it = col.First()
			return nil
		})
	}
	return it
}

// AcceptActivity
//
// In Inbox: https://www.w3.org/TR/activitypub/#follow-activity-inbox
//
// The side effect of receiving this in an inbox is determined by the type of the object received, and it is possible
// to accept types not described in this document (for example, an Offer).
//
// If the object of an Accept received to an inbox is a Follow activity previously sent by the receiver, the server
// SHOULD add the actor to the receiver's Following Collection.
func AcceptActivity(p P, act *vocab.Activity, receivedIn vocab.IRI) (*vocab.Activity, error) {
	if vocab.IsNil(act.Object) {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if vocab.IsNil(act.Actor) {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}

	if act.Object.IsLink() {
		// dereference object activity
		if actLoader, ok := p.s.(ReadStore); ok {
			obj, err := actLoader.Load(act.Object.GetLink())
			if err != nil {
				return act, errors.NotValidf("Unable to dereference object: %s", act.Object.GetLink())
			}
			act.Object = firstOrItem(obj)
		}
	}
	err := vocab.OnActivity(act.Object, func(follow *vocab.Activity) error {
		err := dispatchFollowSideEffectToLocalCollections(p, follow)
		if err != nil {
			return err
		}
		// NOTE(marius): Accepts need to be propagated back to the originating actor if missing from recipients list
		if act.Type == vocab.AcceptType {
			actor := follow.Actor
			if !actor.GetLink().Equal(vocab.PublicNS) && !p.IsLocal(actor) && !act.Recipients().Contains(actor) {
				act.BCC.Append(actor)
			}
		}
		return nil
	})
	return act, err
}

func dispatchFollowSideEffectToLocalCollections(p P, a *vocab.Activity) error {
	good := vocab.ActivityVocabularyTypes{vocab.FollowType}
	if !good.Contains(a.Type) {
		return errors.NotValidf("Object Activity has wrong type %s, expected %v", a.Type, good)
	}

	errs := make([]error, 0, 2)
	if err := p.AddToLocalCollections(a.Actor, vocab.Followers.IRI(a.Object)); err != nil {
		errs = append(errs, err)
	}

	if err := p.AddToLocalCollections(a.Object, vocab.Following.IRI(a.Actor)); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func RejectActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(act.Object) {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if vocab.IsNil(act.Actor) {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}

	if colSaver, ok := l.(CollectionStore); ok {
		inbox := vocab.Inbox.IRI(act.Actor)
		err := colSaver.RemoveFrom(inbox, act.Object.GetLink())
		return act, err
	}
	return act, nil
}

const BlockedCollection = vocab.CollectionPath("blocked")

// BlockActivity
//
// https://www.w3.org/TR/activitypub/#block-activity-outbox
//
// The Block activity is used to indicate that the posting actor does not want another actor
// (defined in the object property) to be able to interact with objects posted by the actor posting the Block activity.
// The server SHOULD prevent the blocked user from interacting with any object posted by the actor.
//
// Servers SHOULD NOT deliver Block Activities to their object.
func BlockActivity(p P, act *vocab.Activity, receivedIn vocab.IRI) (*vocab.Activity, error) {
	if vocab.IsNil(act.Object) {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if vocab.IsNil(act.Actor) {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	if act.Type != vocab.BlockType {
		return act, errors.NotValidf("Activity has wrong type %s, expected %s", act.Type, vocab.BlockType)
	}

	obIRI := act.Object.GetLink()
	// Remove object from any recipients collections
	act.To.Remove(obIRI)
	act.CC.Remove(obIRI)
	act.Bto.Remove(obIRI)
	act.BCC.Remove(obIRI)

	return act, p.AddItemToCollection(BlockedCollection.IRI(act.Actor), obIRI)
}

// FlagActivity
// There isn't any side effect to this activity except delivering it to the inboxes of its recipients.
// From the list of recipients we remove the Object itself if it represents an Actor being flagged,
// or its author if it's another type of object.
func FlagActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(act.Object) {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if vocab.IsNil(act.Actor) {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	if act.Type != vocab.FlagType {
		return act, errors.NotValidf("Activity has wrong type %s, expected %s", act.Type, vocab.FlagType)
	}

	vocab.OnObject(act.Object, func(o *vocab.Object) error {
		var toRemoveIRI vocab.IRI
		if !vocab.ActorTypes.Contains(o.Type) {
			if o.AttributedTo != nil {
				// Remove object's author from any recipients collections
				toRemoveIRI = o.AttributedTo.GetLink()
			}
		} else {
			// Remove object from any recipients collections
			toRemoveIRI = o.GetLink()
		}
		if toRemoveIRI.IsValid() {
			act.To.Remove(toRemoveIRI)
			act.CC.Remove(toRemoveIRI)
			act.Bto.Remove(toRemoveIRI)
			act.BCC.Remove(toRemoveIRI)
		}
		return nil
	})

	return act, nil
}

const IgnoredCollection = vocab.CollectionPath("ignored")

// IgnoreActivity
// This relies on custom behavior for the repository, which would allow for an ignored collection,
// where we save these
func IgnoreActivity(p P, act *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(act.Object) {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if vocab.IsNil(act.Actor) {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	if act.Type != vocab.IgnoreType {
		return act, errors.NotValidf("Activity has wrong type %s, expected %s", act.Type, vocab.IgnoreType)
	}

	obIRI := act.Object.GetLink()
	// Remove object from any recipients collections
	act.To.Remove(obIRI)
	act.CC.Remove(obIRI)
	act.Bto.Remove(obIRI)
	act.BCC.Remove(obIRI)

	return act, p.AddItemToCollection(IgnoredCollection.IRI(act.Actor), obIRI)
}
