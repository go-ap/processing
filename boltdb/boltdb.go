package boltdb

import (
	"bytes"
	"fmt"
	"github.com/boltdb/bolt"
	as "github.com/go-ap/activitystreams"
	"github.com/go-ap/errors"
	"github.com/go-ap/jsonld"
	s "github.com/go-ap/storage"
	"strings"
)

type boltDB struct {
	d     *bolt.DB
	root  []byte
	logFn loggerFn
	errFn loggerFn
}

type loggerFn func(string, ...interface{})

const (
	bucketActors      = "actors"
	bucketActivities  = "activities"
	bucketObjects     = "objects"
	bucketCollections = "collections"
)

// Config
type Config struct {
	Path       string
	BucketName string
	LogFn      loggerFn
	ErrFn      loggerFn
}

// New returns a new boltDB repository
func New(c Config) (*boltDB, error) {
	db, err := bolt.Open(c.Path, 0600, nil)
	if err != nil {
		return nil, errors.Annotatef(err, "could not open db")
	}
	rootBucket := []byte(c.BucketName)
	err = db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket(rootBucket)
		if root == nil || !root.Writable() {
			return errors.NotFoundf("root bucket not found or is not writeable")
		}
		return nil
	})
	if err != nil {
		return nil, errors.Annotatef(err, "could not set up buckets")
	}

	b := boltDB{
		d:     db,
		root:  rootBucket,
		logFn: func(string, ...interface{}) {},
		errFn: func(string, ...interface{}) {},
	}
	if c.ErrFn != nil {
		b.errFn = c.ErrFn
	}
	if c.LogFn != nil {
		b.logFn = c.LogFn
	}
	return &b, nil
}

func loadFromBucket(db *bolt.DB, root, bucket []byte, f s.Filterable) (as.ItemCollection, uint, error) {
	col := make(as.ItemCollection, 0)

	err := db.View(func(tx *bolt.Tx) error {
		root := tx.Bucket(root)
		if root == nil {
			return errors.Errorf("Invalid bucket %s", root)
		}
		// Assume bucket exists and has keys
		b := root.Bucket(bucket)
		if b == nil {
			return errors.Errorf("Invalid bucket %s.%s", root, bucket)
		}

		c := b.Cursor()
		if c == nil {
			return errors.Errorf("Invalid bucket cursor %s.%s", root, bucket)
		}
		for _, iri := range f.IRIs() {
			prefix := []byte(iri.GetLink())
			for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
				if it, err := as.UnmarshalJSON(v); err == nil {
					col = append(col, it)
				}
			}
		}

		return nil
	})
	
	return col, uint(len(col)), err
}

// Load
func (b *boltDB) Load(f s.Filterable) (as.ItemCollection, uint, error) {
	return nil, 0, errors.NotImplementedf("BoltDB Load not implemented")
}

// LoadActivities
func (b *boltDB) LoadActivities(f s.Filterable) (as.ItemCollection, uint, error) {
	return loadFromBucket(b.d, b.root, []byte(bucketActivities), f)
}

// LoadObjects
func (b *boltDB) LoadObjects(f s.Filterable) (as.ItemCollection, uint, error) {
	return loadFromBucket(b.d, b.root, []byte(bucketObjects), f)
}

// LoadActors
func (b *boltDB) LoadActors(f s.Filterable) (as.ItemCollection, uint, error) {
	return loadFromBucket(b.d, b.root, []byte(bucketActors), f)
}

// LoadCollection
func (b *boltDB) LoadCollection(f s.Filterable) (as.CollectionInterface, error) {
	var ret as.CollectionInterface

	err := b.d.View(func(tx *bolt.Tx) error {
		root := tx.Bucket(b.root)
		if root == nil {
			return errors.Errorf("Invalid bucket %s", root)
		}
		bucket := []byte(bucketCollections)
		// Assume bucket exists and has keys
		colBkt := root.Bucket(bucket)
		if colBkt == nil {
			return errors.Errorf("Invalid bucket %s.%s", root, bucket)
		}

		c := colBkt.Cursor()
		if c == nil {
			return errors.Errorf("Invalid bucket cursor %s.%s", root, bucket)
		}
		for _, iri := range f.IRIs() {
			blob := colBkt.Get([]byte(iri.GetLink()))
			var IRIs []as.IRI
			if err := jsonld.Unmarshal(blob, &IRIs); err == nil {
				col := &as.OrderedCollection{}
				col.ID = as.ObjectID(iri)
				col.Type = as.OrderedCollectionType
				ret = col
				f := boltFilters{
					iris: IRIs,
				}
				var searchActors, searchObjects, searchActivities bool
				for _, it := range IRIs {
					if strings.Contains(it.String(), bucketActivities) {
						searchActivities = true
					}
					if strings.Contains(it.String(), bucketActors) {
						searchActors = true
					}
					if strings.Contains(it.String(), bucketObjects) {
						searchObjects = true
					}
					break
				}
				if searchActivities {
					col.OrderedItems, col.TotalItems, err = b.LoadActivities(f)
				}
				if searchActors {
					col.OrderedItems, col.TotalItems, err = b.LoadActors(f)
				}
				if searchObjects {
					col.OrderedItems, col.TotalItems, err = b.LoadObjects(f)
				}
				ret = col
			}
		}

		return nil
	})

	return ret, err
}

func save(db *bolt.DB, rootBkt, bucket []byte, it as.Item) (as.Item, error) {
	entryBytes, err := jsonld.Marshal(it)
	if err != nil {
		return it, errors.Annotatef(err, "could not marshal activity")
	}
	err = db.Update(func(tx *bolt.Tx) error {
		root := tx.Bucket(rootBkt)
		if root == nil {
			return errors.Errorf("Invalid bucket %s", rootBkt)
		}
		if !root.Writable() {
			return errors.Errorf("Non writeable bucket %s", rootBkt)
		}
		// Assume bucket exists and has keys
		b := root.Bucket(bucket)
		if b == nil {
			return errors.Errorf("Invalid bucket %s.%s", rootBkt, bucket)
		}
		if !b.Writable() {
			return errors.Errorf("Non writeable bucket %s %s", rootBkt, bucket)
		}
		err := b.Put([]byte(it.GetLink()), entryBytes)
		if err != nil {
			return fmt.Errorf("could not insert entry: %v", err)
		}

		return nil
	})

	return it, err
}

// SaveActivity
func (b *boltDB) SaveActivity(it as.Item) (as.Item, error) {
	var err error
	if it, err = save(b.d, b.root, []byte(bucketActivities), it); err == nil {
		b.logFn("Added new activity: %s", it.GetLink())
	}
	return it, err
}

// SaveActor
func (b *boltDB) SaveActor(it as.Item) (as.Item, error) {
	var err error
	if it, err = save(b.d, b.root, []byte(bucketActors), it); err == nil {
		b.logFn("Added new activity: %s", it.GetLink())
	}
	return it, err
}

// SaveObject
func (b *boltDB) SaveObject(it as.Item) (as.Item, error) {
	var err error
	if it, err = save(b.d, b.root, []byte(bucketObjects), it); err == nil {
		b.logFn("Added new activity: %s", it.GetLink())
	}
	return it, err
}
