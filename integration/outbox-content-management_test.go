package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
	"github.com/go-ap/errors"
	"github.com/go-ap/processing"
	"github.com/google/go-cmp/cmp"
)

type outboxContentManagementTests struct {
	// NOTE(marius): elements needed to initialize the processor.P
	mp MockPlumbing
	st *LocalStorage

	// NOTE(marius): test to be run
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (o *outboxContentManagementTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		o.mp = MockPlumbing{}
		o.st = nil
		return ctx, nil
	})

	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)

	// NOTE(marius): create merges recipients for activity and object
	ctx.Step(`^a Create and Object pair with to, bto, cc, bcc and audience$`, o.aCreateAndObjectPairWithToBtoCcBccAndAudience)
	ctx.Step(`^Object and Activity have merged to, bto, cc, bcc and audience$`, o.objectAndActivityHaveMergedToBtoCcBccAndAudience)

	// NOTE(marius): create adds activity actor to object's attributedTo if missing
	ctx.Step(`^a Create for an Object without AttributedTo$`, o.aCreateForAnObjectWithoutAttributedTo)
	ctx.Step(`^the Object has the AttributedTo as the Activity Actor$`, o.theObjectHasTheAttributedToAsTheActivityActor)

	// NOTE(marius): Update
	ctx.Step(`^an Update with an Object containing only partial data$`, o.anUpdateWithAnObjectContainingOnlyPartialData)
	ctx.Step(`^an Update with an unauthorized Actor$`, o.anUpdateWithAnUnauthorizedActor)
	ctx.Step(`^no properties of the Object are unset$`, o.noPropertiesOfTheObjectAreUnset)
	ctx.Step(`^there is a validation error$`, o.thereIsAValidationError)
}

func (o *outboxContentManagementTests) aCreateAndObjectPairWithToBtoCcBccAndAudience() error {
	o.st = Storage(WithLocalMocks(Author))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Create(
		build.WithActor(Author.ID),
		build.WithTo(vocab.IRI("http://example.com/to-a")),
		build.WithCC(vocab.IRI("http://example.com/cc-a")),
		build.WithBto(vocab.IRI("http://example.com/bto-a")),
		build.WithBCC(vocab.IRI("http://example.com/bcc-a")),
		build.WithAudience(vocab.PublicNS),
		build.WithObject(build.Note(
			build.WithID("http://example.com"),
			build.WithContent("Lorem ipsum"),
			build.WithTo(vocab.IRI("http://example.com/to-o")),
			build.WithCC(vocab.IRI("http://example.com/cc-o")),
			build.WithBto(vocab.IRI("http://example.com/bto-o")),
			build.WithBCC(vocab.IRI("http://example.com/bcc-o")),
			build.WithAudience(vocab.PublicNS),
		)),
	)

	return nil
}

func (o *outboxContentManagementTests) objectAndActivityHaveMergedToBtoCcBccAndAudience() error {
	p := o.mp.InitP(processing.WithStorage(o.st))
	got, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return fmt.Errorf("received error: %w", err)
	}
	expectedTo := vocab.ItemCollection{vocab.IRI("http://example.com/to-a"), vocab.IRI("http://example.com/to-o")}
	expectedBto := vocab.ItemCollection{vocab.IRI("http://example.com/bto-a"), vocab.IRI("http://example.com/bto-o")}
	expectedCC := vocab.ItemCollection{vocab.IRI("http://example.com/cc-a"), vocab.IRI("http://example.com/cc-o")}
	expectedBCC := vocab.ItemCollection{vocab.IRI("http://example.com/bcc-a"), vocab.IRI("http://example.com/bcc-o")}

	errNotEqual := func(which, prop, diff string) error {
		return fmt.Errorf("%s's %s recipients property are not equal: %s", which, prop, diff)
	}

	return vocab.OnActivity(got, func(act *vocab.Activity) error {
		if !cmp.Equal(expectedTo, act.To) {
			return errNotEqual("activity", "to", cmp.Diff(expectedTo, act.To))
		}
		if !cmp.Equal(expectedCC, act.CC) {
			return errNotEqual("activity", "cc", cmp.Diff(expectedTo, act.CC))
		}
		if !cmp.Equal(expectedBto, act.Bto) {
			return errNotEqual("activity", "bto", cmp.Diff(expectedTo, act.Bto))
		}
		if !cmp.Equal(expectedBCC, act.BCC) {
			return errNotEqual("activity", "bcc", cmp.Diff(expectedTo, act.BCC))
		}
		return vocab.OnObject(act.Object, func(ob *vocab.Object) error {
			if !cmp.Equal(expectedTo, ob.To) {
				return errNotEqual("object", "to", cmp.Diff(expectedTo, ob.To))
			}
			if !cmp.Equal(expectedCC, ob.CC) {
				return errNotEqual("object", "cc", cmp.Diff(expectedTo, ob.CC))
			}
			if !cmp.Equal(expectedBto, ob.Bto) {
				return errNotEqual("object", "bto", cmp.Diff(expectedTo, ob.Bto))
			}
			if !cmp.Equal(expectedBCC, ob.BCC) {
				return errNotEqual("object", "bcc", cmp.Diff(expectedTo, ob.BCC))
			}
			return nil
		})
	})
}

