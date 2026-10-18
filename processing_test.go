package processing

import (
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
	conformance "github.com/go-ap/storage-conformance-suite"
)

var testIDGenerator = func() func(it vocab.Item, byActivity vocab.Item) (vocab.ID, error) {
	cnt := atomic.Int32{}
	return func(it vocab.Item, byActivity vocab.Item) (vocab.ID, error) {
		defer func() { cnt.Add(1) }()
		path := "object"

		typ := it.GetType()
		switch {
		case vocab.ActorTypes.Match(typ):
			path = "actor"
		case append(vocab.ActivityTypes, vocab.IntransitiveActivityTypes...).Match(typ):
			path = "activity"
		}
		pref := ""
		if typ != nil {
			pref = strings.ToLower(typ.AsTypes().String()) + "-"
		}
		return vocab.IRI("http://example.com").AddPath(path, pref+strconv.Itoa(int(cnt.Load()))), nil

	}
}

func ExampleP_ProcessActivity_in_outbox() {
	allIRIsAreLocal := func(_ vocab.IRI) bool { return true }
	storage := new(conformance.MemStorage)
	p := New(
		WithStorage(storage),
		WithLocalIRIChecker(allIRIsAreLocal),
		WithIDGenerator(testIDGenerator()),
	)

	actor := build.Person(
		build.WithID("http://example.com/~jdoe"),
		build.WithOutbox(vocab.IRI("http://example.com/~jdoe/outbox")),
	)
	outbox := build.OrderedCollection(build.WithID(vocab.Outbox.IRI(actor)))
	object := build.Note()
	activity := build.Create(
		build.WithActor(actor),
		build.WithObject(object),
	)
	_, _ = storage.Save(actor)
	_, _ = storage.Save(outbox)

	it, err := p.ProcessActivity(activity, *actor, vocab.Outbox.IRI(actor))
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
	// Activity ID: http://example.com/activity/create-0
	//        Type: Create
	//    Actor ID: http://example.com/~jdoe
	//   Object ID: http://example.com/object/note-1
}

func ExampleP_ProcessActivity_in_inbox() {
	noIRIsAreLocal := func(_ vocab.IRI) bool { return false }
	storage := new(conformance.MemStorage)
	p := New(
		WithStorage(storage),
		WithLocalIRIChecker(noIRIsAreLocal),
	)

	actor := build.Person(
		build.WithID("http://example.com/~jdoe"),
		build.WithInbox(vocab.IRI("http://example.com/~jdoe/inbox")),
		build.WithOutbox(vocab.IRI("http://example.com/~jdoe/outbox")),
	)
	outbox := build.OrderedCollection(build.WithID(vocab.Outbox.IRI(actor)))
	inbox := build.OrderedCollection(build.WithID(vocab.Inbox.IRI(actor)))

	_, _ = storage.Save(actor)
	_, _ = storage.Save(inbox)
	_, _ = storage.Save(outbox)

	object := build.Note()
	activity := build.Create(
		build.WithID("http://example.com/~jdoe/outbox/0"),
		build.WithActor(actor),
		build.WithObject(object),
	)

	it, err := p.ProcessActivity(activity, *actor, vocab.Inbox.IRI(actor))
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
