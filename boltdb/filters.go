package boltdb

import as "github.com/go-ap/activitystreams"

// boltFilters
type boltFilters struct {
	id as.IRI
	types []as.ActivityVocabularyType
	iris []as.IRI
}

// ID
func (f boltFilters) ID() as.IRI {
	return f.id
}

// Types
func (f boltFilters) Types() []as.ActivityVocabularyType {
	return f.types
}
// IRIs
func (f boltFilters) IRIs() []as.IRI {
	return f.iris
}
