package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
	"github.com/go-ap/errors"
	"github.com/go-ap/processing"
	"github.com/google/go-cmp/cmp"
)

type outboxNegateTests struct {
	// NOTE(marius): elements needed to initialize the processor.P
	mp MockPlumbing
	st *LocalStorage

	// NOTE(marius): test to be run
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (o *outboxNegateTests) anUndoActivityIsReceived() error {
	toUndoID := RootID.AddPath("to-undo")
	toUndo := build.Create(
		build.WithID(toUndoID),
		build.WithActor(AuthorID),
		build.WithObject(build.Note(build.WithContent("Lorem ipsum"))),
	)

	o.st = Storage(WithLocalMocks(Author, toUndo))
	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Undo(
		build.WithActor(Author.ID),
		build.WithObject(toUndoID),
	)
	return nil
}

func (o *outboxNegateTests) anUndoActivityWithDifferentActorThanTheOneOfItsObjectIsReceived() error {
	toUndoID := RootID.AddPath("to-undo")
	toUndo := build.Create(
		build.WithID(toUndoID),
		build.WithActor(vocab.IRI("http://example.com/~notDoe")),
		build.WithObject(build.Note(build.WithContent("Lorem ipsum"))),
	)

	o.st = Storage(WithLocalMocks(Author, toUndo))
	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Undo(
		build.WithActor(Author.ID),
		build.WithObject(toUndoID),
	)
	return nil
}

func (o *outboxNegateTests) thereIsAValidationError() error {
	expectedErr := errors.BadRequestf("the %s activity has a different actor than its object: %s, expected %s", vocab.UndoType, AuthorID, "http://example.com/~notDoe")
	p := o.mp.InitP(processing.WithStorage(o.st))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if cmp.Equal(err, expectedErr, EquateWeakErrors) {
		return nil
	}
	return fmt.Errorf("unexpected error received: %w", err)
}

func (o *outboxNegateTests) itGetsAccepted() error {
	p := o.mp.InitP(processing.WithStorage(o.st))
	act, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return fmt.Errorf("received error: %w", err)
	}
	if vocab.IsNil(act) {
		return fmt.Errorf("returned activity is nil %v", act)
	}
	return nil
}

func (o *outboxNegateTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		o.mp = MockPlumbing{}
		o.st = nil
		return ctx, nil
	})

	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)

	ctx.Step(`^an Undo activity is received$`, o.anUndoActivityIsReceived)
	ctx.Step(`^it gets accepted$`, o.itGetsAccepted)

	ctx.Step(`^an Undo activity with different Actor than the one of it\'s Object is received$`, o.anUndoActivityWithDifferentActorThanTheOneOfItsObjectIsReceived)
	ctx.Step(`^there is a validation error$`, o.thereIsAValidationError)
}

func Test_Outbox_Negate(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"outbox-negate.feature"}

	c := new(outboxNegateTests)
	godog.TestSuite{
		Name:                "ActivityPub Outbox Negate",
		ScenarioInitializer: c.initScenario,
		Options:             &opts,
	}.Run()
}

func Test_Outbox_Undo(t *testing.T) {
	t.Run("Undo", func(t *testing.T) {
		t.Skipf("TODO")
	})
}
