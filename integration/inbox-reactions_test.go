package integration

import (
	"context"
	"testing"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
	"github.com/go-ap/processing"
)

type inboxReactionsTests struct {
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (i *inboxReactionsTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^receiving a Like activity$`, i.receivingALikeActivity)
	ctx.Step(`^the processing is successful and the likes collection contains the activity$`, i.theProcessingIsSuccessfulAndTheLikesCollectionContainsTheActivity)
}

func (i *inboxReactionsTests) receivingALikeActivity() error {
	objectID := AuthorID.AddPath("to-like")
	i.Activity = build.Like(
		build.WithID(RemoteAuthorID.AddPath("like")),
		build.WithActor(RemoteAuthorID),
		build.WithObject(objectID),
	)
	i.Author = *RemoteAuthor
	i.ReceivedIn = vocab.Inbox.IRI(Author)
	return nil
}

func (i *inboxReactionsTests) theProcessingIsSuccessfulAndTheLikesCollectionContainsTheActivity(ctx context.Context) error {
	t := godog.T(ctx)

	objectID := AuthorID.AddPath("to-like")
	likesID := objectID.AddPath("likes")

	object := build.Note(
		build.WithID(objectID),
		build.WithName("something to like"),
		build.WithLikes(likesID),
	)
	likes := build.Collection(build.WithID(likesID))

	st := Storage(WithLocalMocks(Author, likes, object))
	ms := Remote(WithHandlers(Handler(RemoteAuthorID, ServeItemHandler(t, nil, RemoteAuthor))))

	allowAllActors := func(actor, object vocab.Item) error {
		return nil
	}
	mp := MockPlumbing{}
	p := mp.InitP(processing.WithStorage(st), WithRemote(ms), processing.WithAuthorizationCheckFn(allowAllActors))
	_, err := p.ProcessActivity(i.Activity, i.Author, i.ReceivedIn)
	if err != nil {
		t.Errorf("Unexpected error received: %v", err)
	}

	if err = ms.Success(t); err != nil {
		return err
	}

	likesLoad, err := st.Load(likesID)
	if err != nil {
		t.Errorf("Unexpected error received when loading local likes collection: %v", err)
	}

	checks := BuildAssertions(
		HasID(likesID),
		HasType(vocab.CollectionType),
		HasExactItems(i.Activity),
		HasTotalItems(1),
	)

	checks.Run(t, likesLoad)
	return nil
}

func Test_Inbox_Reactions(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"inbox-reactions.feature"}

	i := new(inboxReactionsTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Inbox Reactions",
		ScenarioInitializer: i.initScenario,
		Options:             &opts,
	}.Run()
}
