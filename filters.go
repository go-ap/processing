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
	Types() as.ActivityVocabularyTypes
	IRIs() as.IRIs
}

// FilterableCollection can filter collections
type FilterableCollection interface {
	FilterableObject
	TotalItemsGt() uint
	TotalItemsLt() uint
	TotalItemsEq() uint
	TotalItemsGtE() uint
	TotalItemsLtE() uint
	Contains() as.IRIs
}

// FilterableActivity can filter activities
type FilterableActivity interface {
	FilterableObject
	Actors() as.IRIs
	Objects() as.IRIs
	Targets() as.IRIs
}

// FilterableObject can filter objects
type FilterableObject interface {
	FilterableItems
	AttributedTo() as.IRIs
	InReplyTo() as.IRIs
	MediaTypes() []as.MimeType
	PublishedBefore() time.Time
	PublishedAfter() time.Time
	URLs() as.IRIs
	// Audience returns the list of IRIs to check against full Audience targeting for the object
	// It should include all relevant fields: To, CC, BTo, BCC, and Audience
	// ---
	// An element of the Audience is used to get its Inbox end-point and then disseminate the current Activity
	// to it.
	Audience() as.IRIs
}
