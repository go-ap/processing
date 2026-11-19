package integration

import (
	"testing"
	"time"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
	"github.com/go-ap/processing"
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
	ctx.Step(`^a Delete activity is received$`, i.aDeleteActivityIsReceived)
	ctx.Step(`^an Update activity is received$`, i.anUpdateActivityIsReceived)
	ctx.Step(`^an Update activity is received from a server authority other than the original's object$`, i.anUpdateActivityIsReceivedFromAServerAuthorityOtherThanTheOriginalsObject)
	ctx.Step(`^an Update activity with an actor that is not the author of the updated object is received$`, i.anUpdateActivityWithAnActorThatIsNotTheAuthorOfTheUpdatedObjectIsReceived)
	ctx.Step(`^processing is successful$`, i.processingIsSuccessful)
	ctx.Step(`^the object is fully replaced by the activity\'s object$`, i.theObjectIsFullyReplacedByTheActivitysObject)
	ctx.Step(`^the object is replaced with a Tombstone$`, i.theObjectIsReplacedWithATombstone)
	ctx.Step(`^there is a validation error$`, i.thereIsAValidationError)
}

func (i *inboxContentManagementTests) aCreateActivityIsReceived() error {
	return godog.ErrPending
}

func (i *inboxContentManagementTests) aDeleteActivityIsReceived() error {
	return godog.ErrPending
}

func (i *inboxContentManagementTests) anUpdateActivityIsReceived() error {
	return godog.ErrPending
}

func (i *inboxContentManagementTests) anUpdateActivityIsReceivedFromAServerAuthorityOtherThanTheOriginalsObject() error {
	return godog.ErrPending
}

func (i *inboxContentManagementTests) anUpdateActivityWithAnActorThatIsNotTheAuthorOfTheUpdatedObjectIsReceived() error {
	return godog.ErrPending
}

func (i *inboxContentManagementTests) processingIsSuccessful() error {
	return godog.ErrPending
}

func (i *inboxContentManagementTests) theObjectIsFullyReplacedByTheActivitysObject() error {
	return godog.ErrPending
}

func (i *inboxContentManagementTests) theObjectIsReplacedWithATombstone() error {
	return godog.ErrPending
}

func (i *inboxContentManagementTests) thereIsAValidationError() error {
	return godog.ErrPending
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
		build.WithAttributedTo(AuthorID),
		build.WithMediaType("text/html"),
		build.WithPublished(time.Date(1984, time.February, 29, 0, 0, 0, 0, time.UTC)),
		build.WithReplies(),
		build.WithTo(vocab.PublicNS),
		build.WithSourceContent("Lorem ipsum dolor sic amet"),
		build.WithSourceMediaType("text/markdown"),
	)

	updated := build.Object(
		build.WithID(original.ID),
		build.WithContent("<p>Content</p>"),
	)

	updateID := vocab.IRI("http://example.com/~jdoe/upd-1")
	update := build.Update(
		build.WithID(updateID),
		build.WithActor(RemoteAuthorID),
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
