package integration

import (
	"context"
	"testing"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
	"github.com/go-ap/processing"
)

type inboxNotificationsTests struct {
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (i *inboxNotificationsTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^receiving an Announce activity$`, i.receivingAnAnnounceActivity)
	ctx.Step(`^the processing is successful and the shares collection contains the activity$`, i.theProcessingIsSuccessfulAndTheSharesCollectionContainsTheActivity)
}

func (i *inboxNotificationsTests) receivingAnAnnounceActivity() error {
	objectID := AuthorID.AddPath("to-share")
	i.Activity = build.Announce(
		build.WithID(RemoteAuthorID.AddPath("announce")),
		build.WithActor(RemoteAuthorID),
		build.WithObject(objectID),
	)
	i.Author = *RemoteAuthor
	i.ReceivedIn = vocab.Inbox.IRI(Author)
	return nil
}

func (i *inboxNotificationsTests) theProcessingIsSuccessfulAndTheSharesCollectionContainsTheActivity(ctx context.Context) error {
	t := godog.T(ctx)

	objectID := AuthorID.AddPath("to-share")
	sharesID := objectID.AddPath("shares")

	object := build.Note(
		build.WithID(objectID),
		build.WithName("something to like"),
		build.WithShares(sharesID),
	)
	shares := build.Collection(build.WithID(sharesID))

	st := Storage(WithLocalMocks(Author, shares, object))
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

	sharesLoad, err := st.Load(sharesID)
	if err != nil {
		t.Errorf("Unexpected error received when loading local shares collection: %v", err)
	}

	checks := BuildAssertions(
		HasID(sharesID),
		HasType(vocab.CollectionType),
		HasExactItems(i.Activity),
		HasTotalItems(1),
	)

	checks.Run(t, sharesLoad)
	return nil
}

func Test_Inbox_Notifications(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"inbox-notifications.feature"}

	i := new(inboxNotificationsTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Inbox Notifications",
		ScenarioInitializer: i.initScenario,
		Options:             &opts,
	}.Run()
}
