package storage

import (
	as "github.com/go-ap/activitystreams"
	"time"
)

// Filterable can filter objects by Type and ObjectID
// This should be the minimal interface a filter object should implement for the storage layer
// to work.
// It also allows for an activitystreams.IRI to be used as a filter.
type Filterable interface {
	GetLink() as.IRI
}

type FilterableItems interface {
	Filterable
	Types() []as.ActivityVocabularyType
	IRIs() []as.IRI
}

// FilterableCollection can filter collections
type FilterableCollection interface {
	FilterableObject
	TotalItemsGt() uint
	TotalItemsLt() uint
	TotalItemsEq() uint
	TotalItemsGtE() uint
	TotalItemsLtE() uint
	Contains() []as.IRI
}

// FilterableActivity can filter activities
type FilterableActivity interface {
	FilterableObject
	Actors() []as.IRI
	Objects() []as.IRI
	Targets() []as.IRI
}

// FilterableObject can filter objects
type FilterableObject interface {
	FilterableItems
	AttributedTo() []as.IRI
	InReplyTo() []as.IRI
	MediaTypes() []as.MimeType
	PublishedBefore() time.Time
	PublishedAfter() time.Time
	URLs() []as.IRI
	// Audience returns the list of IRIs to check against full Audience targeting for the object
	// It should include all relevant fields: To, CC, BTo, BCC, and Audience
	Audience() []as.IRI
}
