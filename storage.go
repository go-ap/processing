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
	ActorLoader
	ObjectLoader
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

// Saver
type Saver interface {
	ActivitySaver
	ActorSaver
	ObjectSaver
}

// ActivitySaver
// This interface doesn't have Update and Delete actions as we want to keep activities immutable
type ActivitySaver interface {
	SaveActivity(as.Item) (as.Item, error)
}

// ActorSaver
type ActorSaver interface {
	SaveActor(as.Item) (as.Item, error)
	UpdateActor(as.Item) (as.Item, error)
	DeleteActor(as.Item) (as.Item, error)
}

// ObjectSaver
type ObjectSaver interface {
	SaveObject(as.Item) (as.Item, error)
	UpdateObject(as.Item) (as.Item, error)
	DeleteObject(as.Item) (as.Item, error)
}
