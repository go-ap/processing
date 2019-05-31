package boltdb

import (
	"errors"
	as "github.com/go-ap/activitystreams"
	s "github.com/go-ap/storage"
)

type boltDB struct{}

func (b *boltDB) Load(f s.Filterable) (as.ItemCollection, int, error) {
	return nil, 0, errors.New("not implemented")
}
func (b *boltDB) LoadActivities(f s.Filterable) (as.ItemCollection, int, error) {
	return nil, 0, errors.New("not implemented")
}
func (b *boltDB) LoadObjects(f s.Filterable) (as.ItemCollection, int, error) {
	return nil, 0, errors.New("not implemented")
}
func (b *boltDB) LoadCollection(f s.Filterable) (as.CollectionInterface, int, error) {
	return nil, 0, errors.New("not implemented")
}
func (b *boltDB) SaveActivity(as.Item) (as.Item, error) {
	return nil, errors.New("not implemented")
}
func (b *boltDB) SaveActor(as.Item) (as.Item, error) {
	return nil, errors.New("not implemented")
}
func (b *boltDB) SaveObject(as.Item) (as.Item, error) {
	return nil, errors.New("not implemented")
}