func (o *outboxContentManagementTests) aCreateForAnObjectWithoutAttributedTo() error {
	o.st = Storage(WithLocalMocks(Author))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Create(
		build.WithActor(AuthorID),
		build.WithObject(build.Note(
			build.WithID("http://example.com"),
			build.WithContent("Lorem ipsum"),
			build.WithAudience(vocab.PublicNS),
		)),
	)
	return nil
}

func (o *outboxContentManagementTests) theObjectHasTheAttributedToAsTheActivityActor() error {
	p := o.mp.InitP(processing.WithStorage(o.st))
	got, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return fmt.Errorf("received error: %w", err)
	}

	return vocab.OnActivity(got, func(act *vocab.Activity) error {
		actorID := act.Actor.GetLink()
		return vocab.OnObject(act.Object, func(ob *vocab.Object) error {
			if !cmp.Equal(ob.AttributedTo, actorID) {
				return fmt.Errorf("object's AttributedTo different than activity's Actor: %s", cmp.Diff(ob.AttributedTo, actorID))
			}
			return nil
		})
	})
}

func Test_Outbox_Create(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"outbox-create.feature"}

	c := new(outboxContentManagementTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Outbox Create",
		ScenarioInitializer: c.initScenario,
		Options:             &opts,
	}.Run()
}

func Test_Outbox_Create_Old(t *testing.T) {
	mp := MockPlumbing{}
	st := Storage(WithLocalMocks(Author))
	P := mp.InitP(processing.WithStorage(st))

	tests := TestSuite{
		Name: "Create",
		Tests: []RunnableTest{
			TestPair{
				Name:       "Actor gets copied to Object.AttributedTo",
				Author:     *Author,
				ReceivedIn: vocab.Outbox.IRI(Author),
				Activity: build.Create(
					build.WithActor(Author.ID),
					build.WithObject(build.Note(build.WithName("Lorem Ipsum"))),
				),
				TestFns: TestAssertions{
					HasActor(HasID(Author.ID)),
					RequestObject(HasAttributedTo(Author.ID)),
				},
			},
			TestPair{
				Name:       "Merge Recipients To/CC/Bto/BCC",
				Author:     *Author,
				ReceivedIn: vocab.Outbox.IRI(Author),
				Activity: build.Create(
					build.WithType(vocab.CreateType),
					build.WithActor(Author.ID),
					build.WithTo(vocab.IRI("http://example.com/to/a1")),
					build.WithCC(vocab.IRI("http://example.com/cc/a1")),
					build.WithBto(vocab.IRI("http://example.com/bto/a1")),
					build.WithBCC(vocab.IRI("http://example.com/bcc/a1")),
					build.WithObject(build.Group(
						build.WithType(vocab.GroupType),
						build.WithName("Lorem"),
						build.WithTo(vocab.IRI("http://example.com/to/o1")),
						build.WithCC(vocab.IRI("http://example.com/cc/o1")),
						build.WithBto(vocab.IRI("http://example.com/bto/o1")),
						build.WithBCC(vocab.IRI("http://example.com/bcc/o1")),
						build.WithPreferredUsername("lipsum"),
					)),
				),
				TestFns: TestAssertions{
					HasTo(vocab.IRIs{"http://example.com/to/a1", "http://example.com/to/o1"}),
					HasBto(vocab.IRIs{"http://example.com/bto/a1", "http://example.com/bto/o1"}),
					HasCC(vocab.IRIs{"http://example.com/cc/a1", "http://example.com/cc/o1"}),
					HasBCC(vocab.IRIs{"http://example.com/bcc/a1", "http://example.com/bcc/o1"}),
					RequestObject(
						HasTo(vocab.IRIs{"http://example.com/to/a1", "http://example.com/to/o1"}),
						HasBto(vocab.IRIs{"http://example.com/bto/a1", "http://example.com/bto/o1"}),
						HasCC(vocab.IRIs{"http://example.com/cc/a1", "http://example.com/cc/o1"}),
						HasBCC(vocab.IRIs{"http://example.com/bcc/a1", "http://example.com/bcc/o1"}),
					),
				},
			},
		},
	}

	t.Run(tests.Label(), tests.TestFn(P))
}

