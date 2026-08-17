package processing

import (
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// TODO(marius): add more valid types
var validUndoActivityTypes = vocab.ActivityVocabularyTypes{
	vocab.CreateType, /* vocab.UndoType, vocab.DeleteType,*/
	vocab.LikeType, vocab.DislikeType,
	vocab.BlockType, vocab.FollowType,
	vocab.AnnounceType,
}

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
		if !validUndoActivityTypes.Match(objAct.Type) {
			return errors.BadRequestf("Object Activity has wrong type %s, expected one of %v", objAct.Type, validUndoActivityTypes)
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
func (p P) NegatingActivity(undo *vocab.Activity) (*vocab.Activity, error) {
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
func (p P) UndoActivity(undo *vocab.Activity) (*vocab.Activity, error) {
	var err error

	iri := undo.GetLink()
	if len(iri) == 0 {
		iri, _ = p.createIDFn(undo, nil)
	}
	err = vocab.OnActivity(undo.Object, func(toUndo *vocab.Activity) error {
		for _, to := range undo.Bto {
			if !toUndo.Bto.Contains(to.GetLink()) {
				toUndo.Bto = append(toUndo.Bto, to)
			}
		}
		for _, to := range undo.BCC {
			if !toUndo.BCC.Contains(to.GetLink()) {
				toUndo.BCC = append(toUndo.BCC, to)
			}
		}
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
		return undo, err
	}

	if p.IsLocal(undo.Object) {
		// NOTE(marius): remove the activity that we operated Undo on
		if err = p.s.Delete(undo.Object.GetLink()); err != nil && !errors.IsNotFound(err) {
			return undo, err
		}
	}
	return undo, nil
}

var UndoableRelationshipActivityTypes = vocab.ActivityVocabularyTypes{
	vocab.FollowType, vocab.FlagType,
	vocab.IgnoreType, vocab.BlockType,
}

// UndoCreateActivity
//
// Removes the side effects of an existing Create activity
// Currently this means only removal of the Create object
func (p P) UndoCreateActivity(create *vocab.Activity) (*vocab.Activity, error) {
	errs := make([]error, 0)
	rem := create.GetLink()

	allRec := create.Recipients()
	removeFromCols := make(vocab.IRIs, 0)
	if p.IsLocal(create.Actor) {
		removeFromCols = append(removeFromCols, vocab.Outbox.IRI(create.Actor))
	}
	for _, rec := range allRec {
		recIRI := rec.GetLink()
		if recIRI == vocab.PublicNS || !p.IsLocalIRI(recIRI) {
			continue
		}
		if !vocab.ValidCollectionIRI(recIRI) {
			// if not a valid collection, then the current recIRI represents an actor, and we need their inbox
			removeFromCols = append(removeFromCols, vocab.Inbox.IRI(recIRI))
		}
	}
	for _, iri := range removeFromCols {
		if err := p.s.RemoveFrom(iri, rem); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return create, errors.Annotatef(errors.Join(errs...), "failed to fully process Undo activity")
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
func (p P) UndoAppreciationActivity(like *vocab.Activity) (*vocab.Activity, error) {
	errs := make([]error, 0)
	rem := like.GetLink()

	allRec := like.Recipients()
	removeFromCols := make(vocab.IRIs, 0)
	if p.IsLocal(like.Actor) {
		removeFromCols = append(removeFromCols, vocab.Outbox.IRI(like.Actor))
		removeFromCols = append(removeFromCols, vocab.Liked.IRI(like.Actor))
	}
	if p.IsLocal(like.Object) {
		removeFromCols = append(removeFromCols, vocab.Likes.IRI(like.Object))
	}
	for _, rec := range allRec {
		recIRI := rec.GetLink()
		if recIRI == vocab.PublicNS || !p.IsLocalIRI(recIRI) {
			continue
		}
		if !vocab.ValidCollectionIRI(recIRI) {
			// if not a valid collection, then the current recIRI represents an actor, and we need their inbox
			removeFromCols = append(removeFromCols, vocab.Inbox.IRI(recIRI))
		}
	}
	for _, iri := range removeFromCols {
		if err := p.s.RemoveFrom(iri, rem); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return like, errors.Annotatef(errors.Join(errs...), "failed to fully process Undo activity")
	}
	return like, nil
}

// UndoRelationshipManagementActivity
//
// Removes the side effects of an existing RelationshipActivity activity (Follow, Block, Ignore, Flag)
// Currently this means the removal of the object from the collection corresponding to the original Activity type.
// Follow - removes the original object from the actor's followers collection.
// Block - removes the original object from the actor's blocked collection.
// Ignore - removes the original object from the actor's ignored collection.
// Flag - is a special case where there isn't a specific collection that needs to be operated on.
func (p *P) UndoRelationshipManagementActivity(toUndo *vocab.Activity) (*vocab.Activity, error) {
	errs := make([]error, 0)
	rem := toUndo.GetLink()
	// NOTE(marius): we need to remove the toUndo activity from the Outbox of its actor.
	if err := p.s.RemoveFrom(vocab.Outbox.Of(toUndo.Actor).GetLink(), rem); err != nil {
		errs = append(errs, err)
	}

	// NOTE(marius): for all recipients we need to remove the activity from their Inbox'es.
	for _, rec := range toUndo.Recipients() {
		removeFrom := rec.GetLink()
		if removeFrom == vocab.PublicNS || !p.IsLocalIRI(removeFrom) {
			continue
		}
		if !vocab.ValidCollectionIRI(removeFrom) {
			// NOTE(marius): if recipient is not a  valid collection,
			// then the current iri represents an actor, and we try to get their Inbox
			removeFrom = vocab.Inbox.Of(rec).GetLink()
		}
		if err := p.s.RemoveFrom(removeFrom, rem); err != nil {
			errs = append(errs, err)
		}
	}

	removeFromCols := make(vocab.IRIs, 0)
	typ := toUndo.GetType()
	switch {
	case vocab.RejectType.Match(typ):
		// TODO(marius): I don't think there's any side-effect for Reject activities.
	case vocab.AcceptType.Match(typ):
		// TODO(marius): when receiving an Undo:
		//  * for Accept(Follow) - we need to remove the Follow's actor from the Undo's actor Followers collection
		if colIRI := vocab.Followers.Of(toUndo.Actor).GetLink(); p.IsLocalIRI(colIRI) {
			removeFromCols = append(removeFromCols, colIRI)
		}
	case vocab.FollowType.Match(typ):
		// NOTE(marius): when receiving Undo:
		//  * for Follow - we need to remove the Follow from its Actor Outbox, and also from its Object Inbox
		if colIRI := vocab.Following.Of(toUndo.Actor).GetLink(); p.IsLocalIRI(colIRI) {
			removeFromCols = append(removeFromCols, colIRI)
		}
		if colIRI := vocab.Followers.Of(toUndo.Object).GetLink(); p.IsLocalIRI(colIRI) {
			removeFromCols = append(removeFromCols, colIRI)
		}
	case vocab.BlockType.Match(typ):
		// NOTE(marius): when receiving Undo:
		//  * for Block - we need to remove the Block's Object from the blocked collection of the Undo's Actor.
		if colIRI := BlockedCollection.Of(toUndo.Actor).GetLink(); p.IsLocalIRI(colIRI) {
			removeFromCols = append(removeFromCols, colIRI)
		}
	case vocab.IgnoreType.Match(typ):
		// NOTE(marius): when receiving Undo:
		// * for Ignore - we need to remove the Ignore's Object from the ignored collection of the Undo's Actor.
		if colIRI := IgnoredCollection.Of(toUndo.Actor).GetLink(); p.IsLocalIRI(colIRI) {
			removeFromCols = append(removeFromCols, colIRI)
		}
	}

	rem = toUndo.Object.GetLink()
	for _, iri := range removeFromCols {
		if err := p.s.RemoveFrom(iri, rem); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return toUndo, errors.Annotatef(errors.Join(errs...), "failed to fully process Undo activity")
	}
	return toUndo, nil
}
