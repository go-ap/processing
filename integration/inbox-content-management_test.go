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

type inboxContentManagementTests struct {
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (i *inboxContentManagementTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)

	ctx.Step(`^a Create activity is received$`, i.aCreateActivityIsReceived)
	ctx.Step(`^processing is successful and object matches$`, i.processingIsSuccessfulAndObjectMatches)

	ctx.Step(`^an Update activity is received$`, i.anUpdateActivityIsReceived)
	ctx.Step(`^the object is fully replaced by the activity\'s object$`, i.theObjectIsFullyReplacedByTheActivitysObject)

	ctx.Step(`^an Update activity is received from a server authority other than the original's object$`, i.anUpdateActivityIsReceivedFromAServerAuthorityOtherThanTheOriginalsObject)
	ctx.Step(`^there is a validation error$`, i.thereIsAValidationError)

	ctx.Step(`^an Update activity with an actor that is not the author of the updated object is received$`, i.anUpdateActivityWithAnActorThatIsNotTheAuthorOfTheUpdatedObjectIsReceived)

	ctx.Step(`^a Delete activity is received$`, i.aDeleteActivityIsReceived)
	ctx.Step(`^processing is successful$`, i.processingIsSuccessful)

	ctx.Step(`^the object is replaced with a Tombstone$`, i.theObjectIsReplacedWithATombstone)
}

func (i *inboxContentManagementTests) aCreateActivityIsReceived() error {
	article := build.Article(
		build.WithID(RemoteAuthorID.AddPath("brand-new")),
		build.WithName("Lipsum name"),
		build.WithSummary("Lipsum summary"),
		build.WithContent("<p>Lorem ipsum dolor sic amet</p>"),
		build.WithMediaType("text/html"),
		build.WithReplies(),
		build.WithAudience(vocab.PublicNS),
		build.WithSourceContent("Lorem ipsum dolor sic amet"),
		build.WithSourceMediaType("text/markdown"),
	)

	i.Activity = build.Create(
		build.WithID(RemoteAuthor.ID.AddPath("create")),
		build.WithActor(RemoteAuthorID),
		build.WithObject(article),
	)

	i.Author = *RemoteAuthor
	i.ReceivedIn = vocab.Inbox.IRI(Author)

	return nil
}

func (i *inboxContentManagementTests) processingIsSuccessfulAndObjectMatches(ctx context.Context) error {
	t := godog.T(ctx)

	st := Storage(WithLocalMocks(Author))
	ms := Remote(
		WithClientActorKey(Author, ed2559Key),
		WithHandlers(Handler(RemoteAuthorID, ServeItemHandler(t, nil, RemoteAuthor))),
	)

	mp := MockPlumbing{}
	p := mp.InitP(processing.WithStorage(st), WithRemote(ms))

	act, err := p.ProcessActivity(i.Activity, i.Author, i.ReceivedIn)
	if err != nil {
		t.Errorf("Unexpected error received: %v", err)
	}

	checks := BuildAssertions(
		HasID(RemoteAuthor.ID.AddPath("create")),
		HasType(vocab.CreateType),
		WasPublished(mp.PopTime),
		RequestObject(
			HasID(RemoteAuthorID.AddPath("brand-new")),
			HasType(vocab.ArticleType),
			WasPublished(mp.PopTime),
			HasName("Lipsum name"),
			HasSummary("Lipsum summary"),
			HasContent("<p>Lorem ipsum dolor sic amet</p>"),
		),
	)

	return vocab.OnActivity(act, func(act *vocab.Activity) error {
		checks.Run(t, act)
		return nil
	})
}

func (i *inboxContentManagementTests) anUpdateActivityIsReceived() error {
	updated := build.Object(
		build.WithID(RemoteAuthorID.AddPath("to-update")),
		build.WithContent("<p>Content</p>"),
	)

	i.Activity = build.Update(
		build.WithID(RemoteAuthor.ID.AddPath("upd-1")),
		build.WithActor(RemoteAuthorID),
		build.WithObject(updated),
	)

	i.Author = *RemoteAuthor
	i.ReceivedIn = vocab.Inbox.IRI(Author)

	return nil
}

