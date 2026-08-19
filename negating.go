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
	if toUndo.GetID() == "" {
		return InvalidActivity("empty IRI")
	}
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
		if err = p.s.Delete(toUndo.GetLink()); err != nil && errors.IsNotFound(err) {
			err = nil
		}
	}
	return err
}
