package processing

import (
	"strings"

	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	s "github.com/go-ap/storage"
)

// NegatingActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-undo
//
// The Negating Activity use case primarily deals with the ability to redact previously completed activities.
// See 5.5 Inverse Activities and "Undo" for more information:
// https://www.w3.org/TR/activitystreams-vocabulary/#inverse
func NegatingActivity(l s.WriteStore, act *pub.Activity) (*pub.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	if act.Type != pub.UndoType {
		return act, errors.NotValidf("Activity has wrong type %s, expected %s", act.Type, pub.UndoType)
	}
	// TODO(marius): a lot of validation logic should be moved to the validation package
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
	// the object of the activity needs to be an activity
	if !pub.ActivityTypes.Contains(act.Object.GetType()) {
		return act, errors.NotValidf("Activity object has wrong type %s, expected one of %v", act.Type, pub.ActivityTypes)
	}
	err := pub.OnActivity(act.Object, func(a *pub.Activity) error {
		if act.Actor.GetLink() != a.Actor.GetLink() {
			return errors.NotValidf("The %s activity has a different actor than its object: %s, expected %s", act.Type, act.Actor.GetLink(), a.Actor.GetLink())
		}
		// TODO(marius): add more valid types
		good := pub.ActivityVocabularyTypes{pub.LikeType, pub.DislikeType, pub.BlockType, pub.FollowType}
		if !good.Contains(a.Type) {
			return errors.NotValidf("Object Activity has wrong type %s, expected %v", a.Type, good)
		}
		return nil
	})
	if err != nil {
		return act, err
	}
	return UndoActivity(l, act)
}

// UndoActivity
//
// https://www.w3.org/TR/activitypub/#undo-activity-outbox
//
// The Undo activity is used to undo a previous activity. See the Activity Vocabulary documentation on
// Inverse Activities and "Undo". For example, Undo may be used to undo a previous Like, Follow, or Block.
// The undo activity and the activity being undone MUST both have the same actor.
// Side effects should be undone, to the extent possible. For example, if undoing a Like, any counter that had been
// incremented previously should be decremented appropriately.
// There are some exceptions where there is an existing and explicit "inverse activity" which should be used instead.
// Create based activities should instead use Delete, and Add activities should use Remove.
//
// https://www.w3.org/TR/activitypub/#undo-activity-inbox
//
// The Undo activity is used to undo the side effects of previous activities. See the ActivityStreams documentation
// on Inverse Activities and "Undo". The scope and restrictions of the Undo activity are the same as for the Undo
// activity in the context of client to server interactions, but applied to a federated context.
func UndoActivity(r s.WriteStore, act *pub.Activity) (*pub.Activity, error) {
	var err error

	iri := act.GetLink()
	if len(iri) == 0 {
		createID(act.Object, pub.Outbox.IRI(act.Actor), nil)
	}
	err = pub.OnActivity(act.Object, func(toUndo *pub.Activity) error {
		for _, to := range act.Bto {
			if !toUndo.Bto.Contains(to.GetLink()) {
				toUndo.Bto = append(toUndo.Bto, to)
			}
		}
		for _, to := range act.BCC {
			if !toUndo.BCC.Contains(to.GetLink()) {
				toUndo.BCC = append(toUndo.BCC, to)
			}
		}
		switch toUndo.GetType() {
		case pub.DislikeType:
			// TODO(marius): Dislikes should not trigger a removal from Likes/Liked collections
			fallthrough
		case pub.LikeType:
			UndoAppreciationActivity(r, toUndo)
		case pub.FollowType:
			fallthrough
		case pub.BlockType:
			fallthrough
		case pub.FlagType:
			fallthrough
		case pub.IgnoreType:
			return errors.NotImplementedf("Undoing %s is not implemented", toUndo.GetType())
		}
		return nil
	})
	if err != nil {
		return act, err
	}

	err = r.Delete(act.Object)
	return act, err
}

// UndoAppreciationActivity
//
// Removes the side effects of an existing Appreciation activity (Like or Dislike)
// Currently this means only removal of the Liked/Disliked object from the actor's `liked` collection and
// removal of the Like/Dislike Activity from the object's `likes` collection
func UndoAppreciationActivity(r s.WriteStore, act *pub.Activity) (*pub.Activity, error) {
	errs := make([]error, 0)
	rem := act.GetLink()
	colSaver, ok := r.(s.CollectionStore)
	if !ok {
		return act, nil
	}
	allRec := act.Recipients()
	removeFromCols := make(pub.IRIs, 0)
	removeFromCols = append(removeFromCols, pub.Outbox.IRI(act.Actor))
	removeFromCols = append(removeFromCols, pub.Liked.IRI(act.Actor))
	removeFromCols = append(removeFromCols, pub.Likes.IRI(act.Object))
	for _, rec := range allRec {
		iri := rec.GetLink()
		if iri == pub.PublicNS {
			continue
		}
		if !pub.ValidCollectionIRI(iri) {
			// if not a valid collection, then the current iri represents an actor, and we need their inbox
			removeFromCols = append(removeFromCols, pub.Inbox.IRI(iri))
		}
	}
	for _, iri := range removeFromCols {
		if err := colSaver.RemoveFrom(iri, rem); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return act, errors.Newf("%s", strings.Join(msgs, ", "))
	}
	return act, nil
}