func (o *outboxContentManagementTests) anUpdateWithAnObjectContainingOnlyPartialData() error {
	noteID := RootID.AddPath("to-update")
	willBeUpdated := build.Note(
		build.WithID(noteID),
		build.WithName("Lipsum name"),
		build.WithSummary("Lipsum summary"),
		build.WithContent("<p>Lorem ipsum dolor sic amet</p>"),
		build.WithAttributedTo(AuthorID),
		build.WithMediaType("text/html"),
		build.WithPublished(time.Date(1984, time.February, 29, 0, 0, 0, 0, time.UTC)),
		build.WithReplies(),
		build.WithTo(vocab.ItemCollection{vocab.PublicNS}),
		build.WithSourceContent("Lorem ipsum dolor sic amet"),
		build.WithSourceMediaType("text/markdown"),
	)

	o.st = Storage(WithLocalMocks(Author, willBeUpdated))
	updated1 := build.Note(build.WithID(noteID), build.WithContent("<p>Content</p>"))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Update(build.WithActor(Author.ID), build.WithObject(updated1))

	return nil
}

func (o *outboxContentManagementTests) noPropertiesOfTheObjectAreUnset() error {
	p := o.mp.InitP(processing.WithStorage(o.st))
	got, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return fmt.Errorf("received error: %w", err)
	}

	_ = o.mp.PopTime() // NOTE(marius): pop the Update activity published time
	expectedUpdated := build.Note(
		build.WithID(RootID.AddPath("to-update")),
		build.WithName("Lipsum name"),
		build.WithSummary("Lipsum summary"),
		build.WithContent("<p>Content</p>"),
		build.WithAttributedTo(AuthorID),
		build.WithMediaType("text/html"),
		build.WithPublished(time.Date(1984, time.February, 29, 0, 0, 0, 0, time.UTC)),
		build.WithUpdated(o.mp.PopTime()),
		build.WithReplies(),
		build.WithTo(vocab.ItemCollection{vocab.PublicNS}),
		build.WithSourceContent("Lorem ipsum dolor sic amet"),
		build.WithSourceMediaType("text/markdown"),
	)
	return vocab.OnActivity(got, func(act *vocab.Activity) error {
		if !cmp.Equal(act.Object, expectedUpdated) {
			return fmt.Errorf("expected object different than received: %s", cmp.Diff(expectedUpdated, act.Object))
		}
		return nil
	})
}

func (o *outboxContentManagementTests) anUpdateWithAnUnauthorizedActor() error {
	localAuthor := vocab.IRI("http://example.com/~notDoe")
	noteID := localAuthor.AddPath("to-update")
	willBeUpdated := build.Note(
		build.WithID(noteID),
		build.WithAttributedTo(localAuthor),
		build.WithContent("<p>Lorem ipsum dolor sic amet</p>"),
		build.WithMediaType("text/html"),
	)

	o.st = Storage(WithLocalMocks(Author, willBeUpdated))
	updated1 := build.Note(build.WithID(noteID), build.WithContent("<p>Content</p>"))

	// NOTE(marius): a different actor to execute the Update than the attributedTo of the original
	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Update(build.WithActor(Author.ID), build.WithObject(updated1))

	return nil
}

