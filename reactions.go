package processing

import (
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
	"strings"
)

// ReactionsActivity processes matching activities
// The Reactions use case primarily deals with reactions to content.
// This can include activities such as liking or disliking content, ignoring updates,
// flagging content as being inappropriate, accepting or rejecting objects, etc.
func ReactionsActivity(l s.WriteStore, act *pub.Activity) (*pub.Activity, error) {
	var err error
	if act.Object != nil {
		switch act.Type {
		case pub.DislikeType:
			fallthrough
		case pub.LikeType:
			act, err = AppreciationActivity(l, act)
		case pub.RejectType:
			fallthrough
		case pub.TentativeRejectType:
			// I think nothing happens here.
			act, err = RejectActivity(l, act)
		case pub.TentativeAcceptType:
			fallthrough
		case pub.AcceptType:
			act, err = AcceptActivity(l, act)
		case pub.BlockType:
			act, err = BlockActivity(l, act)
		case pub.FlagType:
			act, err = FlagActivity(l, act)
		case pub.IgnoreType:
			act, err = IgnoreActivity(l, act)
		}
	}

	return act, err
}

type multi struct {
	errors []error
}

func (m multi) Error() string {
	b := strings.Builder{}
	for _, err := range m.errors {
		b.WriteString(err.Error())
	}
	return b.String()
}

// AppreciationActivity
// The Like(and Dislike) activity indicates the actor likes the object.
// The side effect of receiving this in an outbox is that the server SHOULD add the object to the actor's liked Collection.
func AppreciationActivity(l s.WriteStore, act *pub.Activity) (*pub.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	good := pub.ActivityVocabularyTypes{pub.LikeType, pub.DislikeType}
	if !good.Contains(act.Type) {
		return act, errors.NotValidf("Activity has wrong type %s, expected %v", act.Type, good)
	}

	colSaver, ok := l.(s.CollectionStore)
	if !ok {
		return act, nil
	}

	saveToCollections := func(colSaver s.CollectionStore, actors, objects pub.ItemCollection) error {
		colErrors := multi{}
		colToAdd := make(map[pub.IRI][]pub.IRI)
		for _, object := range objects {
			for _, actor := range actors {
				liked := handlers.Liked.IRI(actor)
				colToAdd[liked] = append(colToAdd[liked], object.GetLink())
			}
			likes := handlers.Likes.IRI(object)
			colToAdd[likes] = append(colToAdd[likes], act.GetLink())
		}
		for col, iris := range colToAdd {
			for _, iri := range iris {
				if err := colSaver.AddTo(col, iri); err != nil {
					colErrors.errors = append(colErrors.errors, errors.Annotatef(err, "Unable to save %s to collection %s", iris, col))
				}
			}
		}
		if len(colErrors.errors) > 0 {
			return colErrors
		}
		return nil
	}
	var actors, objects pub.ItemCollection
	if pub.IsItemCollection(act.Actor) {
		pub.OnItemCollection(act.Actor, func(c *pub.ItemCollection) error {
			actors = *c
			return nil
		})
	} else {
		actors = make(pub.ItemCollection, 1)
		actors[0] = act.Actor
	}
	if pub.IsItemCollection(act.Object) {
		pub.OnItemCollection(act.Object, func(c *pub.ItemCollection) error {
			objects = *c
			return nil
		})
	} else {
		objects = make(pub.ItemCollection, 1)
		objects[0] = act.Object
	}

	// NOTE(marius): we're only saving to the Liked and Likes collections for Likes in order to conform to the spec.
	if act.GetType() == pub.LikeType {
		// TODO(marius): do something sensible with these errors, they shouldn't stop execution,
		//               but they are still good to know
		_ = saveToCollections(colSaver, actors, objects)
	}
	return act, nil
}

func firstOrItem(it pub.Item) pub.Item {
	if pub.IsNil(it) {
		return it
	}
	if it.IsCollection() {
		pub.OnCollectionIntf(it, func(col pub.CollectionInterface) error {
			it = col.Collection().First()
			return nil
		})
	}
	return it
}

