package processing

import (
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// ValidUndoActivityTypes are the types we currently support operating Undo on
var ValidUndoActivityTypes = append(UndoableRelationshipActivityTypes, vocab.CreateType, vocab.AnnounceType)

// ValidateClientNegatingActivity
func (p P) ValidateClientNegatingActivity(act *vocab.Activity) error {
	if vocab.IsNil(act.Object) {
		return InvalidActivityObject("is nil")
	}

	if ob, err := p.DereferenceItem(act.Object); err != nil {
		return err
	} else {
		act.Object = ob
	}
	return vocab.OnActivity(act.Object, func(objAct *vocab.Activity) error {
		if !act.Actor.GetLink().Equals(objAct.Actor.GetLink(), false) {
			return errors.BadRequestf("The %s activity has a different actor than its object: %s, expected %s", act.Type, act.Actor.GetLink(), objAct.Actor.GetLink())
		}
		if !ValidUndoActivityTypes.Match(objAct.Type) {
			return errors.BadRequestf("Object Activity has wrong type %s, expected one of %v", objAct.Type, ValidUndoActivityTypes)
		}
		return nil
	})
}

// NegatingActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-undo
//
// The Negating Activity use case primarily deals with the ability to redact previously completed activities.
// See 5.5 Inverse Activities and "Undo" for more information:
// https://www.w3.org/TR/activitystreams-vocabulary/#inverse
func (p *P) NegatingActivity(undo *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(undo.Object) {
		return undo, errors.BadRequestf("Missing object for %s Activity", undo.Type)
	}
	if vocab.IsNil(undo.Actor) {
		return undo, errors.BadRequestf("Missing actor for %s Activity", undo.Type)
	}
	if !vocab.UndoType.Match(undo.Type) {
		return undo, errors.BadRequestf("Activity has wrong type %s, expected %s", undo.Type, vocab.UndoType)
	}
	return p.UndoActivity(undo)
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
func (p *P) UndoActivity(undo *vocab.Activity) (*vocab.Activity, error) {
	iri := undo.GetLink()
	if len(iri) == 0 {
		iri, _ = p.createIDFn(undo, nil)
	}
	_ = vocab.OnActivity(undo.Object, func(toUndo *vocab.Activity) error {
		_ = toUndo.Bto.Append(undo.Bto...)
		_ = toUndo.BCC.Append(undo.BCC...)
		return nil
	})
	return undo, p.undoThisActivity(undo.Object)
}

func (p *P) undoThisActivity(toUndo vocab.Item) error {
	err := vocab.OnActivity(toUndo, func(toUndo *vocab.Activity) error {
		var err error
		typ := toUndo.GetType()
		switch {
		case vocab.CreateType.Match(typ):
			_, err = p.UndoCreateActivity(toUndo)
		case vocab.DislikeType.Match(typ):
			// TODO(marius): Dislikes should not trigger a removal from Likes/Liked collections
			fallthrough
		case vocab.LikeType.Match(typ):
			_, err = p.UndoAppreciationActivity(toUndo)
		case UndoableRelationshipActivityTypes.Match(typ):
			_, err = p.UndoRelationshipManagementActivity(toUndo)
		case vocab.AnnounceType.Match(typ):
			_, err = p.UndoAnnounceActivity(toUndo)
		}
		return err
	})
	if err != nil {
		return err
	}

	if p.IsLocal(toUndo) {
		// NOTE(marius): remove the activity that we operated Undo on
		err = p.s.Delete(toUndo.GetLink())
		if err != nil && errors.IsNotFound(err) {
			err = nil
		}
	}
	return err
}

var UndoableRelationshipActivityTypes = vocab.ActivityVocabularyTypes{
	vocab.FollowType, vocab.FlagType,
	vocab.IgnoreType, vocab.BlockType,
	vocab.AcceptType, vocab.TentativeAcceptType,
}

// UndoCreateActivity
//
// Removes the side effects of an existing Create activity
// Currently this means only removal of the Create object
func (p P) UndoCreateActivity(create *vocab.Activity) (*vocab.Activity, error) {
	if create == nil {
		return create, InvalidActivity("nil Create activity")
	}
	errs := make([]error, 0)
	toRemove := create.GetLink()

	allRec := create.Recipients()
	removeFromCols := make(vocab.IRIs, 0)
	if p.IsLocal(create.Actor) {
		_ = removeFromCols.Append(vocab.Outbox.IRI(create.Actor))
	}
	for _, rec := range allRec {
		recIRI := rec.GetLink()
		if recIRI == vocab.PublicNS || !p.IsLocalIRI(recIRI) {
			continue
		}
		if !vocab.ValidCollectionIRI(recIRI) {
			// if not a valid collection, then the current recIRI represents an actor, and we need their inbox
			if recIRI = vocab.Inbox.IRI(recIRI); !p.IsLocalIRI(recIRI) {
				continue
			}
		}
		_ = removeFromCols.Append(recIRI)
	}
	for _, removeFrom := range removeFromCols {
		if err := p.s.RemoveFrom(removeFrom, toRemove); err != nil {
			errs = append(errs, errors.Annotatef(err, "unable to remove from collection %s", removeFrom))
		}
	}
	if len(errs) > 0 {
		return create, errors.Annotatef(errors.Join(errs...), "failed to Undo Create activity")
	}
	err := vocab.OnItem(create.Object, func(ob vocab.Item) error {
		if !p.IsLocal(ob) {
			return nil
		}
		return p.s.Delete(ob.GetLink())
	})
	return create, err
}

// UndoAppreciationActivity
//
// Removes the side effects of an existing Appreciation activity (Like or Dislike)
// Currently this means only removal of the Liked/Disliked object from the actor's `liked` collection and
// removal of the Like/Dislike Activity from the object's `likes` collection
func (p *P) UndoAppreciationActivity(like *vocab.Activity) (*vocab.Activity, error) {
	if like == nil {
		return like, InvalidActivity("nil Like activity")
	}
	errs := make([]error, 0)
	toRemove := like.GetLink()

	allRec := like.Recipients()
	removeFromCols := make(vocab.IRIs, 0)
	if p.IsLocal(like.Actor) {
		_ = removeFromCols.Append(vocab.Outbox.IRI(like.Actor), vocab.Liked.IRI(like.Actor))
	}
	if p.IsLocal(like.Object) {
		_ = removeFromCols.Append(vocab.Likes.IRI(like.Object))
	}
	for _, rec := range allRec {
		recIRI := rec.GetLink()
		if recIRI == "" || recIRI == vocab.PublicNS {
			continue
		}
		if !vocab.ValidCollectionIRI(recIRI) {
			// if not a valid collection, then the current recIRI represents an actor, and we need their inbox
			if recIRI = vocab.Inbox.IRI(recIRI); !p.IsLocalIRI(recIRI) {
				continue
			}
		}
		_ = removeFromCols.Append(recIRI)
	}
	for _, removeFrom := range removeFromCols {
		if err := p.s.RemoveFrom(removeFrom, toRemove); err != nil {
			errs = append(errs, errors.Annotatef(err, "unable to remove from collection %s", removeFrom))
		}
	}
	if len(errs) > 0 {
		return like, errors.Annotatef(errors.Join(errs...), "failed Undo Like activity")
	}
	return like, nil
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
		if err := p.s.RemoveFrom(outbox, toRemove); err != nil {
			errs = append(errs, errors.Annotatef(err, "unable to remove from collection %s %v", outbox, toRemove))
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
		if err := p.s.RemoveFrom(removeFrom, toRemove); err != nil {
			errs = append(errs, errors.Annotatef(err, "unable to remove from collection %s %v", removeFrom, toRemove))
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
			removeCollectionOperations[colIRI] = vocab.ItemCollection{toUndo.Object}
		}
		if colIRI := vocab.Followers.IRI(toUndo.Object); p.IsLocalIRI(colIRI) && !vocab.IsNil(toUndo.Actor) {
			removeCollectionOperations[colIRI] = vocab.ItemCollection{toUndo.Actor}
		}
	case vocab.BlockType.Match(typ):
		// NOTE(marius): when receiving Undo for Block:
		//  * we need to remove the Block's Object from the blocked collection of the Undo's Actor.
		if colIRI := BlockedCollection.Of(toUndo.Actor).GetLink(); p.IsLocalIRI(colIRI) && !vocab.IsNil(toUndo.Object) {
			removeCollectionOperations[colIRI] = vocab.ItemCollection{toUndo.Object}
		}
	case vocab.IgnoreType.Match(typ):
		// NOTE(marius): when receiving Undo for Ignore:
		//  * we need to remove the Ignore's Object from the ignored collection of the Undo's Actor.
		if colIRI := IgnoredCollection.Of(toUndo.Actor).GetLink(); p.IsLocalIRI(colIRI) && !vocab.IsNil(toUndo.Object) {
			removeCollectionOperations[colIRI] = vocab.ItemCollection{toUndo.Object}
		}
	}

	for colIRI, iris := range removeCollectionOperations {
		if err := p.s.RemoveFrom(colIRI, iris...); err != nil {
			errs = append(errs, errors.Annotatef(err, "unable to remove from collection %s %v", colIRI, iris))
		}
	}
	if len(errs) > 0 {
		return toUndo, errors.Annotatef(errors.Join(errs...), "failed to Undo %s activity", toUndo.GetType())
	}
	return toUndo, nil
}
