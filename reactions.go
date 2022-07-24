package processing

import (
	"strings"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// ReactionsActivity processes matching activities
// The Reactions use case primarily deals with reactions to content.
// This can include activities such as liking or disliking content, ignoring updates,
// flagging content as being inappropriate, accepting or rejecting objects, etc.
func ReactionsActivity(p P, act *vocab.Activity) (*vocab.Activity, error) {
	var err error
	if act.Object != nil {
		switch act.Type {
		case vocab.DislikeType:
			fallthrough
		case vocab.LikeType:
			act, err = AppreciationActivity(p.s, act)
		case vocab.RejectType:
			fallthrough
		case vocab.TentativeRejectType:
			// I think nothing happens here.
			act, err = RejectActivity(p.s, act)
		case vocab.TentativeAcceptType:
			fallthrough
		case vocab.AcceptType:
			act, err = AcceptActivity(p, act)
		case vocab.BlockType:
			act, err = BlockActivity(p.s, act)
		case vocab.FlagType:
			act, err = FlagActivity(p.s, act)
		case vocab.IgnoreType:
			act, err = IgnoreActivity(p.s, act)
		}
	}

	return act, err
}

type multi []error

func (m multi) Error() string {
	b := strings.Builder{}
	for _, err := range m {
		b.WriteString(err.Error())
	}
	return b.String()
}

func (m multi) As(e any) bool {
	if len(m) == 0 {
		return false
	}
	return errors.As(m[0], e)
}

// AppreciationActivity
// The Like(and Dislike) activity indicates the actor likes the object.
// The side effect of receiving this in an outbox is that the server SHOULD add the object to the actor's liked Collection.
func AppreciationActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	good := vocab.ActivityVocabularyTypes{vocab.LikeType, vocab.DislikeType}
	if !good.Contains(act.Type) {
		return act, errors.NotValidf("Activity has wrong type %s, expected %v", act.Type, good)
	}

	colSaver, ok := l.(CollectionStore)
	if !ok {
		return act, nil
	}

	saveToCollections := func(colSaver CollectionStore, actors, objects vocab.ItemCollection) error {
		errs := make(multi, 0)
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
				if err := colSaver.AddTo(col, iri); err != nil {
					errs = append(errs, errors.Annotatef(err, "Unable to save %s to collection %s", iris, col))
				}
			}
		}
		if len(errs) > 0 {
			return errs
		}
		return nil
	}
	var actors, objects vocab.ItemCollection
	if vocab.IsItemCollection(act.Actor) {
		vocab.OnItemCollection(act.Actor, func(c *vocab.ItemCollection) error {
			actors = *c
			return nil
		})
	} else {
		actors = make(vocab.ItemCollection, 1)
		actors[0] = act.Actor
	}
	if vocab.IsItemCollection(act.Object) {
		vocab.OnItemCollection(act.Object, func(c *vocab.ItemCollection) error {
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
		_ = saveToCollections(colSaver, actors, objects)
	}
	return act, nil
}

func firstOrItem(it vocab.Item) vocab.Item {
	if vocab.IsNil(it) {
		return it
	}
	if it.IsCollection() {
		vocab.OnCollectionIntf(it, func(col vocab.CollectionInterface) error {
			it = col.Collection().First()
			return nil
		})
	}
	return it
}

// AcceptActivity
// The side effect of receiving this in an inbox is that the server SHOULD add the object to the actor's followers Collection.
func AcceptActivity(p P, act *vocab.Activity) (*vocab.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	good := vocab.ActivityVocabularyTypes{vocab.AcceptType, vocab.TentativeAcceptType}
	if !good.Contains(act.Type) {
		return act, errors.NotValidf("Activity has wrong type %s, expected %v", act.Type, good)
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
	err := vocab.OnActivity(act.Object, func(a *vocab.Activity) error {
		if !act.Actor.GetLink().Equals(a.Object.GetLink(), false) {
			return errors.NotValidf("The %s activity has a different actor than its object: %s, expected %s", act.Type, act.Actor.GetLink(), a.Actor.GetLink())
		}
		return finalizeFollowActivity(p, a)
	})
	return act, err
}

func finalizeFollowActivity(p P, a *vocab.Activity) error {
	good := vocab.ActivityVocabularyTypes{vocab.FollowType}
	if !good.Contains(a.Type) {
		return errors.NotValidf("Object Activity has wrong type %s, expected %v", a.Type, good)
	}

	errs := make(multi, 0)
	if err := p.AddItemToCollection(vocab.Followers.IRI(a.Object), a.Actor); err != nil {
		errs = append(errs, err)
	}
	if err := p.AddItemToCollection(vocab.Following.IRI(a.Actor), a.Object); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func RejectActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	good := vocab.ActivityVocabularyTypes{vocab.RejectType, vocab.TentativeRejectType}
	if !good.Contains(act.Type) {
		return act, errors.NotValidf("Activity has wrong type %s, expected %v", act.Type, good)
	}

	errs := make(multi, 0)
	if colSaver, ok := l.(CollectionStore); ok {
		inbox := vocab.Inbox.IRI(act.Actor)
		err := colSaver.RemoveFrom(inbox, act.Object.GetLink())
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return act, errs
	}
	return act, nil
}

const BlockedCollection = vocab.CollectionPath("blocked")

// BlockActivity
// The side effect of receiving this in an outbox is that the server SHOULD add the object to the actor's blocked Collection.
func BlockActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
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

	if colSaver, ok := l.(CollectionStore); ok {
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
func FlagActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
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
func IgnoreActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
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

	if colSaver, ok := l.(CollectionStore); ok {
		err := colSaver.AddTo(IgnoredCollection.IRI(act.Actor), obIRI)
		if err != nil {
			return act, err
		}
	}
	return act, nil
}