// AcceptActivity
// The side effect of receiving this in an inbox is that the server SHOULD add the object to the actor's followers Collection.
func AcceptActivity(l s.WriteStore, act *pub.Activity) (*pub.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	good := pub.ActivityVocabularyTypes{pub.AcceptType, pub.TentativeAcceptType}
	if !good.Contains(act.Type) {
		return act, errors.NotValidf("Activity has wrong type %s, expected %v", act.Type, good)
	}

	if act.Object.IsLink() {
		// dereference object activity
		if actLoader, ok := l.(s.ReadStore); ok {
			obj, err := actLoader.Load(act.Object.GetLink())
			if err != nil {
				return act, errors.NotValidf("Unable to dereference object: %s", act.Object.GetLink())
			}
			act.Object = firstOrItem(obj)
		}
	}
	err := pub.OnActivity(act.Object, func(a *pub.Activity) error {
		if !act.Actor.GetLink().Equals(a.Object.GetLink(), false) {
			return errors.NotValidf("The %s activity has a different actor than its object: %s, expected %s", act.Type, act.Actor.GetLink(), a.Actor.GetLink())
		}
		good := pub.ActivityVocabularyTypes{pub.FollowType}
		if !good.Contains(a.Type) {
			return errors.NotValidf("Object Activity has wrong type %s, expected %v", a.Type, good)
		}
		followers := handlers.Followers.IRI(act.Actor)
		following := handlers.Following.IRI(a.Actor)
		if colSaver, ok := l.(s.CollectionStore); ok {
			if err := colSaver.AddTo(following, a.Object.GetLink()); err != nil {
				return err
			}
			if err := colSaver.AddTo(followers, a.Actor.GetLink()); err != nil {
				return err
			}
		}
		return nil
	})
	return act, err
}

func RejectActivity(l s.WriteStore, act *pub.Activity) (*pub.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	good := pub.ActivityVocabularyTypes{pub.RejectType, pub.TentativeRejectType}
	if !good.Contains(act.Type) {
		return act, errors.NotValidf("Activity has wrong type %s, expected %v", act.Type, good)
	}

	if colSaver, ok := l.(s.CollectionStore); ok {
		inbox := handlers.Inbox.IRI(act.Actor)
		err := colSaver.RemoveFrom(inbox, act.Object.GetLink())
		if err != nil {
			return act, err
		}
	}
	return act, nil
}

const BlockedCollection = handlers.CollectionType("blocked")

// BlockActivity
// The side effect of receiving this in an outbox is that the server SHOULD add the object to the actor's blocked Collection.
func BlockActivity(l s.WriteStore, act *pub.Activity) (*pub.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	if act.Type != pub.BlockType {
		return act, errors.NotValidf("Activity has wrong type %s, expected %s", act.Type, pub.BlockType)
	}

	obIRI := act.Object.GetLink()
	// Remove object from any recipients collections
	act.To.Remove(obIRI)
	act.CC.Remove(obIRI)
	act.Bto.Remove(obIRI)
	act.BCC.Remove(obIRI)

	if colSaver, ok := l.(s.CollectionStore); ok {
		err := colSaver.AddTo(BlockedCollection.IRI(act.Actor), obIRI)
		if err != nil {
			return act, err
		}
	}
	return act, nil
}

// FlagActivity
// There isn't any side effect to this activity except delivering it to the inboxes of its recipients.
// From the list of recipients we remove the Object itself if it represents an Actor being flagged,
// or its author if it's another type of object.
func FlagActivity(l s.WriteStore, act *pub.Activity) (*pub.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	if act.Type != pub.FlagType {
		return act, errors.NotValidf("Activity has wrong type %s, expected %s", act.Type, pub.FlagType)
	}

	pub.OnObject(act.Object, func(o *pub.Object) error {
		var toRemoveIRI pub.IRI
		if !pub.ActorTypes.Contains(o.Type) {
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

const IgnoredCollection = handlers.CollectionType("ignored")

// IgnoreActivity
// This relies on custom behavior for the repository, which would allow for an ignored collection,
// where we save these
func IgnoreActivity(l s.WriteStore, act *pub.Activity) (*pub.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	if act.Type != pub.IgnoreType {
		return act, errors.NotValidf("Activity has wrong type %s, expected %s", act.Type, pub.IgnoreType)
	}

	obIRI := act.Object.GetLink()
	// Remove object from any recipients collections
	act.To.Remove(obIRI)
	act.CC.Remove(obIRI)
	act.Bto.Remove(obIRI)
	act.BCC.Remove(obIRI)

	if colSaver, ok := l.(s.CollectionStore); ok {
		err := colSaver.AddTo(IgnoredCollection.IRI(act.Actor), obIRI)
		if err != nil {
			return act, err
		}
	}
	return act, nil
}