func (o *outboxContentManagementTests) thereIsAValidationError() error {
	expectedErr := errors.Unauthorizedf("actor does not own the object being updated")
	p := o.mp.InitP(processing.WithStorage(o.st))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if cmp.Equal(err, expectedErr, EquateWeakErrors) {
		return nil
	}
	return fmt.Errorf("unexpected error received: %w", err)
}

func Test_Outbox_Update(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"outbox-update.feature"}

	c := new(outboxContentManagementTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Outbox Update",
		ScenarioInitializer: c.initScenario,
		Options:             &opts,
	}.Run()
}

func Test_Outbox_Update_Old(t *testing.T) {
	mp := MockPlumbing{}

	toUpdate := build.Note(
		build.WithID(RootID.AddPath("to-update")),
		build.WithName("Lipsum name"),
		build.WithSummary("Lipsum summary"),
		build.WithContent("<p>Lorem ipsum dolor sic amet</p>"),
		build.WithAttributedTo(AuthorID),
		build.WithMediaType("text/html"),
		build.WithPublished(time.Date(1984, time.February, 29, 0, 0, 0, 0, time.UTC)),
		build.WithReplies(),
		build.WithTo(vocab.ItemCollection{vocab.PublicNS}),
		build.WithSourceContent("Lorem ipsum dolor sic amet"),
		build.WithSourceMediaType("text/markdown"),
	)

	st := Storage(WithLocalMocks(Author, toUpdate))
	P := mp.InitP(processing.WithStorage(st))

	updated1 := toUpdate
	updated1.Content = vocab.DefaultLangValue("<p>Content</p>")

	tests := TestSuite{
		Name: "Update",
		Tests: []RunnableTest{
			TestPair{
				Name:       "Partial",
				Author:     *Author,
				ReceivedIn: vocab.Outbox.IRI(Author),
				Activity:   build.Update(build.WithActor(Author.ID), build.WithObject(updated1)),
				TestFns: TestAssertions{
					HasID(mp.ShiftIRI),
					HasType(vocab.UpdateType),
					WasPublished(mp.PopTime),
					RequestObject(
						WasUpdated(mp.PopTime),
						// Original name
						HasName(vocab.DefaultLangValue("Lipsum name")),
						// Original summary
						HasSummary(vocab.DefaultLangValue("Lipsum summary")),
						// Updated content
						HasContent(vocab.DefaultLangValue("<p>Content</p>")),
					),
				},
			},
		},
	}

	t.Run(tests.Label(), tests.TestFn(P))
}

func Test_Outbox_Delete(t *testing.T) {
	mp := MockPlumbing{}

	toDelete := build.Article(
		build.WithID(RootID.AddPath("to-delete")),
		build.WithName("Lipsum name"),
		build.WithSummary("Lipsum summary"),
		build.WithContent("<p>Lorem ipsum dolor sic amet</p>"),
		build.WithAttributedTo(AuthorID),
		build.WithMediaType("text/html"),
		build.WithPublished(time.Date(1984, time.February, 29, 0, 0, 0, 0, time.UTC)),
		build.WithReplies(),
		build.WithTo(vocab.PublicNS),
		build.WithSourceContent("Lorem ipsum dolor sic amet"),
		build.WithSourceMediaType("text/markdown"),
	)

	st := Storage(WithLocalMocks(Author, toDelete))
	P := mp.InitP(processing.WithStorage(st))

	tests := TestSuite{
		Name: "Delete",
		Tests: []RunnableTest{
			TestPair{
				Name:       "Article",
				Author:     *Author,
				ReceivedIn: vocab.Outbox.IRI(Author),
				Activity:   build.Delete(build.WithActor(Author.ID), build.WithObject(toDelete.ID)),
				TestFns: TestAssertions{
					HasID(mp.ShiftIRI),
					HasType(vocab.DeleteType),
					HasActor(HasID(Author.ID)),
					WasPublished(mp.PopTime),
					RequestObject(
						HasID(toDelete.ID),
						HasType(vocab.TombstoneType),
						HasFormerType(toDelete.Type),
						HasTo(toDelete.To),
						WasDeleted(mp.PopTime),
					),
					st.CheckObject(toDelete.ID, HasType(vocab.TombstoneType)),
				},
			},
		},
	}

	t.Run(tests.Label(), tests.TestFn(P))
}