func (i *inboxContentManagementTests) theObjectIsFullyReplacedByTheActivitysObject(ctx context.Context) error {
	pubDate := time.Date(1984, time.February, 29, 0, 0, 0, 0, time.UTC)

	originalID := RemoteAuthorID.AddPath("to-update")
	original := build.Note(
		build.WithID(originalID),
		build.WithName("Lipsum name"),
		build.WithSummary("Lipsum summary"),
		build.WithContent("<p>Lorem ipsum dolor sic amet</p>"),
		build.WithAttributedTo(RemoteAuthorID),
		build.WithMediaType("text/html"),
		build.WithPublished(pubDate),
		build.WithReplies(),
		build.WithAudience(vocab.PublicNS),
		build.WithSourceContent("Lorem ipsum dolor sic amet"),
		build.WithSourceMediaType("text/markdown"),
	)

	t := godog.T(ctx)

	st := Storage(WithLocalMocks(Author))
	ms := Remote(
		WithClientActorKey(Author, ed2559Key),
		WithHandlers(
			Handler(RemoteAuthorID, ServeItemHandler(t, nil, RemoteAuthor)),
			Handler(originalID, ServeItemHandler(t, nil, original)),
		),
	)

	mp := MockPlumbing{}
	p := mp.InitP(processing.WithStorage(st), WithRemote(ms))

	act, err := p.ProcessActivity(i.Activity, i.Author, i.ReceivedIn)
	if err != nil {
		t.Errorf("Unexpected error received: %v", err)
	}

	checks := BuildAssertions(
		HasID(RemoteAuthor.ID.AddPath("upd-1")),
		HasType(vocab.UpdateType),
		WasPublished(mp.PopTime),
		RequestObject(
			HasID(original.ID),
			HasType(nil),
			WasPublished(time.Time{}),
			WasUpdated(time.Time{}),
			// Name was replaced with nil
			HasName[vocab.NaturalLanguageValues](nil),
			// Summary was replaced with nil
			HasSummary[vocab.NaturalLanguageValues](nil),
			// Content was updated
			HasContent(vocab.DefaultLangValue("<p>Content</p>")),
		),
	)

	return vocab.OnActivity(act, func(act *vocab.Activity) error {
		checks.Run(t, act)
		return nil
	})
}

func (i *inboxContentManagementTests) anUpdateActivityIsReceivedFromAServerAuthorityOtherThanTheOriginalsObject(ctx context.Context) error {
	toUpdate := build.Object(
		build.WithID("http://example.com/to-update"),
		build.WithContent("<p>Content</p>"),
	)

	i.Activity = build.Update(
		build.WithID("http://social.example.com/update"),
		build.WithActor(RemoteAuthorID),
		build.WithObject(toUpdate),
	)

	i.Author = *RemoteAuthor
	i.ReceivedIn = vocab.Inbox.IRI(Author)

	return nil
}

func (i *inboxContentManagementTests) anUpdateActivityWithAnActorThatIsNotTheAuthorOfTheUpdatedObjectIsReceived(ctx context.Context) error {
	toUpdate := build.Object(
		build.WithID("http://example.com/to-update"),
		build.WithContent("<p>Content</p>"),
	)

	i.Activity = build.Update(
		build.WithID("http://social.example.com/update"),
		build.WithActor(RemoteAuthorID),
		build.WithObject(toUpdate),
	)

	i.Author = *RemoteAuthor
	i.ReceivedIn = vocab.Inbox.IRI(Author)
	return nil
}

func (i *inboxContentManagementTests) thereIsAValidationError(ctx context.Context) error {
	t := godog.T(ctx)

	toUpdate := build.Object(
		build.WithID("http://example.com/to-update"),
		build.WithContent("<p>Content</p>"),
		build.WithAudience(vocab.PublicNS),
		// NOTE(marius): the author of the object is not the RemoteAuthor
		build.WithAttributedTo(vocab.IRI("http://social.example.com/~alice")),
	)

	st := Storage(WithLocalMocks(Author))
	ms := Remote(
		WithHandlers(
			Handler(toUpdate.ID, ServeItemHandler(t, nil, toUpdate)),
			Handler(RemoteAuthorID, ServeItemHandler(t, nil, RemoteAuthor)),
		),
	)

	mp := MockPlumbing{}
	p := mp.InitP(processing.WithStorage(st), WithRemote(ms))

	expectedErr := errors.Unauthorizedf("actor does not own the object being updated")
	_, err := p.ProcessActivity(i.Activity, i.Author, i.ReceivedIn)
	if cmp.Equal(err, expectedErr, EquateWeakErrors) {
		return nil
	}
	return fmt.Errorf("expected error and none received: %v", err)
}

func (i *inboxContentManagementTests) aDeleteActivityIsReceived() error {
	toDeleteID := RemoteRootID.AddPath("to-delete")
	i.Activity = build.Delete(
		build.WithID("http://example.com/delete"),
		build.WithActor(RemoteAuthorID),
		build.WithObject(toDeleteID),
	)

	i.Author = *RemoteAuthor
	i.ReceivedIn = vocab.Inbox.IRI(Author)

	return nil
}

