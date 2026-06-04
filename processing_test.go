package processing

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/filters"
)

type memStorage struct {
	sync.Map
}

func (ms *memStorage) Load(i vocab.IRI, f ...filters.Check) (vocab.Item, error) {
	raw, ok := ms.Map.Load(i)
	if !ok {
		return nil, errors.NotFoundf("unable to find %s", i)
	}
	ob, ok := raw.(vocab.Item)
	if !ok {
		return nil, errors.Newf("invalid item type in storage %T", raw)
	}

	if len(f) == 0 {
		return ob, nil
	}
	typ := ob.GetType()
	switch {
	case vocab.ActivityVocabularyTypes{vocab.OrderedCollectionType, vocab.OrderedCollectionPageType}.Match(typ):
		clone, _ := ob.(*vocab.OrderedCollection)
		obCopy := *clone
		return filters.Checks(f).Run(&obCopy), nil
	case vocab.ActivityVocabularyTypes{vocab.CollectionType, vocab.CollectionPageType}.Match(typ):
		clone, _ := ob.(*vocab.Collection)
		obCopy := *clone
		return filters.Checks(f).Run(&obCopy), nil
	default:
		return ob, nil
	}
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
	if vocab.IsNil(it) || !it.IsObject() {
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

func (ms *memStorage) loadCol(colIRI vocab.IRI) (vocab.CollectionInterface, error) {
	it, ok := ms.Map.Load(colIRI)
	if !ok {
		return nil, errors.Newf("unable to load collection %s", colIRI)
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

func ExampleP_ProcessActivity_in_outbox() {
	allIRIsAreLocal := func(_ vocab.IRI) bool { return true }

	cnt := atomic.Int32{}
	idGenerator := func(it vocab.Item, byActivity vocab.Item) (vocab.ID, error) {
		defer func() { cnt.Add(1) }()
		var actorID vocab.ID
		switch {
		case append(vocab.ActivityTypes, vocab.IntransitiveActivityTypes...).Match(it.GetType()):
			_ = vocab.OnActivity(it, func(act *vocab.Activity) error {
				actorID = act.Actor.GetID()
				return nil
			})
			return actorID.AddPath("outbox", strconv.Itoa(int(cnt.Load()))), nil
		default:
			_ = vocab.OnActivity(byActivity, func(act *vocab.Activity) error {
				actorID = act.Actor.GetID()
				return nil
			})
			return actorID.AddPath("objects", strconv.Itoa(int(cnt.Load()))), nil
		}
	}
	p := New(
		WithStorage(new(memStorage)),
		WithLocalIRIChecker(allIRIsAreLocal),
		WithIDGenerator(idGenerator),
	)

	actor := vocab.Actor{
		ID:     "http://example.com/~jdoe",
		Outbox: vocab.IRI("http://example.com/~jdoe/outbox"),
		Type:   vocab.PersonType,
	}
	object := &vocab.Note{}
	activity := vocab.Activity{
		Type:   vocab.CreateType,
		Actor:  actor,
		Object: object,
	}

	it, err := p.ProcessActivity(activity, actor, vocab.Outbox.IRI(actor))
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	_ = vocab.OnIntransitiveActivity(it, func(act *vocab.IntransitiveActivity) error {
		fmt.Printf("Activity ID: %v\n", act.ID)
		fmt.Printf("       Type: %v\n", act.Type)
		fmt.Printf("   Actor ID: %v\n", act.Actor.GetID())
		return vocab.OnActivity(it, func(act *vocab.Activity) error {
			fmt.Printf("  Object ID: %v\n", act.Object.GetID())
			return nil
		})
	})

	// Output:
	// Activity ID: http://example.com/~jdoe/outbox/0
	//        Type: Create
	//    Actor ID: http://example.com/~jdoe
	//   Object ID: http://example.com/~jdoe/objects/1
}

func TestUpdateItemProperties(t *testing.T) {
	t.Skipf("TODO")
}

func TestUpdateObjectProperties(t *testing.T) {
	t.Skipf("TODO")
}

func TestUpdatePersonProperties(t *testing.T) {
	t.Skipf("TODO")
}

func TestCollectionManagementActivity(t *testing.T) {
	t.Skipf("TODO")
}

func TestValidActivityCollection(t *testing.T) {
	t.Skipf("TODO")
}

func TestRelationshipManagementActivity(t *testing.T) {
	t.Skipf("TODO")
}

func TestContentExperienceActivity(t *testing.T) {
	t.Skipf("TODO")
}

func TestQuestionActivity(t *testing.T) {
	t.Skipf("TODO")
}

func TestEventRSVPActivity(t *testing.T) {
	t.Skipf("TODO")
}

func TestGeoSocialEventsActivity(t *testing.T) {
	t.Skipf("TODO")
}

func TestGroupManagementActivity(t *testing.T) {
	t.Skipf("TODO")
}

func TestNotificationActivity(t *testing.T) {
	t.Skipf("TODO")
}

func TestOffersActivity(t *testing.T) {
	t.Skipf("TODO")
}
