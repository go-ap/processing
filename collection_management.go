package processing

import (
	"fmt"

	"git.sr.ht/~mariusor/lw"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// AddActivity Indicates that the actor has added the object to the target.
// If the target property is not explicitly specified, the target would need to be determined implicitly by context.
// The origin can be used to identify the context from which the object originated.
func (p *P) AddActivity(add *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(add) {
		return nil, InvalidActivity("nil Add activity")
	}

	if vocab.IsNil(add.Object) {
		return nil, InvalidActivityObject("unable to Add nil object")
	}

	addCtx := lw.Ctx{"to": add.Target.GetLink(), "object": add.Object.GetLink()}
	// NOTE(marius): we use [vocab.OnItem] here to handle both the cases when the target or the object
	// are composed of multiple items.
	err := vocab.OnItem(add.Target, func(target vocab.Item) error {
		// NOTE(marius): this behaviour has no atomicity, as we exit at first failure
		// and we don't undo any of the previous adds if target was composed of multiple collections.
		return vocab.OnItem(add.Object, func(object vocab.Item) error {
			return p.s.AddTo(target.GetLink(), object)
		})
	})
	if err != nil && !errors.IsConflict(err) {
		p.l.WithContext(addCtx, lw.Ctx{"err": err.Error()}).Warnf("unable to add object")
		return nil, errors.Annotatef(err, "unable to add %s to target collection %s", add.Object, add.Target)
	}

	return add, nil
}

// RemoveActivity Indicates that the actor is removing the object from the origin.
// If specified, the origin indicates the context from which the object is being removed.
func (p *P) RemoveActivity(remove *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(remove) {
		return nil, InvalidActivity("nil Remove activity")
	}

	if vocab.IsNil(remove.Object) {
		return nil, InvalidActivityObject("unable to Remove nil object")
	}

	removeCtx := lw.Ctx{"to": remove.Origin.GetLink(), "object": remove.Object.GetLink()}
	// NOTE(marius): we use [vocab.OnItem] here to handle both the cases when the target or the object
	// are composed of multiple items.
	err := vocab.OnItem(remove.Origin, func(origin vocab.Item) error {
		// NOTE(marius): this behaviour has no atomicity, as we exit at first failure
		// and we don't undo any of the previous removals if origin was composed of multiple collections.
		return vocab.OnItem(remove.Object, func(object vocab.Item) error {
			return p.s.RemoveFrom(origin.GetLink(), object)
		})
	})
	if err != nil {
		p.l.WithContext(removeCtx, lw.Ctx{"err": err.Error()}).Warnf("unable to remove object")
		return nil, errors.Annotatef(err, "unable to remove %s from origin collection %s", remove.Object, remove.Target)
	}
	return remove, nil
}

// MoveActivity Indicates that the actor has moved object from origin to target.
// If the origin or target are not specified, either can be determined by context.
func (p *P) MoveActivity(move *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(move) {
		return nil, InvalidActivity("nil Move activity")
	}

	// NOTE(marius): for the special case of the Move activity having its Object being identical to the Origin
	// we consider that to be an Update of that object to the Move activity's Target.
	if vocab.ItemsEqual(move.Object, move.Origin) {
		return p.UpdateObjectID(move)
	}

	// NOTE(marius): we use [vocab.OnItem] here to handle the cases when the target, the origin or the object
	// are composed of multiple items.
	moveCtx := lw.Ctx{"from": move.Origin.GetLink(), "to": move.Target.GetLink(), "object": move.Object.GetLink()}
	err := vocab.OnItem(move.Object, func(object vocab.Item) error {
		err := vocab.OnItem(move.Origin, func(origin vocab.Item) error {
			return p.s.RemoveFrom(origin.GetLink(), object)
		})
		if err != nil {
			return err
		}
		return vocab.OnItem(move.Target, func(target vocab.Item) error {
			return p.s.AddTo(target.GetLink(), object)
		})
	})
	if err != nil && !errors.IsConflict(err) {
		p.l.WithContext(moveCtx, lw.Ctx{"err": err.Error()}).Warnf("unable to move object")
		return nil, errors.Annotatef(err, "unable to move %s from origin collection %s to target collection %s", move.Object, move.Origin, move.Target)
	}

	return move, nil
}

// UpdateObjectID processes a [vocab.MoveType] activity which has identical Object and Origin properties.
//
// This is a corner case of the [vocab.UpdateType] activities where we want to update the object's ID.
//
// ¡This behaviour is not sanctioned by the ActivityPub SWICG, and it's specific to GoActivityPub only!
//
// We documented why we want this in https://todo.sr.ht/~mariusor/go-activitypub/366
func (p *P) UpdateObjectID(move *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(move) {
		return nil, InvalidActivity("Move activity is nil")
	}

	origin := move.Origin
	if vocab.IsNil(origin) {
		return nil, ValidationError(fmt.Sprintf("Origin is not valid: is nil"))
	}

	object, err := p.DereferenceItem(move.Object)
	if err != nil {
		return nil, ValidationError(fmt.Sprintf("Move Object wasn't available in local storage"))
	}
	if !object.GetLink().Equal(origin.GetLink(), true) {
		return nil, ValidationError(fmt.Sprintf("Object and Origin of Move activity should not be different"))
	}
	if !vocab.IsObject(move.Target) {
		return nil, ValidationError(fmt.Sprintf("Target object %T of Move activity is invalid", move.Target))
	}

	if object, err = vocab.CopyUnsafeItemProperties(object, move.Target); err != nil {
		return nil, errors.Newf("Unable to copy Target to Object for special Move activity")
	}

	if object, err = p.s.Save(object); err != nil {
		return nil, err
	}

	return move, nil
}
