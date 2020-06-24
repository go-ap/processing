package processing

import (
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
)

// ReactionsActivity processes matching activities
// The Reactions use case primarily deals with reactions to content.
// This can include activities such as liking or disliking content, ignoring updates,
// flagging content as being inappropriate, accepting or rejecting objects, etc.
func ReactionsActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
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
			return act, errors.NotImplementedf("Processing reaction activity of type %s is not implemented", act.GetType())
		}
	}

	return act, err
}

// AppreciationActivity
// The Like(and Dislike) activity indicates the actor likes the object.
// The side effect of receiving this in an outbox is that the server SHOULD add the object to the actor's liked Collection.
func AppreciationActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
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

	if colSaver, ok := l.(s.CollectionSaver); ok {
		liked := handlers.Liked.IRI(act.Actor)
		if err := colSaver.AddToCollection(liked, act.Object.GetLink()); err != nil {
			return act, errors.Annotatef(err, "Unable to save %s to collection %s", act.Object.GetType(), liked)
		}
		likes := handlers.Likes.IRI(act.Object)
		if err := colSaver.AddToCollection(likes, act.GetLink()); err != nil {
			return act, errors.Annotatef(err, "Unable to save %s to collection %s", act.GetType(), likes)
		}
	}

	return act, nil
}

// AcceptActivity
// The side effect of receiving this in an inbox is that the server SHOULD add the object to the actor's followers Collection.
func AcceptActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
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
		if actLoader, ok := l.(s.ActivityLoader); ok {
			obj, cnt, err := actLoader.LoadActivities(act.Object.GetLink())
			if err != nil {
				return act, errors.NotValidf("Unable to dereference object: %s", act.Object.GetLink())
			}
			if cnt != 1 {
				return act, errors.NotValidf("Too many objects to dereference for IRI: %s", act.Object.GetLink())
			}
			act.Object = obj.First()
		}
	}
	err := pub.OnActivity(act.Object, func(a *pub.Activity) error {
		if act.Actor.GetLink() != a.Object.GetLink() {
			return errors.NotValidf("The %s activity has a different actor than its object: %s, expected %s", act.Type, act.Actor.GetLink(), a.Actor.GetLink())
		}
		good := pub.ActivityVocabularyTypes{pub.FollowType}
		if !good.Contains(a.Type) {
			return errors.NotValidf("Object Activity has wrong type %s, expected %v", a.Type, good)
		}
		followers := handlers.Followers.IRI(act.Actor)
		following := handlers.Following.IRI(a.Actor)
		if colSaver, ok := l.(s.CollectionSaver); ok {
			if err := colSaver.AddToCollection(following, a.Object.GetLink()); err != nil {
				return err
			}
			if err := colSaver.AddToCollection(followers, a.Actor.GetLink()); err != nil {
				return err
			}
		}
		return nil
	})
	return act, err
}

func RejectActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
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

	if colSaver, ok := l.(s.CollectionSaver); ok {
		inbox := handlers.Inbox.IRI(act.Actor)
		err := colSaver.RemoveFromCollection(inbox, act.Object.GetLink())
		if err != nil {
			return act, err
		}
	}
	return act, nil
}

// BlockActivity
// The side effect of receiving this in an outbox is that the server SHOULD add the object to the actor's blocked Collection.
func BlockActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
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

	b := handlers.CollectionType("blocked")
	if colSaver, ok := l.(s.CollectionSaver); ok {
		blocked := b.IRI(act.Actor)
		err := colSaver.AddToCollection(blocked, obIRI)
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
func FlagActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
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
			// Remove object's author from any recipients collections
			toRemoveIRI = o.AttributedTo.GetLink()
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
