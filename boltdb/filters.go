package boltdb

import as "github.com/go-ap/activitystreams"

// boltFilters
type boltFilters struct {
	types []as.ActivityVocabularyType
	iris []as.IRI
}

// Types
func (f boltFilters) Types() []as.ActivityVocabularyType {
	return f.types
}
// IRIs
func (f boltFilters) IRIs() []as.IRI {
	return f.iris
}
