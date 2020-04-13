package processing

import (
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
	"path"
)

// RelationshipManagementActivity processes matching activities
// The Relationship Management use case primarily deals with representing activities involving the management
// of interpersonal and social relationships (e.g. friend requests, management of social network, etc).
// See 5.2 Representing Relationships Between Entities for more information:
// https://www.w3.org/TR/activitystreams-vocabulary/#connections
func RelationshipManagementActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	switch act.Type {
	case pub.FollowType:
		return FollowActivity(l, act)
	case pub.AcceptType:
		fallthrough
	case pub.AddType:
		fallthrough
	case pub.BlockType:
		fallthrough
	case pub.CreateType:
		fallthrough
	case pub.DeleteType:
		fallthrough
	case pub.IgnoreType:
		fallthrough
	case pub.InviteType:
		fallthrough
	case pub.RejectType:
		fallthrough
	default:
		return act, errors.NotImplementedf("Activity %s is not implemented", act.GetType())
	}
	return act, nil
}

// FollowActivity
// is used when following an actor.
func FollowActivity(r s.Saver, act *pub.Activity) (*pub.Activity, error) {
	ob := act.Object.GetLink()
	if colSaver, ok := r.(s.CollectionSaver); ok {
		if !handlers.ValidCollection(path.Base(ob.String())) {
			// TODO(marius): add check if IRI represents an actor (or rely on the collection saver to break if not)
			ob = handlers.Inbox.IRI(ob)
		}
		err := colSaver.AddToCollection(ob, act.GetLink())
		if err != nil {
			return act, err
		}
	}
	return act, nil
}
