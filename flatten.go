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
	act.AttributedTo = as.FlattenToIRI(act.AttributedTo)
	act.Audience = FlattenItemCollection(act.Audience)
	act.To = FlattenItemCollection(act.To)
	act.Bto = FlattenItemCollection(act.Bto)
	act.CC = FlattenItemCollection(act.CC)
	act.BCC = FlattenItemCollection(act.BCC)
	return act
}

// FlattenObjectProperties flattens the Object's properties from Object types to IRI
func FlattenPersonProperties(o *auth.Person) *auth.Person {
	o.Parent.Parent = *as.FlattenObjectProperties(&o.Parent.Parent)
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

// FlattenItemCollection flattens an Item Collection to their respective IRIs
func FlattenItemCollection(col as.ItemCollection) as.ItemCollection {
	if col == nil {
		return col
	}
	as.ItemCollectionDeduplication(&col)
	for k, it := range col {
		col[k] = it.GetLink()
	}

	return col
}

// FlattenCollection flattens a Collection's objects to their respective IRIs
func FlattenCollection(col *as.Collection) *as.Collection {
	if col == nil {
		return col
	}
	as.ItemCollectionDeduplication(&col.Items)
	for k, it := range col.Items {
		col.Items[k] = it.GetLink()
	}

	return col
}

// FlattenOrderedCollection flattens an OrderedCollection's objects to their respective IRIs
func FlattenOrderedCollection(col *as.OrderedCollection) *as.OrderedCollection {
	if col == nil {
		return col
	}
	for k, it := range col.OrderedItems {
		col.OrderedItems[k] = it.GetLink()
	}

	return col
}
