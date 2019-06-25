package storage

import (
	as "github.com/go-ap/activitystreams"
)

// Repository
type Repository interface {
	Loader
	Saver
}

// Loader
type Loader interface {
	ActivityLoader
	ObjectLoader
	CollectionLoader
}

// ActivityLoader
type ActivityLoader interface {
	LoadActivities(f Filterable) (as.ItemCollection, uint, error)
}

// ObjectLoader
type ObjectLoader interface {
	LoadObjects(f Filterable) (as.ItemCollection, uint, error)
}

// ActorLoader
type ActorLoader interface {
	LoadActors(f Filterable) (as.ItemCollection, uint, error)
}

// CollectionLoader
type CollectionLoader interface {
	LoadCollection(f Filterable) (as.CollectionInterface, error)
}

// Saver saves
type Saver interface {
	ActivitySaver
	ObjectSaver
}

// IDGenerator generates an ObjectID for an ActivityStreams object.
type IDGenerator interface {
	// GenerateID takes an ActivityStreams object, IRI and activity object triplet.
	//  The Object is the object we want to generate the ID for.
	//  The IRI is the IRI of the collection that the Object will be a part of.
	//  The Activity is the activity that generated the object.
	GenerateID(it as.Item, partOf as.IRI, by as.Item) (as.ObjectID, error)
}

// ActivitySaver saves ActivityStreams activities.
// This interface doesn't have Update and Delete actions as we want to keep activities immutable
type ActivitySaver interface {
	// SaveActivity saves the incoming Activity object, and returns it together with any properties
	// populated by the method's side effects. (eg, Published property can point to the current time, etc).
	SaveActivity(as.Item) (as.Item, error)
}

// ActorSaver saves ActivityStreams actors.
type ActorSaver interface {
	// SaveActor saves the incoming Actor object, and returns it together with any properties
	// populated by the method's side effects. (eg, Published property can point to the current time, etc).
	SaveActor(as.Item) (as.Item, error)
	// UpdateActor updates the incoming Actor object, and returns it together with any properties
	// populated by the method's side effects. (eg, Updated property can point to the current time, etc).
	UpdateActor(as.Item) (as.Item, error)
	// DeleteActor deletes the incoming Actor object, and returns the resulting Tombstone.
	DeleteActor(as.Item) (as.Item, error)
}

// ObjectSaver saves ActivityStreams objects.
type ObjectSaver interface {
	IDGenerator
	// SaveObject saves the incoming ActivityStreams Object, and returns it together with any properties
	// populated by the method's side effects. (eg, Published property can point to the current time, etc).
	SaveObject(as.Item) (as.Item, error)
	// UpdateObject updates the incoming ActivityStreams Object, and returns it together with any properties
	// populated by the method's side effects. (eg, Updated property can point to the current time, etc).
	UpdateObject(as.Item) (as.Item, error)
	// DeleteObject deletes the incoming ActivityStreams Object, and returns the resulting Tombstone.
	DeleteObject(as.Item) (as.Item, error)
}

// CollectionSaver manages collections for ActivityStreams objects.
type CollectionSaver interface {
	// CreateCollection creates the "col" collection.
	CreateCollection(col as.CollectionInterface) (as.CollectionInterface, error)
	// AddToCollection adds "it" element to the "col" collection.
	AddToCollection(col as.IRI, it as.Item) error
}
