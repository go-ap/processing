package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
	"github.com/go-ap/processing"
)

type outboxCollectionManagementTests struct {
	// NOTE(marius): elements needed to initialize the processor.P
	mp MockPlumbing
	st *LocalStorage

	// NOTE(marius): test to be run
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (o *outboxCollectionManagementTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		o.mp = MockPlumbing{}
		o.st = nil
		return ctx, nil
	})

	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)

	// NOTE(marius): Add
	ctx.Step(`^an Activity with an Object and a Target collection is received$`, o.anActivityWithObjectAndATargetCollectionIsReceived)
	ctx.Step(`^the object will exist in the Target$`, o.theObjectWillExistInTheTarget)

	// NOTE(marius): Remove
	ctx.Step(`^an Activity with an Object and an Origin collection is received$`, o.anActivityWithAnObjectAndAnOriginCollectionIsReceived)
	ctx.Step(`^the object will not exist in the Origin$`, o.theObjectWillNotExistInTheOrigin)
}

func (o *outboxCollectionManagementTests) anActivityWithObjectAndATargetCollectionIsReceived(ctx context.Context) error {
	targetID := RootID.AddPath("target")
	target := build.OrderedCollection(build.WithID(targetID))

	noteID := RootID.AddPath("to-move")
	toAdd := build.Note(
		build.WithID(noteID),
		build.WithName("Lipsum name"),
		build.WithAttributedTo(AuthorID),
		build.WithReplies(),
		build.WithTo(vocab.ItemCollection{vocab.PublicNS}),
	)
	o.st = Storage(WithLocalMocks(Author, target, toAdd))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Add(
		build.WithActor(Author.ID),
		build.WithObject(noteID),
		build.WithTarget(targetID),
	)

	return nil
}

func (o *outboxCollectionManagementTests) theObjectWillExistInTheTarget(ctx context.Context) error {
	p := o.mp.InitP(processing.WithStorage(o.st))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return fmt.Errorf("received error: %w", err)
	}

	targetID := RootID.AddPath("target")
	noteID := RootID.AddPath("to-move")

	targetCol, err := o.st.Load(targetID)
	if err != nil {
		return fmt.Errorf("unable to load collection %s: %w", targetID, err)
	}
	err = vocab.OnOrderedCollection(targetCol, func(collection *vocab.OrderedCollection) error {
		if !collection.OrderedItems.IRIs().Contains(noteID) {
			return fmt.Errorf("unable to find note %s in collection %s", noteID, targetID)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("target %s wasn't a collection %w", targetID, err)
	}
	return nil
}

func (o *outboxCollectionManagementTests) anActivityWithAnObjectAndAnOriginCollectionIsReceived(ctx context.Context) error {
	noteID := RootID.AddPath("to-move")
	toRemove := build.Note(
		build.WithID(noteID),
		build.WithName("Lipsum name"),
		build.WithAttributedTo(AuthorID),
		build.WithReplies(),
		build.WithTo(vocab.ItemCollection{vocab.PublicNS}),
	)

	originID := RootID.AddPath("target")
	target := build.OrderedCollection(
		build.WithID(originID),
		build.WithTotalItems(1),
		build.WithItems(toRemove),
	)

	o.st = Storage(WithLocalMocks(Author, target, toRemove))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Remove(
		build.WithActor(Author.ID),
		build.WithObject(noteID),
		build.WithTarget(originID),
	)

	return nil
}

func (o *outboxCollectionManagementTests) theObjectWillNotExistInTheOrigin(ctx context.Context) error {
	p := o.mp.InitP(processing.WithStorage(o.st))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return fmt.Errorf("received error: %w", err)
	}

	targetID := RootID.AddPath("target")
	noteID := RootID.AddPath("to-move")

	targetCol, err := o.st.Load(targetID)
	if err != nil {
		return fmt.Errorf("unable to load collection %s: %w", targetID, err)
	}
	err = vocab.OnOrderedCollection(targetCol, func(collection *vocab.OrderedCollection) error {
		if collection.TotalItems != 0 {
			return fmt.Errorf("total items for collection %s should have been %d, received %d", targetID, 0, collection.TotalItems)
		}
		if collection.OrderedItems.IRIs().Contains(noteID) {
			return fmt.Errorf("note %s still exists in collection %s", noteID, targetID)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("target %s wasn't a collection %w", targetID, err)
	}
	return nil
}

func Test_Outbox_CollectionManagement(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"outbox-collection-management.feature"}

	c := new(outboxCollectionManagementTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Outbox Collection Management",
		ScenarioInitializer: c.initScenario,
		Options:             &opts,
	}.Run()
}

func Test_Outbox_Add(t *testing.T) {
	t.Run("Add", func(t *testing.T) {
		mp := MockPlumbing{}

		targetID := RootID.AddPath("target")
		target := build.OrderedCollection(build.WithID(targetID))

		noteID := RootID.AddPath("to-move")
		toAdd := build.Note(
			build.WithID(noteID),
			build.WithName("Lipsum name"),
			build.WithAttributedTo(AuthorID),
			build.WithReplies(),
			build.WithTo(vocab.ItemCollection{vocab.PublicNS}),
		)

		st := Storage(WithLocalMocks(Author, toAdd, target))
		p := mp.InitP(processing.WithStorage(st))
		tests := TestSuite{
			Name: "Add",
			Tests: []RunnableTest{
				TestPair{
					Name:       "Object to Target",
					Author:     *Author,
					ReceivedIn: vocab.Outbox.IRI(Author),
					Activity: build.Add(
						build.WithActor(Author.ID),
						build.WithObject(noteID),
						build.WithTarget(targetID),
					),
					TestFns: TestAssertions{
						HasID(mp.ShiftIRI),
						HasType(vocab.AddType),
						WasPublished(mp.PopTime),
						RequestObject(HasID(noteID)),
					},
				},
			},
		}

		t.Run(tests.Label(), tests.TestFn(p))
	})
}

func Test_Outbox_Move(t *testing.T) {
	t.Run("Move", func(t *testing.T) {
		t.Skipf("TODO")
		mp := MockPlumbing{}

		st := Storage(WithLocalMocks(Author))
		P := mp.InitP(processing.WithStorage(st))

		tests := TestSuite{}
		t.Run(tests.Label(), tests.TestFn(P))
	})
}

func Test_Outbox_Remove(t *testing.T) {
	t.Run("Remove", func(t *testing.T) {
		t.Skipf("TODO")
		mp := MockPlumbing{}
		st := Storage(WithLocalMocks(Author))
		P := mp.InitP(processing.WithStorage(st))

		tests := TestSuite{}
		t.Run(tests.Label(), tests.TestFn(P))
	})
}
