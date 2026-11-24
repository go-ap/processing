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

type generalOutboxTests struct {
	// NOTE(marius): elements needed to initialize the processor.P
	mp MockPlumbing
	st *LocalStorage

	// NOTE(marius): test to be run
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (c *generalOutboxTests) anActivityIsReceived(ctx context.Context) error {
	c.st = Storage(WithLocalMocks(Author))

	c.Author = *Author
	c.ReceivedIn = vocab.Outbox.IRI(Author)
	c.Activity = build.Announce(
		build.WithActor(Author.ID),
		build.WithObject(build.Note(
			build.WithID("http://example.com"),
			build.WithContent("Lorem ipsum"),
		)),
	)

	return nil
}

func (c *generalOutboxTests) aNonActivityObjectIsReceived(ctx context.Context) error {
	c.st = Storage(WithLocalMocks(Author))

	c.Author = *Author
	c.ReceivedIn = vocab.Outbox.IRI(Author)
	c.Activity = build.Note(build.WithID("http://example.com"))

	return nil
}

func (c *generalOutboxTests) itGetsWrappedIntoACreateActivity(ctx context.Context) error {
	p := c.mp.InitP(processing.WithStorage(c.st))
	_, err := p.ProcessActivity(c.Activity, c.Author, c.ReceivedIn)
	if err != nil {
		return fmt.Errorf("received unexpected error: %w", err)
	}
	return nil
}

func (c *generalOutboxTests) itGetsAccepted(ctx context.Context) error {
	p := c.mp.InitP(processing.WithStorage(c.st))
	_, err := p.ProcessActivity(c.Activity, c.Author, c.ReceivedIn)
	return err
}

func (c *generalOutboxTests) objectsWithBtoAndBCCProperties(ctx context.Context) error {
	c.st = Storage(WithLocalMocks(Author))

	c.Author = *Author
	c.ReceivedIn = vocab.Outbox.IRI(Author)
	c.Activity = build.Create(
		build.WithObject(build.Note(build.WithID("http://example.com"))),
		build.WithActor(AuthorID),
		build.WithBCC(AuthorID),
		build.WithBto(vocab.Followers.IRI(AuthorID)),
	)

	return nil
}

func (c *generalOutboxTests) theBtoAndBCCPropertiesAreRemoved(ctx context.Context) error {
	p := c.mp.InitP(processing.WithStorage(c.st))
	act, err := p.ProcessActivity(c.Activity, c.Author, c.ReceivedIn)
	if err != nil {
		return err
	}
	errNotEmpty := func(prop string) error {
		return fmt.Errorf("%s collection is not empty", prop)
	}

	return vocab.OnActivity(act, func(act *vocab.Activity) error {
		return vocab.OnObject(act.Object, func(ob *vocab.Object) error {
			if len(ob.BCC) > 0 {
				return errNotEmpty("BCC")
			}
			if len(ob.Bto) > 0 {
				return errNotEmpty("Bto")
			}

			savedOb, err := c.st.Load(ob.ID)
			if err != nil {
				return fmt.Errorf("failed to load object from storage: %w", err)
			}
			return vocab.OnObject(savedOb, func(ob *vocab.Object) error {
				if len(ob.BCC) > 0 {
					return fmt.Errorf("local stored object's %w", errNotEmpty("BCC"))
				}
				if len(ob.Bto) > 0 {
					return fmt.Errorf("local stored object's %w", errNotEmpty("Bto"))
				}
				return nil
			})
		})
	})
}

func (c *generalOutboxTests) anActivityWithExistingIDIsReceived(ctx context.Context) error {
	c.st = Storage(WithLocalMocks(Author))

	c.Author = *Author
	c.ReceivedIn = vocab.Outbox.IRI(Author)
	c.Activity = build.Create(
		build.WithID("http://example.com/activity1"),
		build.WithObject(build.Note(build.WithID("http://example.com"))),
		build.WithActor(AuthorID),
		build.WithBCC(AuthorID),
		build.WithBto(vocab.Followers.IRI(AuthorID)),
	)

	return nil
}

func (c *generalOutboxTests) ignoresItAndGeneratesANewIDInstead(ctx context.Context) error {
	p := c.mp.InitP(processing.WithStorage(c.st))
	act, err := p.ProcessActivity(c.Activity, c.Author, c.ReceivedIn)
	if err != nil {
		return err
	}

	return vocab.OnActivity(act, func(act *vocab.Activity) error {
		if act.ID.Equal("http://example.com/activity1") {
			return fmt.Errorf("activity ID is the same after processing")
		}
		return nil
	})
}

func (c *generalOutboxTests) noAuthor(ctx context.Context) error {
	c.Author = vocab.Actor{}
	return nil
}

func (c *generalOutboxTests) thereIsAValidationError(ctx context.Context) error {
	expectedErr := errors.Unauthorizedf("missing actor: not allowed to post to outbox %s", c.ReceivedIn)
	p := c.mp.InitP(processing.WithStorage(c.st))
	_, err := p.ProcessActivity(c.Activity, c.Author, c.ReceivedIn)
	if cmp.Equal(err, expectedErr, EquateWeakErrors) {
		return nil
	}
	return fmt.Errorf("expected error and none received: %w", err)
}

func (c *generalOutboxTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		c.mp = MockPlumbing{}
		c.st = nil
		return ctx, nil
	})

	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)

	// NOTE(marius): success on activity
	ctx.Step(`^an Activity is received$`, c.anActivityIsReceived)
	ctx.Step(`^it gets accepted$`, c.itGetsAccepted)

	// NOTE(marius): failing on non activities
	ctx.Step(`^a non-Activity Object is received$`, c.aNonActivityObjectIsReceived)
	ctx.Step(`^it gets wrapped into a Create Activity$`, c.itGetsWrappedIntoACreateActivity)

	// NOTE(marius): strip bcc and bto properties
	ctx.Step("^an Activity with an Object having 'bto' and 'bcc' properties is received$", c.objectsWithBtoAndBCCProperties)
	ctx.Step(`^the 'bto' and 'bcc' aren't removed before storage and delivery$`, c.theBtoAndBCCPropertiesAreRemoved)

	// NOTE(marius): recreate ID for activity
	ctx.Step(`^an Activity with existing ID is received$`, c.anActivityWithExistingIDIsReceived)
	ctx.Step(`^ignores it and generates a new ID instead$`, c.ignoresItAndGeneratesANewIDInstead)

	// NOTE(marius): no author activity
	ctx.Step(`^no Author$`, c.noAuthor)
	ctx.Step(`^there is a validation error$`, c.thereIsAValidationError)
}

func Test_GeneralOutbox(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"outbox-general.feature"}

	c := new(generalOutboxTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub compatibility tests",
		ScenarioInitializer: c.initScenario,
		Options:             &opts,
	}.Run()
}
