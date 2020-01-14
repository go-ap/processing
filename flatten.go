package processing

import (
	pub "github.com/go-ap/activitypub"
)

// FlattenActivityProperties flattens the Activity's properties from Object type to IRI
func FlattenActivityProperties(act *pub.Activity) *pub.Activity {
	//act.Object = pub.FlattenToIRI(act.Object)
	act.Actor = pub.FlattenToIRI(act.Actor)
	act.Target = pub.FlattenToIRI(act.Target)
	act.Result = pub.FlattenToIRI(act.Result)
	act.Origin = pub.FlattenToIRI(act.Origin)
	act.Result = pub.FlattenToIRI(act.Result)
	act.Instrument = pub.FlattenToIRI(act.Instrument)
	act.AttributedTo = pub.FlattenToIRI(act.AttributedTo)
	act.Audience = FlattenItemCollection(act.Audience)
	act.To = FlattenItemCollection(act.To)
	act.Bto = FlattenItemCollection(act.Bto)
	act.CC = FlattenItemCollection(act.CC)
	act.BCC = FlattenItemCollection(act.BCC)
	return act
}

// FlattenItemCollection flattens an Item Collection to their respective IRIs
func FlattenItemCollection(col pub.ItemCollection) pub.ItemCollection {
	if col == nil {
		return col
	}
	pub.ItemCollectionDeduplication(&col)
	for k, it := range col {
		col[k] = it.GetLink()
	}

	return col
}

// FlattenCollection flattens a Collection's objects to their respective IRIs
func FlattenCollection(col *pub.Collection) *pub.Collection {
	if col == nil {
		return col
	}
	pub.ItemCollectionDeduplication(&col.Items)
	for k, it := range col.Items {
		col.Items[k] = it.GetLink()
	}

	return col
}

// FlattenOrderedCollection flattens an OrderedCollection's objects to their respective IRIs
func FlattenOrderedCollection(col *pub.OrderedCollection) *pub.OrderedCollection {
	if col == nil {
		return col
	}
	for k, it := range col.OrderedItems {
		col.OrderedItems[k] = it.GetLink()
	}

	return col
}
