package processing

import (
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// RelationshipManagementActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-relationships
//
// The Relationship Management use case primarily deals with representing activities involving the management
// of interpersonal and social relationships (e.g. friend requests, management of social network, etc).
// See 5.2 Representing Relationships Between Entities for more information:
// https://www.w3.org/TR/activitystreams-vocabulary/#connections
func RelationshipManagementActivity(p P, act *vocab.Activity, receivedIn vocab.IRI) (*vocab.Activity, error) {
	if vocab.IsNil(act.Object) {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if vocab.IsNil(act.Actor) {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	switch act.Type {
	case vocab.FollowType:
		return FollowActivity(p, act, receivedIn)
	case vocab.RejectType:
		fallthrough
	case vocab.TentativeRejectType:
		return RejectActivity(p.s, act)
	case vocab.TentativeAcceptType:
		fallthrough
	case vocab.AcceptType:
		return AcceptActivity(p, act, receivedIn)
	case vocab.BlockType:
		return BlockActivity(p, act, receivedIn)
	case vocab.AddType:
		fallthrough
	case vocab.CreateType:
		fallthrough
	case vocab.DeleteType:
		fallthrough
	case vocab.IgnoreType:
		fallthrough
	case vocab.InviteType:
		fallthrough
	default:
		return act, errors.NotImplementedf("Activity %s is not implemented", act.GetType())
	}
	return act, nil
}

// FollowActivity is used when following an actor.
//
// https://www.w3.org/TR/activitypub/#follow-activity-outbox
//
// The Follow activity is used to subscribe to the activities of another actor.
// The side effect of receiving this in an outbox is that the server SHOULD add the object to the actor's following
// Collection when and only if an Accept activity is subsequently received with this Follow activity as its object.
//
// https://www.w3.org/TR/activitypub/#follow-activity-inbox
//
// The side effect of receiving this in an inbox is that the server SHOULD generate either an Accept or Reject
// activity with the Follow as the object and deliver it to the actor of the Follow. The Accept or Reject MAY be
// generated automatically, or MAY be the result of user input (possibly after some delay in which the user reviews).
// Servers MAY choose to not explicitly send a Reject in response to a Follow, though implementors ought to be aware
// that the server sending the request could be left in an intermediate state. For example, a server might not send
// a Reject to protect a user's privacy.
// In the case of receiving an "Accept" referencing this Follow as the object, the server SHOULD add the actor to the
// object actor's Followers Collection. In the case of a Reject, the server MUST NOT add the actor to the object
// actor's Followers Collection.
//
// NOTE: Sometimes a successful Follow subscription may occur but at some future point delivery to the follower
// fails for an extended period of time. Implementations should be aware that there is no guarantee that actors on
// the network will remain reachable and should implement accordingly. For instance, if attempting to deliver to
// an actor for perhaps six months while the follower remains unreachable, it is reasonable that the delivering
// server remove the subscriber from the followers list. Timeframes and behavior for dealing with unreachable
// actors are left to the discretion of the delivering server.
func FollowActivity(p P, act *vocab.Activity, receivedIn vocab.IRI) (*vocab.Activity, error) {
	if !vocab.IsNil(act.Object) && !act.To.Contains(act.Object.GetLink()) {
		// TODO(marius): add check if IRI represents an actor (or rely on the collection saver to break if not)
		act.To.Append(act.Object.GetLink())
	}
	return act, nil
}
