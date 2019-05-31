package boltdb

import (
	as "github.com/go-ap/activitystreams"
	"github.com/go-ap/errors"
	s "github.com/go-ap/storage"
)

type boltDB struct{}

func (b *boltDB) Load(f s.Filterable) (as.ItemCollection, int, error) {
	return nil, 0, errors.NotImplementedf("BoltDB Load not implemented")
}
func (b *boltDB) LoadActivities(f s.Filterable) (as.ItemCollection, int, error) {
	return nil, 0, errors.NotImplementedf("BoltDB LoadActivities not implemented")
}
func (b *boltDB) LoadObjects(f s.Filterable) (as.ItemCollection, int, error) {
	return nil, 0, errors.NotImplementedf("BoltDB LoadObjects not implemented")
}
func (b *boltDB) LoadCollection(f s.Filterable) (as.CollectionInterface, int, error) {
	return nil, 0, errors.NotImplementedf("BoltDB LoadCollection not implemented")
}
func (b *boltDB) SaveActivity(as.Item) (as.Item, error) {
	return nil, errors.NotImplementedf("BoltDB SaveActivity not implemented")
}
func (b *boltDB) SaveActor(as.Item) (as.Item, error) {
	return nil, errors.NotImplementedf("BoltDB SaveActor not implemented")
}
func (b *boltDB) SaveObject(as.Item) (as.Item, error) {
	return nil, errors.NotImplementedf("BoltDB SaveObject not implemented")
}
