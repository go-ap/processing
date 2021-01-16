package processing

import (
	pub "github.com/go-ap/activitypub"
)

// FlattenActivityProperties flattens the Activity's properties from Object type to IRI
func FlattenActivityProperties(act *pub.Activity) *pub.Activity {
	pub.OnIntransitiveActivity(act, func(in *pub.IntransitiveActivity) error {
		FlattenIntransitiveActivityProperties(in)
		return nil
	})
	act.Object = pub.FlattenToIRI(act.Object)
	return act
}

// FlattenIntransitiveActivityProperties flattens the Activity's properties from Object type to IRI
func FlattenIntransitiveActivityProperties(act *pub.IntransitiveActivity) *pub.IntransitiveActivity {
	act.Actor = pub.FlattenToIRI(act.Actor)
	act.Target = pub.FlattenToIRI(act.Target)
	act.Result = pub.FlattenToIRI(act.Result)
	act.Origin = pub.FlattenToIRI(act.Origin)
	act.Result = pub.FlattenToIRI(act.Result)
	act.Instrument = pub.FlattenToIRI(act.Instrument)
	pub.OnObject(act, func(o *pub.Object) error {
		o = FlattenObjectProperties(o)
		return nil
	})
	return act
}

// FlattenItemCollection flattens an Item Collection to their respective IRIs
func FlattenItemCollection(col pub.ItemCollection) pub.ItemCollection {
	if col == nil {
		return col
	}
	for k, it := range pub.ItemCollectionDeduplication(&col) {
		if iri := it.GetLink(); iri != "" {
			col[k] = iri
		}
	}
	return col
}

// FlattenCollection flattens a Collection's objects to their respective IRIs
func FlattenCollection(col *pub.Collection) *pub.Collection {
	if col == nil {
		return col
	}
	for k, it := range pub.ItemCollectionDeduplication(&col.Items) {
		col.Items[k] = it.GetLink()
	}

	return col
}

// FlattenOrderedCollection flattens an OrderedCollection's objects to their respective IRIs
func FlattenOrderedCollection(col *pub.OrderedCollection) *pub.OrderedCollection {
	if col == nil {
		return col
	}
	for k, it := range pub.ItemCollectionDeduplication(&col.OrderedItems) {
		col.OrderedItems[k] = it.GetLink()
	}

	return col
}

// FlattenActorProperties flattens the Actor's properties from Object types to IRI
func FlattenActorProperties(a *pub.Actor) *pub.Actor {
	pub.OnObject(a, func(o *pub.Object) error {
		o = FlattenObjectProperties(o)
		return nil
	})
	return a
}

// FlattenObjectProperties flattens the Object's properties from Object types to IRI
func FlattenObjectProperties(o *pub.Object) *pub.Object {
	o.Replies = Flatten(o.Replies)
	o.Shares = Flatten(o.Shares)
	o.Likes = Flatten(o.Likes)
	o.AttributedTo = Flatten(o.AttributedTo)
	o.To = FlattenItemCollection(o.To)
	o.Bto = FlattenItemCollection(o.Bto)
	o.CC = FlattenItemCollection(o.CC)
	o.BCC = FlattenItemCollection(o.BCC)
	o.Audience = FlattenItemCollection(o.Audience)
	o.Tag = FlattenItemCollection(o.Tag)
	return o
}

// FlattenProperties flattens the Item's properties from Object types to IRI
func FlattenProperties(it pub.Item) pub.Item {
	if pub.ActivityTypes.Contains(it.GetType()) {
		pub.OnActivity(it, func(a *pub.Activity) error {
			a = FlattenActivityProperties(a)
			return nil
		})
	}
	if pub.ActorTypes.Contains(it.GetType()) { 
		pub.OnActor(it, func(a *pub.Actor) error {
			a = FlattenActorProperties(a)
			return nil
		})
	}
	if pub.ObjectTypes.Contains(it.GetType()) {
		pub.OnObject(it, func(o *pub.Object) error {
			o = FlattenObjectProperties(o)
			return nil
		})
	}
	return it
}

// Flatten checks if Item can be flatten to an IRI or array of IRIs and returns it if so
func Flatten(it pub.Item) pub.Item {
	if it == nil {
		return nil
	}
	if it.IsCollection() {
		if c, ok := it.(pub.CollectionInterface); ok {
			it = FlattenItemCollection(c.Collection())
		}
	}
	if it != nil && len(it.GetLink()) > 0 {
		return it.GetLink()
	}
	return it
}

