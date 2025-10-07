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

	// NOTE(marius): we use [vocab.OnItem] here to handle both the cases when the target or the object
	// are composed of multiple items.
	err := vocab.OnItem(add.Target, func(target vocab.Item) error {
		// NOTE(marius): this behaviour has no atomicity, as we exit at first failure
		// and we don't undo any of the previous adds if target was composed of multiple collections.
		return vocab.OnItem(add.Object, func(object vocab.Item) error {
			if err := p.s.AddTo(target.GetLink(), object); err != nil {
				p.l.WithContext(lw.Ctx{"err": err.Error(), "col": target.GetLink(), "it": object.GetLink()}).Warnf("unable to add object to collection")
				if errors.IsConflict(err) {
					err = nil
				}
				return err
			}
			return nil
		})
	})

	if err != nil {
		return nil, errors.Annotatef(err, "unable to add %s to collection %s", add.Object, add.Target)
	}
	return add, nil
}

// RemoveActivity Indicates that the actor is removing the object.
// If specified, the origin indicates the context from which the object is being removed.
func (p *P) RemoveActivity(remove *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(remove) {
		return nil, InvalidActivity("nil Remove activity")
	}

	// NOTE(marius): we use [vocab.OnItem] here to handle both the cases when the target or the object
	// are composed of multiple items.
	err := vocab.OnItem(remove.Target, func(target vocab.Item) error {
		// NOTE(marius): this behaviour has no atomicity, as we exit at first failure
		// and we don't undo any of the previous removals if target was composed of multiple collections.
		return vocab.OnItem(remove.Object, func(object vocab.Item) error {
			if err := p.s.RemoveFrom(target.GetLink(), object); err != nil {
				p.l.WithContext(lw.Ctx{"err": err.Error(), "col": target.GetLink(), "it": object.GetLink()}).Warnf("unable to remove object from collection")
				if errors.IsConflict(err) {
					err = nil
				}
				return err
			}
			return nil
		})
	})
	if err != nil {
		return nil, errors.Annotatef(err, "unable to remove %s from collection %s", remove.Object, remove.Target)
	}
	return remove, nil
}

// MoveActivity Indicates that the actor has moved object from origin to target.
// If the origin or target are not specified, either can be determined by context.
func (p *P) MoveActivity(move *vocab.Activity) (*vocab.Activity, error) {
	// NOTE(marius): for the special case of the Move activity having its Object being identical to the Origin
	// we consider that to be an Update of that object to the Move activity's Target.
	if vocab.ItemsEqual(move.Object, move.Origin) {
		return p.UpdateObjectID(move)
	}

	originCol, err := p.s.Load(move.Origin.GetLink())
	if err != nil {
		return nil, err
	}
	var object vocab.Item
	err = vocab.OnCollectionIntf(originCol, func(col vocab.CollectionInterface) error {
		for _, it := range col.Collection() {
			if it.GetLink().Equals(move.Object.GetLink(), true) {
				object = it
				break
			}
		}
		col.Remove(object)
		return nil
	})
	if err != nil {
		return nil, err
	}

	targetCol, err := p.s.Load(move.Target.GetLink())
	if err != nil {
		return nil, err
	}

	target, ok := targetCol.(vocab.CollectionInterface)
	if !ok {
		return nil, InvalidTarget("target is not a valid collection")
	}
	if err = target.Append(object); err != nil {
		return nil, err
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

	object, err := p.s.Load(move.Object.GetLink())
	if err != nil {
		return nil, ValidationError(fmt.Sprintf("Move Object wasn't available in local storage"))
	}
	if !object.GetLink().Equals(origin.GetLink(), true) {
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
