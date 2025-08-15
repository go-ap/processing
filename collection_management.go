package processing

import (
	"fmt"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// ObjectMoveActivity processes a [vocab.MoveType] activity which has identical Object and Origin properties.
//
// This is a corner case of the [vocab.UpdateType] activities where we want to update the object's ID.
//
// See this ticket for details: https://todo.sr.ht/~mariusor/go-activitypub/366
func (p *P) ObjectMoveActivity(move *vocab.Activity) (*vocab.Activity, error) {
	object := move.Object
	if vocab.IsNil(object) {
		return move, InvalidActivityObject("is nil")
	}
	origin := move.Origin
	if vocab.IsNil(origin) {
		return move, ValidationError(fmt.Sprintf("Origin is not valid: is nil"))
	}
	if !object.GetLink().Equals(origin.GetLink(), true) {
		return move, ValidationError(fmt.Sprintf("Object and Origin of Move activity should not be different"))
	}

	return move, errors.NotImplementedf("Move of object is still not finalized")
}
