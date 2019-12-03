package storage

import (
	pub "github.com/go-ap/activitypub"
)

// Repository
type Repository interface {
	Loader
	Saver
	Close() error
}

// Loader
type Loader interface {
	ActivityLoader
	ActorLoader
	ObjectLoader
	CollectionLoader
}

// ActivityLoader
type ActivityLoader interface {
	LoadActivities(f Filterable) (pub.ItemCollection, uint, error)
}

// ObjectLoader
type ObjectLoader interface {
	LoadObjects(f Filterable) (pub.ItemCollection, uint, error)
}

// ActorLoader
type ActorLoader interface {
	LoadActors(f Filterable) (pub.ItemCollection, uint, error)
}

// CollectionLoader
type CollectionLoader interface {
	LoadCollection(f Filterable) (pub.CollectionInterface, error)
}

// Saver saves
type Saver interface {
	ActivitySaver
	ActorSaver
	ObjectSaver
}

// IDGenerator generates an ObjectID for an ActivityStreams object.
type IDGenerator interface {
	// GenerateID takes an ActivityStreams object, IRI and activity object triplet.
	//  "it" is the object we want to generate the ID for.
	//  "by' represents the Activity that generated the object.
	GenerateID(it pub.Item, by pub.Item) (pub.ObjectID, error)
}

// ActivitySaver saves ActivityStreams activities.
// This interface doesn't have Update and Delete actions pub we want to keep activities immutable
type ActivitySaver interface {
	// SaveActivity saves the incoming Activity object, and returns it together with any properties
	// populated by the method's side effects. (eg, Published property can point to the current time, etc).
	SaveActivity(pub.Item) (pub.Item, error)
}

// ActorSaver saves ActivityStreams actors.
type ActorSaver interface {
	// SaveActor saves the incoming Actor object, and returns it together with any properties
	// populated by the method's side effects. (eg, Published property can point to the current time, etc).
	SaveActor(pub.Item) (pub.Item, error)
	// UpdateActor updates the incoming Actor object, and returns it together with any properties
	// populated by the method's side effects. (eg, Updated property can point to the current time, etc).
	UpdateActor(pub.Item) (pub.Item, error)
	// DeleteActor deletes the incoming Actor object, and returns the resulting Tombstone.
	DeleteActor(pub.Item) (pub.Item, error)
}

// ObjectSaver saves ActivityStreams objects.
type ObjectSaver interface {
	IDGenerator
	// SaveObject saves the incoming ActivityStreams Object, and returns it together with any properties
	// populated by the method's side effects. (eg, Published property can point to the current time, etc).
	SaveObject(pub.Item) (pub.Item, error)
	// UpdateObject updates the incoming ActivityStreams Object, and returns it together with any properties
	// populated by the method's side effects. (eg, Updated property can point to the current time, etc).
	UpdateObject(pub.Item) (pub.Item, error)
	// DeleteObject deletes the incoming ActivityStreams Object, and returns the resulting Tombstone.
	DeleteObject(pub.Item) (pub.Item, error)
}

// CollectionSaver manages collections for ActivityStreams objects.
type CollectionSaver interface {
	// CreateCollection creates the "col" collection.
	CreateCollection(col pub.CollectionInterface) (pub.CollectionInterface, error)
	// AddToCollection adds "it" element to the "col" collection.
	AddToCollection(col pub.IRI, it pub.Item) error
	// RemoveFromCollection removes "it" item from "col" collection
	RemoveFromCollection(col pub.IRI, it pub.Item) error
}
