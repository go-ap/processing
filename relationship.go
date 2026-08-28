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
func (p *P) RelationshipManagementActivity(act *vocab.Activity, receivedIn vocab.IRI) (*vocab.Activity, error) {
	if vocab.IsNil(act.Object) {
		return act, errors.BadRequestf("Missing object for %s Activity", act.Type)
	}
	if vocab.IsNil(act.Actor) {
		return act, errors.BadRequestf("Missing actor for %s Activity", act.Type)
	}
	switch {
	case vocab.FollowType.Match(act.Type):
		return FollowActivity(p, act, receivedIn)
	case vocab.RejectType.Match(act.Type):
		fallthrough
	case vocab.TentativeRejectType.Match(act.Type):
		return RejectActivity(p.s, act)
	case vocab.TentativeAcceptType.Match(act.Type):
		fallthrough
	case vocab.AcceptType.Match(act.Type):
		return AcceptActivity(p, act, receivedIn)
	case vocab.BlockType.Match(act.Type):
		//return p.BlockActivity(act, receivedIn)
		fallthrough
	case vocab.IgnoreType.Match(act.Type):
		//return p.IgnoreActivity(act, receivedIn)
		fallthrough
	case vocab.AddType.Match(act.Type):
		fallthrough
	case vocab.CreateType.Match(act.Type):
		fallthrough
	case vocab.DeleteType.Match(act.Type):
		fallthrough
	case vocab.InviteType.Match(act.Type):
		fallthrough
	default:
		return act, errors.NotImplementedf("Activity %s is not implemented", act.Type)
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
func FollowActivity(_ *P, act *vocab.Activity, receivedIn vocab.IRI) (*vocab.Activity, error) {
	if !vocab.IsNil(act.Object) {
		validForRecipient := func(i vocab.IRI) bool {
			return len(i) > 0 && !i.Equal(vocab.PublicNS)
		}

		// TODO(marius): add check if IRI represents an actor (or rely on the collection saver to break if not).
		//   This should be moved to the validation logic
		_ = vocab.OnItem(act.Object, func(object vocab.Item) error {
			if obIRI := object.GetLink(); validForRecipient(obIRI) {
				_ = act.To.Append(obIRI)
			}
			return nil
		})
	}
	return act, nil
}

var UndoableRelationshipActivityTypes = vocab.ActivityVocabularyTypes{
	vocab.FollowType, vocab.FlagType,
	vocab.IgnoreType, vocab.BlockType,
	vocab.AcceptType, vocab.TentativeAcceptType,
}

// UndoRelationshipManagementActivity
//
// Removes the side effects of an existing RelationshipActivity activity (Follow, Block, Ignore, Flag)
// Currently this means the removal of the object from the collection corresponding to the original Activity type.
// Block - removes the original object from the actor's blocked collection.
// Ignore - removes the original object from the actor's ignored collection.
// Flag - is a special case where there isn't a specific collection that needs to be operated on.
// Accept, TentativeAccept - undos the activity it has as an object
// Follow - removes the Follow's Object from its Actor following collection
//   - removes the Follow's Actor from its Object followers collection
func (p *P) UndoRelationshipManagementActivity(toUndo *vocab.Activity) (*vocab.Activity, error) {
	if toUndo == nil {
		return toUndo, InvalidActivity("nil relationship activity")
	}
	errs := make([]error, 0)

	toRemove := toUndo.GetLink()
	if p.IsLocal(toUndo.Actor) {
		outbox := vocab.Outbox.IRI(toUndo.Actor)
		// NOTE(marius): we need to remove the toUndo activity from the Outbox of its actor.
		if err := p.s.RemoveFrom(outbox, toRemove); err != nil && !errors.IsNotFound(err) {
			errs = append(errs, errors.Annotatef(err, "unable to remove from collection %s", outbox))
		}
	}

	// NOTE(marius): for all recipients we need to remove the activity from their Inbox'es.
	for _, rec := range toUndo.Recipients() {
		removeFrom := rec.GetLink()
		if removeFrom == "" || vocab.PublicNS.Equal(removeFrom) || !p.IsLocalIRI(removeFrom) {
			continue
		}
		if !vocab.ValidCollectionIRI(removeFrom) {
			// NOTE(marius): if recipient is not a  valid collection,
			//  we assume it represents an actor, and we try to get their Inbox
			if removeFrom = vocab.Inbox.IRI(rec); !p.IsLocalIRI(removeFrom) {
				continue
			}
		}
		if err := p.s.RemoveFrom(removeFrom, toRemove); err != nil && !errors.IsNotFound(err) {
			errs = append(errs, errors.Annotatef(err, "unable to remove from collection %s", removeFrom))
		}
	}

	// NOTE(marius): when this is just an IRI, we try to dereference it.
	//  This should probably be done at validation time.
	if vocab.IsIRI(toUndo.Object) {
		ob, err := p.dereferenceIRI(toUndo.Object.GetID())
		if err != nil {
			return toUndo, errors.Annotatef(err, "unable to dereference activity for Undo: %s", toUndo.Object.GetID())
		}
		toUndo.Object = ob
	}

	// NOTE(marius): here we undo the side-effects of each Activity type.
	//  Check their individual ProcessXXX methods to see what.
	removeCollectionOperations := make(map[vocab.IRI]vocab.ItemCollection)
	typ := toUndo.GetType()
	switch {
	case vocab.AcceptType.Match(typ), vocab.TentativeAcceptType.Match(typ):
		// NOTE(marius): when receiving Undo for Accept:
		//  * we need to Undo the Accept's Object
		_ = vocab.OnActivity(toUndo.Object, func(follow *vocab.Activity) error {
			return p.undoThisActivity(follow)
		})
	case vocab.FollowType.Match(typ):
		// NOTE(marius): when receiving Undo for Follow:
		//  * we need to remove the Follow's Object from it's Actor following collection
		//  * we need to remove the Follow's Actor from it's Object followers collection
		//  This assumes that there was a corresponding Accept sent to finalize the Follow operation.
		//  For the cases where that didn't happen, the following collection removals are noops.
		if colIRI := vocab.Following.IRI(toUndo.Actor); p.IsLocalIRI(colIRI) && !vocab.IsNil(toUndo.Object) {
			removeCollectionOperations[colIRI] = vocab.ItemCollection{toUndo.Object.GetID()}
		}
		if colIRI := vocab.Followers.IRI(toUndo.Object); p.IsLocalIRI(colIRI) && !vocab.IsNil(toUndo.Actor) {
			removeCollectionOperations[colIRI] = vocab.ItemCollection{toUndo.Actor.GetID()}
		}
	case vocab.BlockType.Match(typ):
		// NOTE(marius): when receiving Undo for Block:
		//  * we need to remove the Block's Object from the blocked collection of the Undo's Actor.
		if colIRI := BlockedCollection.IRI(toUndo.Actor); p.IsLocalIRI(colIRI) && !vocab.IsNil(toUndo.Object) {
			removeCollectionOperations[colIRI] = vocab.ItemCollection{toUndo.Object.GetID()}
		}
	case vocab.IgnoreType.Match(typ):
		// NOTE(marius): when receiving Undo for Ignore:
		//  * we need to remove the Ignore's Object from the ignored collection of the Undo's Actor.
		if colIRI := IgnoredCollection.IRI(toUndo.Actor); p.IsLocalIRI(colIRI) && !vocab.IsNil(toUndo.Object) {
			removeCollectionOperations[colIRI] = vocab.ItemCollection{toUndo.Object.GetID()}
		}
	}

	for colIRI, iris := range removeCollectionOperations {
		if err := p.s.RemoveFrom(colIRI, iris...); err != nil && !errors.IsNotFound(err) {
			errs = append(errs, errors.Annotatef(err, "unable to remove from collection %s", colIRI))
		}
	}
	if len(errs) > 0 {
		return toUndo, errors.Annotatef(errors.Join(errs...), "failed to Undo %s activity", toUndo.GetType())
	}
	return toUndo, nil
}
