package processing

import (
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/filters"
)

type Store interface {
	ReadStore
	WriteStore
	CollectionStore
}

type ReadStore interface {
	// Load returns an Item or an ItemCollection from an IRI
	// after filtering it through the FilterFn list of filtering functions. Eg ANY()
	Load(vocab.IRI, ...filters.Check) (vocab.Item, error)
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
	Create(vocab.CollectionInterface) (vocab.CollectionInterface, error)
	// AddTo adds "it" element to the "col" collection.
	AddTo(vocab.IRI, ...vocab.Item) error
	// RemoveFrom removes "it" item from "col" collection
	RemoveFrom(vocab.IRI, ...vocab.Item) error
}
