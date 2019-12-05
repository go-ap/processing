package storage

import (
	pub "github.com/go-ap/activitypub"
)

// Filterable can filter objects by Type and ID
// This should be the minimal interface a filter object shoul,ad implement for the storage layer
// to work.
// It also allows for an activitystreams.IRI to be used pub a filter.
type Filterable interface {
	GetLink() pub.IRI
}

type FilterableItems interface {
	Filterable
	Types() pub.ActivityVocabularyTypes
	IRIs() pub.IRIs
}

// FilterableCollection can filter collections
type FilterableCollection interface {
	FilterableObject
	TotalItemsGt() uint
	TotalItemsLt() uint
	TotalItemsEq() uint
	TotalItemsGtE() uint
	TotalItemsLtE() uint
	Contains() pub.IRIs
}

// FilterableActivity can filter activities
type FilterableActivity interface {
	FilterableObject
	Actors() pub.IRIs
	Objects() pub.IRIs
	Targets() pub.IRIs
}

// FilterableObject can filter objects
type FilterableObject interface {
	FilterableItems
	AttributedTo() pub.IRIs
	InReplyTo() pub.IRIs
	MediaTypes() []pub.MimeType
	Names() []string
	Content() []string
	//PublishedBefore() time.Time
	//PublishedAfter() time.Time
	URLs() pub.IRIs
	// Audience returns the list of IRIs to check against full Audience targeting for the object
	// It should include all relevant fields: To, CC, BTo, BCC, and Audience
	// ---
	// An element of the Audience is used to get its Inbox end-point and then disseminate the current Activity
	// to it.
	Audience() pub.IRIs
	// Context returns the list of IRIs to check against an Object's Context.
	Context() pub.IRIs
}
