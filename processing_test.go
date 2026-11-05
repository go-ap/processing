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
		// We know that all relevant IRIs are local.
		WithLocalIRIChecker(allIRIsAreLocal),
		WithIDGenerator(testIDGenerator()),
	)

	// This is an actor extracted from the HTTP request authorization
	// headers.
	// For how to do that see the go-ap/authorize module, which can extract
	// HTTP-Signatures, and OAuth2 bearer tokens.
	authorized := build.Person(
		build.WithID("http://example.com/~jdoe"),
		build.WithOutbox(vocab.IRI("http://example.com/~jdoe/outbox")),
	)
	outbox := build.OrderedCollection(build.WithID(vocab.Outbox.IRI(authorized)))

	// We save the relevant actor and its outbox locally
	_, _ = storage.Save(authorized)
	_, _ = storage.Save(outbox)

	// This is the client activity we're processing
	activity := build.Create(
		build.WithActor(authorized),
		build.WithObject(build.Note()),
	)

	// The processor needs to know the URL on which the current request has
	// been operated on.
	// As this is a Social API request, it needs to correspond to the
	// authorized actor's /outbox.
	receivedIn := vocab.Outbox.IRI(authorized)

	// After processing the Activity, it will have been updated with any
	// information the specification mentions for the combination of Activity
	// and Object types.
	it, err := p.ProcessActivity(activity, *authorized, receivedIn)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	// This is an example of how to operate on an Activity returned by
	// the ProcessActivity() call.
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

	// The client is used when it's required to dereference properties of the
	// incoming that are referred only by their IDs into full ActivityPub
	// objects. Some instances require that remote fetching is done using
	// requests containing valid HTTP-Signatures, so take that into
	// consideration. See the go-ap/client module for how to initialize it for
	// such cases.
	client := mockClient()

	p := New(
		WithStorage(storage),
		// No IRIs are local for this example.
		WithLocalIRIChecker(noIRIsAreLocal),
		WithClient(client),
	)

	local := build.Person(
		build.WithID("http://social.example.com/alice"),
		build.WithInbox(vocab.IRI("http://social.example.com/alice/inbox")),
		build.WithOutbox(vocab.IRI("http://social.example.com/alice/outbox")),
	)

	// We save the receiving actor and its collections locally
	inbox := build.OrderedCollection(build.WithID(vocab.Inbox.IRI(local)))
	outbox := build.OrderedCollection(build.WithID(vocab.Outbox.IRI(local)))
	_, _ = storage.Save(local)
	_, _ = storage.Save(inbox)
	_, _ = storage.Save(outbox)

	// This is the actor extracted from the HTTP request authorization
	// headers.
	// For how to do that, see the go-ap/authorize module, which can extract
	// HTTP-Signatures, and OAuth2 bearer tokens.
	authorized := build.Person(
		build.WithID("http://example.com/~jdoe"),
		build.WithInbox(vocab.IRI("http://example.com/~jdoe/inbox")),
		build.WithOutbox(vocab.IRI("http://example.com/~jdoe/outbox")),
	)

	// The authorized actor should be available locally to avoid an
	// outbound fetch request for it.
	_, _ = storage.Save(authorized)

	activity := build.Create(
		// The remote activity must have an ID to pass validation.
		build.WithID("http://example.com/~jdoe/outbox/0"),
		// The incoming activity's Actor needs to match the authorized
		// one from above.
		build.WithActor(authorized),
		build.WithObject(build.Note(build.WithID("http://example.com/~jdoe/note-1"))),
	)

	// The processor needs to know the URL on which the current request has
	// been operated on.
	// As this is a Federated Protocol request, it needs to correspond
	// to the actor's /inbox
	receivedIn := vocab.Inbox.IRI(local)

	// After processing the Activity, it will have been updated with any
	// information the specification mentions for the combination of Activity
	// and Object types.
	it, err := p.ProcessActivity(activity, *authorized, receivedIn)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}

	// This is an example of how to operate on an Activity returned by
	// the ProcessActivity() call.
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
	//   Object ID: http://example.com/~jdoe/note-1
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
