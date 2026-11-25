package integration

import (
	"context"
	"testing"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
	"github.com/go-ap/processing"
)

type inboxNegateTests struct {
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (i *inboxNegateTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)

	ctx.Step(`^an Undo activity is received$`, i.anUndoActivityIsReceived)
	ctx.Step(`^processing is successful$`, i.processingIsSuccessful)
}

func (i *inboxNegateTests) anUndoActivityIsReceived() error {
	i.Activity = build.Undo(
		build.WithID(RemoteAuthorID.AddPath("undo-create")),
		build.WithObject(RemoteAuthorID.AddPath("create")),
		build.WithActor(RemoteAuthorID),
	)
	i.Author = *RemoteAuthor
	i.ReceivedIn = vocab.Inbox.IRI(Author)
	return nil
}

func (i *inboxNegateTests) processingIsSuccessful(ctx context.Context) error {
	t := godog.T(ctx)

	createID := RemoteAuthorID.AddPath("create")
	create := build.Create(
		build.WithID(createID),
		build.WithActor(RemoteAuthorID),
		build.WithTo(AuthorID),
		build.WithObject(build.Link(
			build.WithID(RemoteAuthorID.AddPath("link")),
			build.WithHref("http://example.com"),
		)),
	)
	st := Storage(WithLocalMocks(Author))
	ms := Remote(
		WithClientActorKey(RemoteAuthor, rsaKey),
		WithHandlers(
			Handler(RemoteAuthorID, ServeItemHandler(t, nil, RemoteAuthor)),
			Handler(createID, ServeItemHandler(t, Auth(RemoteAuthor, rsaKey), create)),
		),
	)

	allowAllActors := func(actor, object vocab.Item) error {
		return nil
	}
	mp := MockPlumbing{}
	p := mp.InitP(processing.WithStorage(st), WithRemote(ms), processing.WithAuthorizationCheckFn(allowAllActors))
	_, err := p.ProcessActivity(i.Activity, i.Author, i.ReceivedIn)
	if err != nil {
		t.Errorf("Unexpected error received: %v", err)
	}

	return ms.Success(t)
}

func Test_Inbox_Negate(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"inbox-negate.feature"}

	i := new(inboxNegateTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Inbox Negate",
		ScenarioInitializer: i.initScenario,
		Options:             &opts,
	}.Run()
}