func (i *inboxContentManagementTests) theObjectIsReplacedWithATombstone(ctx context.Context) error {
	t := godog.T(ctx)

	toDeleteID := RemoteRootID.AddPath("to-delete")
	toDelete := build.Note(
		build.WithID(toDeleteID),
		build.WithAudience(vocab.PublicNS),
		build.WithContent("<p>Content</p>"),
		build.WithAttributedTo(RemoteAuthorID),
	)

	st := Storage(WithLocalMocks(Author, RemoteAuthor))
	ms := Remote(
		WithClientActorKey(Author, ed2559Key),
		WithHandlers(
			Handler(toDeleteID, ServeItemHandler(t, nil, toDelete)),
			Handler(toDeleteID, ServeItemHandler(t, nil, toDelete)),
		),
	)

	mp := MockPlumbing{}
	p := mp.InitP(processing.WithStorage(st), WithRemote(ms))

	act, err := p.ProcessActivity(i.Activity, i.Author, i.ReceivedIn)
	if err != nil {
		t.Errorf("Unexpected error received: %v", err)
	}

	checks := BuildAssertions(
		HasID(RemoteRootID.AddPath("delete")),
		HasType(vocab.DeleteType),
		RequestObject(
			HasID(toDelete.ID),
			HasType(vocab.TombstoneType),
			HasFormerType(vocab.NoteType),
		),
	)

	checks.Run(t, act)
	return nil
}

func (i *inboxContentManagementTests) processingIsSuccessful(ctx context.Context) error {
	t := godog.T(ctx)

	toDeleteID := RemoteRootID.AddPath("to-delete")
	toDelete := build.Note(
		build.WithID(toDeleteID),
		build.WithAudience(vocab.PublicNS),
		build.WithContent("<p>Content</p>"),
		build.WithAttributedTo(RemoteAuthorID),
	)

	st := Storage(WithLocalMocks(Author, RemoteAuthor, toDelete))
	ms := Remote()

	mp := MockPlumbing{}
	p := mp.InitP(processing.WithStorage(st), WithRemote(ms))

	_, err := p.ProcessActivity(i.Activity, i.Author, i.ReceivedIn)
	if err != nil {
		t.Errorf("Unexpected error received: %v", err)
	}

	return nil
}

func Test_Inbox_ContentManagement(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"inbox-content-management.feature"}

	i := new(inboxContentManagementTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Inbox Content Management Activities",
		ScenarioInitializer: i.initScenario,
		Options:             &opts,
	}.Run()
}

func Test_Inbox_Update(t *testing.T) {
	mp := MockPlumbing{}

	original := build.Note(
		build.WithID(RootID.AddPath("to-update")),
		build.WithName("Lipsum name"),
		build.WithSummary("Lipsum summary"),
		build.WithContent("<p>Lorem ipsum dolor sic amet</p>"),
		build.WithAttributedTo(RemoteAuthorID),
		build.WithMediaType("text/html"),
		build.WithPublished(time.Date(1984, time.February, 29, 0, 0, 0, 0, time.UTC)),
		build.WithReplies(),
		build.WithTo(vocab.PublicNS),
		build.WithSourceContent("Lorem ipsum dolor sic amet"),
		build.WithSourceMediaType("text/markdown"),
	)

	updated := build.Object(
		build.WithID(original.ID),
		build.WithAttributedTo(RemoteAuthorID),
		build.WithContent("<p>Content</p>"),
	)

	updateID := vocab.IRI("http://example.com/~jdoe/upd-1")
	update := build.Update(
		build.WithID(updateID),
		build.WithObject(updated),
	)
	st := Storage(WithLocalMocks(Author, original))
	ms := Remote(WithHandlers(Handler(RemoteAuthorID, ServeItemHandler(t, nil, RemoteAuthor))))
	P := mp.InitP(processing.WithStorage(st), WithRemote(ms))

	tests := TestSuite{
		Name: "Update",
		Tests: []RunnableTest{
			TestPair{
				Name:       "Completely-Replace",
				Author:     *RemoteAuthor,
				ReceivedIn: vocab.Inbox.IRI(Author),
				Activity:   update,
				TestFns: TestAssertions{
					HasID(updateID),
					HasType(vocab.UpdateType),
					WasPublished(mp.PopTime),
					RequestObject(
						HasID(original.ID),
						HasType(nil),
						WasUpdated(mp.PopTime),
						// Name was replaced with nil
						HasName[vocab.NaturalLanguageValues](nil),
						// Summary was replaced with nil
						HasSummary[vocab.NaturalLanguageValues](nil),
						// Content was updated
						HasContent(vocab.DefaultLangValue("<p>Content</p>")),
					),
				},
			},
		},
	}

	t.Run(tests.Label(), tests.TestFn(P))
}
