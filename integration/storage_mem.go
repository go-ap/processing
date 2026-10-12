package integration

import (
	"sync"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/filters"
	"github.com/go-ap/processing"
)

type memStorageError string

func (e memStorageError) Error() string {
	return string(e)
}

type memStorage struct {
	*sync.Map
}

var (
	orderedCollectionTypes = vocab.ActivityVocabularyTypes{
		vocab.OrderedCollectionType,
		vocab.OrderedCollectionPageType,
	}
	collectionTypes = vocab.ActivityVocabularyTypes{
		vocab.CollectionType,
		vocab.CollectionPageType,
	}

	allCollectionTypes = append(collectionTypes, orderedCollectionTypes...)
)

func (ms *memStorage) Load(i vocab.IRI, checks ...filters.Check) (vocab.Item, error) {
	raw, ok := ms.Map.Load(i)
	if !ok {
		return nil, errors.NotFoundf("unable to find %s", i)
	}
	it, ok := raw.(vocab.Item)
	if !ok {
		return nil, errors.Newf("invalid item type in storage %T", raw)
	}

	intransitiveChecks := filters.IntransitiveActivityChecks(checks...)
	activityChecks := filters.ActivityChecks(checks...)
	actorChecks := filters.ActorChecks(checks...)
	objectChecks := filters.ObjectChecks(checks...)

	authorizedChecks := filters.AuthorizedChecks(checks...)

	typ := it.GetType()
	// NOTE(marius): this can probably expedite filtering if we early exit when we fail to load the
	// properties that need to be loaded for sub-filters.
	var err error
	switch {
	case vocab.IntransitiveActivityTypes.Match(typ):
		intransitiveChecks = append(intransitiveChecks, authorizedChecks...)
		err = vocab.OnIntransitiveActivity(it, loadFilteredPropsForIntransitiveActivity(ms, intransitiveChecks...))
	case vocab.ActivityTypes.Match(typ):
		activityChecks = append(activityChecks, authorizedChecks...)
		err = vocab.OnActivity(it, loadFilteredPropsForActivity(ms, activityChecks...))
	case vocab.ActorTypes.Match(typ):
		actorChecks = append(actorChecks, authorizedChecks...)
		err = vocab.OnActor(it, loadFilteredPropsForActor(ms, actorChecks...))
	case vocab.ObjectTypes.Match(typ):
		objectChecks = append(objectChecks, authorizedChecks...)
		err = vocab.OnObject(it, loadFilteredPropsForObject(ms, objectChecks...))
	case allCollectionTypes.Match(typ):
		// NOTE(marius): for the collections that we loaded from the memory store, we need to
		// filter their items on a clone, otherwise the stored value gets modified.
		it = vocab.Clone(it)
	}
	// NOTE(marius): this is done another time, in order to paginate collections
	return filters.Checks(checks).Run(it), err
}

func loadFilteredPropsForIntransitiveActivity(ms *memStorage, checks ...filters.Check) vocab.WithIntransitiveActivityFn {
	targetChecks := filters.TargetChecks(checks...)
	actorChecks := filters.ActorChecks(checks...)
	return func(a *vocab.IntransitiveActivity) error {
		var err error
		if !vocab.IsNil(a.Target) && len(targetChecks) > 0 && !a.ID.Equal(a.Target.GetLink()) {
			if a.Target, err = dereferenceItemAndFilter(ms, a.Target, targetChecks...); err != nil {
				return err
			}
		}
		if !vocab.IsNil(a.Actor) && len(actorChecks) > 0 && !a.ID.Equal(a.Actor.GetLink()) {
			if a.Actor, err = dereferenceItemAndFilter(ms, a.Actor, actorChecks...); err != nil {
				return err
			}
		}
		return vocab.OnObject(a, loadFilteredPropsForObject(ms, checks...))
	}
}

func loadFilteredPropsForActivity(ms *memStorage, checks ...filters.Check) vocab.WithActivityFn {
	objectChecks := filters.ObjectChecks(checks...)
	return func(a *vocab.Activity) error {
		if !vocab.IsNil(a.Object) && !a.ID.Equal(a.Object.GetLink()) {
			var err error
			if a.Object, err = dereferenceItemAndFilter(ms, a.Object, objectChecks...); err != nil {
				return err
			}
		}
		return vocab.OnIntransitiveActivity(a, loadFilteredPropsForIntransitiveActivity(ms, checks...))
	}
}

func loadFilteredPropsForActor(ms *memStorage, checks ...filters.Check) vocab.WithActorFn {
	return func(actor *vocab.Actor) error {
		return vocab.OnObject(actor, loadFilteredPropsForObject(ms, checks...))
	}
}

func loadFilteredPropsForObject(ms *memStorage, fil ...filters.Check) func(o *vocab.Object) error {
	tagChecks := filters.TagChecks(fil...)
	if len(tagChecks) == 0 {
		tagChecks = filters.Checks{filters.NoType}
	}
	return func(o *vocab.Object) error {
		if vocab.IsNil(o.Tag) {
			return nil
		}
		var err error
		o.Tag, err = dereferenceItemAndFilter(ms, o.Tag, tagChecks...)
		return err
	}
}

