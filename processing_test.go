package processing

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"testing"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/processing/internal"
)

var idGenerator = func() func(it vocab.Item, byActivity vocab.Item) (vocab.ID, error) {
	cnt := atomic.Int32{}
	return func(it vocab.Item, byActivity vocab.Item) (vocab.ID, error) {
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
}

func ExampleP_ProcessActivity_in_outbox() {
	allIRIsAreLocal := func(_ vocab.IRI) bool { return true }
	storage := new(internal.MockStorage)
	p := New(
		WithStorage(storage),
		WithLocalIRIChecker(allIRIsAreLocal),
		WithIDGenerator(idGenerator()),
	)

	actor := vocab.Actor{
		ID:     "http://example.com/~jdoe",
		Type:   vocab.PersonType,
		Outbox: vocab.IRI("http://example.com/~jdoe/outbox"),
	}
	object := &vocab.Note{}
	activity := vocab.Activity{
		Type:   vocab.CreateType,
		Actor:  actor,
		Object: object,
	}
	_, _ = storage.Save(actor)
	_, _ = storage.Save(&vocab.OrderedCollection{ID: vocab.Outbox.IRI(actor), Type: vocab.OrderedCollectionType})

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

func ExampleP_ProcessActivity_in_inbox() {
	noIRIsAreLocal := func(_ vocab.IRI) bool { return false }
	storage := new(internal.MockStorage)
	p := New(
		WithStorage(storage),
		WithLocalIRIChecker(noIRIsAreLocal),
	)

	actor := vocab.Actor{
		ID:     "http://example.com/~jdoe",
		Type:   vocab.PersonType,
		Inbox:  vocab.IRI("http://example.com/~jdoe/inbox"),
		Outbox: vocab.IRI("http://example.com/~jdoe/outbox"),
	}
	_, _ = storage.Save(actor)
	_, _ = storage.Save(&vocab.OrderedCollection{ID: vocab.Outbox.IRI(actor), Type: vocab.OrderedCollectionType})
	_, _ = storage.Save(&vocab.OrderedCollection{ID: vocab.Inbox.IRI(actor), Type: vocab.OrderedCollectionType})

	object := &vocab.Note{}
	activity := vocab.Activity{
		ID:     "http://example.com/~jdoe/outbox/0",
		Type:   vocab.CreateType,
		Actor:  actor,
		Object: object,
	}

	it, err := p.ProcessActivity(activity, actor, vocab.Inbox.IRI(actor))
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

	// Output: error: IRI is not valid: empty
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
