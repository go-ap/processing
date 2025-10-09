package processing

import (
	"sync"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/filters"
)

type mockStore struct {
	*sync.Map
}

func (m mockStore) Load(iri vocab.IRI, _ ...filters.Check) (vocab.Item, error) {
	blob, ok := m.Map.Load(iri)
	if !ok {
		return nil, errors.NotFoundf("%s not found in mock storage", iri)
	}
	it, ok := blob.(vocab.Item)
	if !ok {
		return nil, errors.Newf("object of invalid type %T found for %s", blob, iri)
	}
	return it, nil
}

func (m mockStore) Save(item vocab.Item) (vocab.Item, error) {
	if vocab.IsNil(item) {
		return nil, errors.Newf("unable to save nil item")
	}
	iri := item.GetLink()
	m.Map.Store(iri, item)
	return item, nil
}

func (m mockStore) Delete(item vocab.Item) error {
	if vocab.IsNil(item) {
		return errors.Newf("unable to delete nil item")
	}
	m.Map.Delete(item.GetLink())
	return nil
}

func (m mockStore) Create(col vocab.CollectionInterface) (vocab.CollectionInterface, error) {
	if vocab.IsNil(col) {
		return nil, errors.Newf("unable to create nil collection")
	}
	it, err := m.Save(col)
	if err != nil {
		return nil, err
	}
	cc, ok := it.(vocab.CollectionInterface)
	if !ok {
		return nil, errors.Newf("object of invalid type %T saved for %s", it, col.GetLink())
	}
	return cc, nil
}

func (m mockStore) AddTo(col vocab.IRI, it vocab.Item) error {
	if vocab.IsNil(col) {
		return errors.Newf("unable to add to nil collection")
	}
	maybeCol, err := m.Load(col.GetLink())
	if err != nil {
		return err
	}
	cc, ok := maybeCol.(vocab.CollectionInterface)
	if !ok {
		return errors.Newf("object of invalid type %T loaded for %s", maybeCol, col.GetLink())
	}
	if err = cc.Append(it); err != nil {
		return err
	}
	return nil
}

func (m mockStore) RemoveFrom(col vocab.IRI, it vocab.Item) error {
	if vocab.IsNil(col) {
		return errors.Newf("unable to remove from nil collection")
	}
	maybeCol, err := m.Load(col.GetLink())
	if err != nil {
		return err
	}
	cc, ok := maybeCol.(vocab.CollectionInterface)
	if !ok {
		return errors.Newf("object of invalid type %T loaded for %s", it, col.GetLink())
	}
	cc.Remove(it)
	return nil
}

var _ Store = &mockStore{}
