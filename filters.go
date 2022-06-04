package processing

import (
	vocab "github.com/go-ap/activitypub"
)

// Filterable can filter objects by Type and ID
// This should be the minimal interface a filter object should implement for the storage layer
// to work.
// It also allows for an activitypub.IRI to be used as a filter.
type Filterable interface {
	GetLink() vocab.IRI
}

type FilterableItems interface {
	Filterable
	Types() vocab.ActivityVocabularyTypes
	IRIs() vocab.IRIs
}

// FilterableCollection can filter collections
type FilterableCollection interface {
	FilterableObject
	TotalItemsGt() uint
	TotalItemsLt() uint
	TotalItemsEq() uint
	TotalItemsGtE() uint
	TotalItemsLtE() uint
	Contains() vocab.IRIs
}

// FilterableActivity can filter activities
type FilterableActivity interface {
	FilterableObject
	Actors() vocab.IRIs
	Objects() vocab.IRIs
	Targets() vocab.IRIs
}

// FilterableObject can filter objects
type FilterableObject interface {
	FilterableItems
	AttributedTo() vocab.IRIs
	InReplyTo() vocab.IRIs
	MediaTypes() []vocab.MimeType
	Names() []string
	Content() []string
	//PublishedBefore() time.Time
	//PublishedAfter() time.Time
	URLs() vocab.IRIs
	// Audience returns the list of IRIs to check against full Audience targeting for the object
	// It should include all relevant fields: To, CC, BTo, BCC, and Audience
	// ---
	// An element of the Audience is used to get its Inbox end-point and then disseminate the current Activity
	// to it.
	Audience() vocab.IRIs
	// Context returns the list of IRIs to check against an Object's Context property.
	Context() vocab.IRIs
	// Generator returns the list of IRIs to check against an Object's Generator property.
	Generator() vocab.IRIs
}
