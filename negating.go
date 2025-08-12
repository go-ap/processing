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
			return errors.NotValidf("The %s activity has a different actor than its object: %s, expected %s", act.Type, act.Actor.GetLink(), objAct.Actor.GetLink())
		}
		if !validUndoActivityTypes.Contains(objAct.Type) {
			return errors.NotValidf("Object Activity has wrong type %s, expected one of %v", objAct.Type, validUndoActivityTypes)
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
func (p P) NegatingActivity(act *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(act.Object) {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if vocab.IsNil(act.Actor) {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	if act.Type != vocab.UndoType {
		return act, errors.NotValidf("Activity has wrong type %s, expected %s", act.Type, vocab.UndoType)
	}
	return p.UndoActivity(act)
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
func (p P) UndoActivity(act *vocab.Activity) (*vocab.Activity, error) {
	var err error

	iri := act.GetLink()
	if len(iri) == 0 {
		iri, _ = p.createIDFn(act.Object, vocab.Outbox.IRI(act.Actor), nil)
	}
	err = vocab.OnActivity(act.Object, func(toUndo *vocab.Activity) error {
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
		case vocab.CreateType:
			_, err = UndoCreateActivity(p.s, toUndo)
		case vocab.DislikeType:
			// TODO(marius): Dislikes should not trigger a removal from Likes/Liked collections
			fallthrough
		case vocab.LikeType:
			_, err = UndoAppreciationActivity(p.s, toUndo)
		case vocab.FollowType:
			fallthrough
		case vocab.BlockType:
			fallthrough
		case vocab.IgnoreType:
			fallthrough
		case vocab.FlagType:
			_, err = p.UndoRelationshipManagementActivity(toUndo)
		case vocab.AnnounceType:
			_, err = p.UndoAnnounceActivity(toUndo)
		}
		return err
	})
	if err != nil {
		return act, err
	}

	err = p.s.Delete(act.Object)
	return act, err
}

// UndoCreateActivity
//
// Removes the side effects of an existing Create activity
// Currently this means only removal of the Create object
func UndoCreateActivity(r Store, act *vocab.Activity) (*vocab.Activity, error) {
	errs := make([]error, 0)
	rem := act.GetLink()

	allRec := act.Recipients()
	removeFromCols := make(vocab.IRIs, 0)
	removeFromCols = append(removeFromCols, vocab.Outbox.IRI(act.Actor))
	for _, rec := range allRec {
		iri := rec.GetLink()
		if iri == vocab.PublicNS {
			continue
		}
		if !vocab.ValidCollectionIRI(iri) {
			// if not a valid collection, then the current iri represents an actor, and we need their inbox
			removeFromCols = append(removeFromCols, vocab.Inbox.IRI(iri))
		}
	}
	for _, iri := range removeFromCols {
		if err := r.RemoveFrom(iri, rem); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return act, errors.Annotatef(errors.Join(errs...), "failed to fully process Undo activity")
	}
	return act, nil
}

// UndoAppreciationActivity
//
// Removes the side effects of an existing Appreciation activity (Like or Dislike)
// Currently this means only removal of the Liked/Disliked object from the actor's `liked` collection and
// removal of the Like/Dislike Activity from the object's `likes` collection
func UndoAppreciationActivity(r Store, act *vocab.Activity) (*vocab.Activity, error) {
	errs := make([]error, 0)
	rem := act.GetLink()

	allRec := act.Recipients()
	removeFromCols := make(vocab.IRIs, 0)
	removeFromCols = append(removeFromCols, vocab.Outbox.IRI(act.Actor))
	removeFromCols = append(removeFromCols, vocab.Liked.IRI(act.Actor))
	removeFromCols = append(removeFromCols, vocab.Likes.IRI(act.Object))
	for _, rec := range allRec {
		iri := rec.GetLink()
		if iri == vocab.PublicNS {
			continue
		}
		if !vocab.ValidCollectionIRI(iri) {
			// if not a valid collection, then the current iri represents an actor, and we need their inbox
			removeFromCols = append(removeFromCols, vocab.Inbox.IRI(iri))
		}
	}
	for _, iri := range removeFromCols {
		if err := r.RemoveFrom(iri, rem); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return act, errors.Annotatef(errors.Join(errs...), "failed to fully process Undo activity")
	}
	return act, nil
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
		if iri := rec.GetLink(); iri == vocab.PublicNS {
			continue
		}
		removeFrom := rec.GetLink()
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
	switch toUndo.GetType() {
	case vocab.FollowType:
		removeFromCols = append(removeFromCols, vocab.Following.Of(toUndo.Actor).GetLink())
	case vocab.BlockType:
		removeFromCols = append(removeFromCols, BlockedCollection.Of(toUndo.Actor).GetLink())
	case vocab.IgnoreType:
		removeFromCols = append(removeFromCols, IgnoredCollection.Of(toUndo.Actor).GetLink())
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
