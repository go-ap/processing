package processing

import (
	vocab "github.com/go-ap/activitypub"
)

type Store interface {
	ReadStore
	WriteStore
}

// ReadStore
type ReadStore interface {
	// Load returns an Item or an ItemCollection from an IRI
	Load(vocab.IRI) (vocab.Item, error)
}

// WriteStore saves ActivityStreams objects.
type WriteStore interface {
	// Save saves the incoming ActivityStreams Object, and returns it together with any properties
	// populated by the method's side effects. (eg, Published property can point to the current time, etc.).
	Save(vocab.Item) (vocab.Item, error)
	// Delete deletes completely from storage the ActivityStreams Object
	Delete(vocab.Item) error
}

// CollectionStore allows operations on ActivityStreams collections
type CollectionStore interface {
	// Create creates the "col" collection.
	Create(col vocab.CollectionInterface) (vocab.CollectionInterface, error)
	// AddTo adds "it" element to the "col" collection.
	AddTo(col vocab.IRI, it vocab.Item) error
	// RemoveFrom removes "it" item from "col" collection
	RemoveFrom(col vocab.IRI, it vocab.Item) error
}