func dereferenceItemAndFilter(ms *memStorage, it vocab.Item, checks ...filters.Check) (vocab.Item, error) {
	if vocab.IsNil(it) {
		return it, nil
	}
	if len(checks) == 0 {
		// NOTE(marius): no filtering on the object
		return it, nil
	}

	res := make(vocab.ItemCollection, 0)
	err := vocab.OnItem(it, func(iit vocab.Item) error {
		if vocab.IsNil(iit) {
			return nil
		}
		if vocab.IsIRI(iit) {
			o, err := ms.Load(iit.GetLink())
			if err != nil {
				return nil
			}
			if o = filters.Checks(checks).Run(o); o != nil {
				iit = o
			}
		}
		return res.Append(iit)
	})
	return res.Normalize(), err
}

func saveCollectionIfExists(r *memStorage, it, owner vocab.Item) vocab.Item {
	if vocab.IsNil(it) {
		return nil
	}
	r.Map.LoadOrStore(it.GetLink(), createNewCollection(it.GetLink(), owner))
	return it.GetLink()
}

func createNewCollection(colIRI vocab.IRI, owner vocab.Item) vocab.CollectionInterface {
	col := vocab.OrderedCollection{
		ID:        colIRI,
		Type:      vocab.OrderedCollectionType,
		CC:        vocab.ItemCollection{vocab.PublicNS},
		Published: time.Now().Truncate(time.Second).UTC(),
	}
	if !vocab.IsNil(owner) {
		col.AttributedTo = owner.GetLink()
	}
	return &col
}

// createItemCollections
func createItemCollections(ms *memStorage, it vocab.Item) error {
	if vocab.IsNil(it) || !vocab.IsObject(it) {
		return nil
	}
	if vocab.ActorTypes.Match(it.GetType()) {
		_ = vocab.OnActor(it, func(p *vocab.Actor) error {
			p.Inbox = saveCollectionIfExists(ms, p.Inbox, p)
			p.Outbox = saveCollectionIfExists(ms, p.Outbox, p)
			p.Followers = saveCollectionIfExists(ms, p.Followers, p)
			p.Following = saveCollectionIfExists(ms, p.Following, p)
			p.Liked = saveCollectionIfExists(ms, p.Liked, p)
			// NOTE(marius): shadow creating hidden collections for Blocked and Ignored items
			saveCollectionIfExists(ms, filters.BlockedType.Of(p), p)
			saveCollectionIfExists(ms, filters.IgnoredType.Of(p), p)
			return nil
		})
	}
	return vocab.OnObject(it, func(o *vocab.Object) error {
		o.Replies = saveCollectionIfExists(ms, o.Replies, o)
		o.Likes = saveCollectionIfExists(ms, o.Likes, o)
		o.Shares = saveCollectionIfExists(ms, o.Shares, o)
		return nil
	})
}

func (ms *memStorage) Save(it vocab.Item) (vocab.Item, error) {
	if _, ok := ms.Map.Load(it.GetLink()); !ok {
		if err := createItemCollections(ms, it); err != nil {
			return it, errors.Annotatef(err, "could not create object's collections")
		}
	}
	ms.Map.Store(it.GetLink(), it)
	return it, nil
}

func (ms *memStorage) Delete(it vocab.Item) error {
	ms.Map.Delete(it.GetLink())
	return nil
}

func (ms *memStorage) Create(col vocab.CollectionInterface) (vocab.CollectionInterface, error) {
	it, err := ms.Save(col)
	if err != nil {
		return nil, err
	}
	col, ok := it.(vocab.CollectionInterface)
	if !ok {
		return nil, errors.Newf("invalid collection saved")
	}
	return col, nil
}

func (ms *memStorage) loadCol(colIRI vocab.IRI) (vocab.CollectionInterface, error) {
	it, ok := ms.Map.Load(colIRI)
	if !ok {
		return nil, errors.NotFoundf("unable to load collection %s", colIRI)
	}
	col, ok := it.(vocab.CollectionInterface)
	if !ok {
		return nil, errors.Newf("invalid collection type %T %s", it, colIRI)
	}
	return col, nil
}

func (ms *memStorage) AddTo(colIRI vocab.IRI, items ...vocab.Item) error {
	col, err := ms.loadCol(colIRI)
	if err != nil {
		return err
	}

	if err = col.Append(items...); err != nil {
		return err
	}

	_, err = ms.Save(col)
	return err
}

func (ms *memStorage) RemoveFrom(colIRI vocab.IRI, items ...vocab.Item) error {
	col, err := ms.loadCol(colIRI)
	if err != nil {
		return err
	}

	col.Remove(items...)

	_, err = ms.Save(col)
	return err
}

var _ processing.Store = &memStorage{}
