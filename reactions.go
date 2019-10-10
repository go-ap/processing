package processing

import (
	"fmt"
	as "github.com/go-ap/activitystreams"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
)

// ReactionsActivity processes matching activities
// The Reactions use case primarily deals with reactions to content.
// This can include activities such as liking or disliking content, ignoring updates,
// flagging content as being inappropriate, accepting or rejecting objects, etc.
func ReactionsActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	var err error
	if act.Object != nil {
		switch act.Type {
		case as.DislikeType:
			fallthrough
		case as.LikeType:
			act, err = AppreciationActivity(l, act)
		case as.BlockType:
			fallthrough
		case as.AcceptType:
			// TODO(marius): either the actor or the object needs to be local for this action to be valid
			// in the case of C2S... the actor needs to be local
			// in the case of S2S... the object needs to be local
			fallthrough
		case as.FlagType:
			fallthrough
		case as.IgnoreType:
			fallthrough
		case as.RejectType:
			fallthrough
		case as.TentativeAcceptType:
			fallthrough
		case as.TentativeRejectType:
			return act, errors.NotImplementedf("Processing reaction activity of type %s is not implemented", act.GetType())
		}
	}
	return act, err
}

// AppreciationActivity
func AppreciationActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	good := as.ActivityVocabularyTypes{as.LikeType, as.DislikeType}
	if !good.Contains(act.Type) {
		return act, errors.NotValidf("Activity has wrong type %s, expected %v", act.Type, good)
	}

	if colSaver, ok := l.(s.CollectionSaver); ok {
		liked := as.IRI(fmt.Sprintf("%s/%s", act.Actor.GetLink(), handlers.Liked))
		if err := colSaver.AddToCollection(liked, act.Object.GetLink()); err != nil {
			return act, errors.Annotatef(err, "Unable to save item to collection %s", liked)
		}
		likes := as.IRI(fmt.Sprintf("%s/%s", act.Object.GetLink(), handlers.Likes))
		if err := colSaver.AddToCollection(likes, act.GetLink()); err != nil {
			return act, errors.Annotatef(err, "Unable to save item to collection %s", likes)
		}
	}

	return act, nil
}
