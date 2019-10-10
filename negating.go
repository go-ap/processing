package processing


import (
	"fmt"
	"github.com/go-ap/activitypub"
	as "github.com/go-ap/activitystreams"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
)

// NegatingActivity processes matching activities
// The Negating Activity use case primarily deals with the ability to redact previously completed activities.
// See 5.5 Inverse Activities and "Undo" for more information:
// https://www.w3.org/TR/activitystreams-vocabulary/#inverse
func NegatingActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	if act.Type != as.UndoType {
		return act, errors.NotValidf("Activity has wrong type %s, expected %s", act.Type, as.UndoType)
	}
	// the object of the activity needs to be an activity
	if !as.ActivityTypes.Contains(act.Object.GetType()) {
		return act, errors.NotValidf("Activity object has wrong type %s, expected one of %v", act.Type, as.ActivityTypes)
	}
	// dereference object activity
	if act.Object.IsLink() {
		if actLoader, ok := l.(s.ActivityLoader); ok {
			obj, _, err := actLoader.LoadActivities(act.Object.GetLink())
			if err != nil {
				return act, errors.NotValidf("Unable to dereference object: %s", act.Object.GetLink())
			}
			act.Object = obj
		}
	}
	err := activitypub.OnActivity(act.Object, func(a *as.Activity) error {
		if act.Actor.GetLink() != a.Actor.GetLink() {
			return errors.NotValidf("The Undo activity has a different actor than its object: %s, expected %s", act.Actor.GetLink(), a.Actor.GetLink())
		}
		// TODO(marius): add more valid types
		good := as.ActivityVocabularyTypes{as.LikeType, as.DislikeType, as.BlockType, as.FollowType}
		if !good.Contains(act.Type) {
			return errors.NotValidf("Object Activity has wrong type %s, expected %v", act.Type, good)
		}
		return nil
	})
	if err != nil {
		return act, err
	}
	return UndoActivity(l, act)
}

// UndoActivity
func UndoActivity(r s.Saver, act *as.Activity) (*as.Activity, error) {
	var err error

	iri := act.GetLink()
	if len(iri) == 0 {
		r.GenerateID(act, nil)
	}
	err = activitypub.OnActivity(act.Object, func(toUndo *as.Activity) error {
		switch toUndo.GetType() {
		case as.DislikeType:
			fallthrough
		case as.LikeType:
			UndoAppreciationActivity(r, toUndo)
		case as.BlockType:
			fallthrough
		case as.FlagType:
			fallthrough
		case as.IgnoreType:
			return errors.NotImplementedf("Undoing %s is not implemented", toUndo.GetType())
		}
		return nil
	})
	return act, err
}

// UndoAppreciationActivity
func UndoAppreciationActivity(r s.Saver, act *as.Activity) (*as.Activity, error) {
	var err error
	if colSaver, ok := r.(s.CollectionSaver); ok {
		liked := as.IRI(fmt.Sprintf("%s/%s", act.Actor.GetLink(), handlers.Liked))
		err = colSaver.RemoveFromCollection(liked, act.Object.GetLink())

		likes := as.IRI(fmt.Sprintf("%s/%s", act.Object.GetLink(), handlers.Likes))
		err = colSaver.RemoveFromCollection(likes, act.GetLink())
	}

	return act, err
}
