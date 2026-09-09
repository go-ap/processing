package processing

import (
	"fmt"

	"git.sr.ht/~mariusor/lw"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// AddActivity indicates that the actor has added the object to the target.
// If the target property is not explicitly specified, the target would need to be determined implicitly by context.
// The origin can be used to identify the context from which the object originated.
func (p *P) AddActivity(add *vocab.Add) (*vocab.Activity, error) {
	if vocab.IsNil(add) {
		return nil, InvalidActivity("nil Add activity")
	}
	if vocab.IsNil(add.Object) {
		return nil, InvalidActivityObject("unable to Add nil object")
	}
	if vocab.IsNil(add.Target) {
		return nil, InvalidActivityObject("unable to Add to nil target")
	}

	targets := make(vocab.IRIs, 0)
	toAdd := make(vocab.ItemCollection, 0)

	// NOTE(marius): we use [vocab.OnItem] here to handle both the cases when the target or the object
	// are composed of multiple items.
	_ = vocab.OnItem(add.Target, func(it vocab.Item) error {
		// NOTE(marius): this behaviour has no atomicity, as we exit at first failure
		// and we don't undo any of the previous adds if target was composed of multiple collections.
		return targets.Append(it.GetLink())
	})
	_ = vocab.OnItem(add.Object, func(object vocab.Item) error {
		return toAdd.Append(object.GetLink())
	})

	errs := make([]error, 0, len(targets))
	for _, target := range targets {
		if err := p.s.AddTo(target, toAdd...); err != nil && !errors.IsConflict(err) {
			p.l.WithContext(lw.Ctx{"target": target, "items": toAdd, "err": err}).Warnf("unable to add object")
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return nil, errors.Annotatef(errors.Join(errs...), "unable to add %s to target collection %s", add.Object, add.Target)
	}
	return add, nil
}

// RemoveActivity indicates that the actor is removing the object from the origin.
// If specified, the origin indicates the context from which the object is being removed.
func (p *P) RemoveActivity(remove *vocab.Remove) (*vocab.Activity, error) {
	if vocab.IsNil(remove) {
		return nil, InvalidActivity("nil Remove activity")
	}
	if vocab.IsNil(remove.Object) {
		return nil, InvalidActivityObject("unable to Remove nil object")
	}
	if vocab.IsNil(remove.Origin) {
		return nil, InvalidActivityObject("unable to Remove from nil origin")
	}

	origins := make(vocab.IRIs, 0)
	toRemove := make(vocab.ItemCollection, 0)

	// NOTE(marius): we use OnItem here to handle both the cases when the target or the object
	//  are composed of multiple items.
	_ = vocab.OnItem(remove.Origin, func(it vocab.Item) error {
		// NOTE(marius): this behaviour has no atomicity, as we exit at first failure
		//  and we don't undo any of the previous removals if origin was composed of multiple collections.
		return origins.Append(it.GetLink())
	})
	_ = vocab.OnItem(remove.Object, func(object vocab.Item) error {
		return toRemove.Append(object.GetLink())
	})

	errs := make([]error, 0, len(origins))
	for _, origin := range origins {
		if err := p.s.RemoveFrom(origin, toRemove...); err != nil {
			p.l.WithContext(lw.Ctx{"from": origin, "items": toRemove, "err": err}).Warnf("unable to remove object")
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return nil, errors.Annotatef(errors.Join(errs...), "unable to remove %s from target collection %s", remove.Object, remove.Target)
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

	origins := make(vocab.IRIs, 0)
	targets := make(vocab.IRIs, 0)
	toMove := make(vocab.ItemCollection, 0)

	// NOTE(marius): we use OnItem here to handle both the cases when the target or the object
	//  are composed of multiple items.
	_ = vocab.OnItem(move.Origin, func(it vocab.Item) error {
		// NOTE(marius): this behaviour has no atomicity, as we exit at first failure
		//  and we don't undo any of the previous removals if origin was composed of multiple collections.
		return origins.Append(it.GetLink())
	})
	// NOTE(marius): we use [vocab.OnItem] here to handle both the cases when the target or the object
	// are composed of multiple items.
	_ = vocab.OnItem(move.Target, func(it vocab.Item) error {
		// NOTE(marius): this behaviour has no atomicity, as we exit at first failure
		// and we don't undo any of the previous adds if target was composed of multiple collections.
		return targets.Append(it.GetLink())
	})
	_ = vocab.OnItem(move.Object, func(object vocab.Item) error {
		return toMove.Append(object.GetLink())
	})

	errs := make([]error, 0, len(targets))
	for _, target := range targets {
		if err := p.s.AddTo(target, toMove...); err != nil && !errors.IsConflict(err) {
			p.l.WithContext(lw.Ctx{"target": target, "items": toMove, "err": err}).Warnf("unable to add object for move operation")
			errs = append(errs, err)
		}
	}
	for _, origin := range origins {
		if err := p.s.RemoveFrom(origin, toMove...); err != nil {
			p.l.WithContext(lw.Ctx{"from": origin, "items": toMove, "err": err}).Warnf("unable to remove object for move operation")
			errs = append(errs, err)
		}
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
	if !object.GetLink().Equal(origin.GetLink()) {
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
