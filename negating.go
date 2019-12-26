package processing

import (
	"fmt"
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
	"path"
	"strings"
)

// NegatingActivity processes matching activities
// The Negating Activity use case primarily deals with the ability to redact previously completed activities.
// See 5.5 Inverse Activities and "Undo" for more information:
// https://www.w3.org/TR/activitystreams-vocabulary/#inverse
func NegatingActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
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
		if actLoader, ok := l.(s.ActivityLoader); ok {
			obj, cnt, err := actLoader.LoadActivities(act.Object.GetLink())
			if err != nil {
				return act, errors.NotValidf("Unable to dereference object: %s", act.Object.GetLink())
			}
			if cnt != 1 {
				return act, errors.NotValidf("Too many objects to dereference object: %s", act.Object.GetLink())
			}
			act.Object = obj.First()
		}
	}
	// the object of the activity needs to be an activity
	if !pub.ActivityTypes.Contains(act.Object.GetType()) {
		return act, errors.NotValidf("Activity object has wrong type %s, expected one of %v", act.Type, pub.ActivityTypes)
	}
	err := pub.OnActivity(act.Object, func(a *pub.Activity) error {
		if act.Actor.GetLink() != a.Actor.GetLink() {
			return errors.NotValidf("The Undo activity has a different actor than its object: %s, expected %s", act.Actor.GetLink(), a.Actor.GetLink())
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
// The Undo activity is used to undo a previous activity. See the Activity Vocabulary documentation on
// Inverse Activities and "Undo". For example, Undo may be used to undo a previous Like, Follow, or Block.
// The undo activity and the activity being undone MUST both have the same actor.
// Side effects should be undone, to the extent possible. For example, if undoing a Like, any counter that had been
// incremented previously should be decremented appropriately.
// There are some exceptions where there is an existing and explicit "inverse activity" which should be used instead.
// Create based activities should instead use Delete, and Add activities should use Remove.
func UndoActivity(r s.Saver, act *pub.Activity) (*pub.Activity, error) {
	var err error

	iri := act.GetLink()
	if len(iri) == 0 {
		r.GenerateID(act, nil)
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
	return act, err
}

// UndoAppreciationActivity
// Removes the side effects of an existing Appreciation activity (Like or Dislike)
// Currently this means only removal of the Liked/Disliked object from the actor's `liked` collection and
// removal of the Like/Dislike Activity from the object's `likes` collection
func UndoAppreciationActivity(r s.Saver, act *pub.Activity) (*pub.Activity, error) {
	errs := make([]error, 0)
	rem := act.GetLink()
	if colSaver, ok := r.(s.CollectionSaver); ok {
		rec := act.Recipients()
		for _, rec := range rec {
			iri := rec.GetLink()
			if iri == pub.PublicNS {
				continue
			}
			base := path.Base(string(iri))
			if !handlers.ValidCollection(base) {
				// if not a valid collection, then it's an actor and we need their inbox
				iri = pub.IRI(fmt.Sprintf("%s/%s", iri, handlers.Inbox))
			}
			if err := colSaver.RemoveFromCollection(iri, rem); err != nil {
				errs = append(errs, err)
			}

		}
		outbox := pub.IRI(fmt.Sprintf("%s/%s", act.Actor.GetLink(), handlers.Outbox))
		if err := colSaver.RemoveFromCollection(outbox, rem); err != nil {
			errs = append(errs, err)
		}
		liked := pub.IRI(fmt.Sprintf("%s/%s", act.Actor.GetLink(), handlers.Liked))
		if err := colSaver.RemoveFromCollection(liked, act.Object.GetLink()); err != nil {
			errs = append(errs, err)
		}
		likes := pub.IRI(fmt.Sprintf("%s/%s", act.Object.GetLink(), handlers.Likes))
		if err := colSaver.RemoveFromCollection(likes, rem); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		err := errors.Newf("%s", strings.Join(msgs, ", "))
		return act, err
	}
	return act, nil
}
