package processing

import (
	as "github.com/go-ap/activitystreams"
	"github.com/go-ap/auth"
)

// FlattenActivityProperties flattens the Activity's properties from Object type to IRI
func FlattenActivityProperties(act *as.Activity) *as.Activity {
	act.Object = as.FlattenToIRI(act.Object)
	act.Actor = as.FlattenToIRI(act.Actor)
	act.Target = as.FlattenToIRI(act.Target)
	act.Result = as.FlattenToIRI(act.Result)
	act.Origin = as.FlattenToIRI(act.Origin)
	act.Result = as.FlattenToIRI(act.Result)
	act.Instrument = as.FlattenToIRI(act.Instrument)
	return act
}

// FlattenObjectProperties flattens the Object's properties from Object types to IRI
func FlattenPersonProperties(o *auth.Person) *auth.Person {
	o.Parent = *as.FlattenObjectProperties(&o.Parent)
	return o
}

// FlattenProperties flattens the Item's properties from Object types to IRI
func FlattenProperties(it as.Item) as.Item {
	if as.ActivityTypes.Contains(it.GetType()) {
		a, err := as.ToActivity(it)
		if err == nil {
			return FlattenActivityProperties(a)
		}
	}
	if as.ActorTypes.Contains(it.GetType()) {
		ob, err := auth.ToPerson(it)
		if err == nil {
			return FlattenPersonProperties(ob)
		}
	}
	if it.GetType() == as.TombstoneType {
		t, err := as.ToTombstone(it)
		if err == nil {
			t.Parent = *as.FlattenObjectProperties(&t.Parent)
			return t
		}
	}
	return as.FlattenProperties(it)
}
