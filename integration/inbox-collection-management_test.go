package integration

import (
	"context"
	"testing"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
	"github.com/go-ap/processing"
)

type inboxCollectionManagementTests struct {
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (i *inboxCollectionManagementTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)

	ctx.Step(`^an Add activity is received$`, i.anAddActivityIsReceived)
	ctx.Step(`^the object will be added to the Target collection$`, i.theObjectWillBeAddedToTheTargetCollection)

	ctx.Step(`^a Remove activity is received$`, i.aRemoveActivityIsReceived)
	ctx.Step(`^the object will be removed from the Origin collection$`, i.theObjectWillBeRemovedFromTheOriginCollection)
}

func (i *inboxCollectionManagementTests) anAddActivityIsReceived() error {
	// NOTE(marius): we use a local collection for a remote activity
	targetID := RootID.AddPath("target-collection")
	object := build.Link(
		build.WithID(RemoteRootID.AddPath("link")),
		build.WithHref("http://example.com"),
	)
	i.Activity = build.Add(
		build.WithID(RemoteAuthorID.AddPath("add-to-remote-target")),
		build.WithActor(RemoteAuthorID),
		build.WithObject(object),
		build.WithTarget(targetID),
	)
	i.Author = *RemoteAuthor
	i.ReceivedIn = vocab.Inbox.IRI(Author)
	return nil
}

func (i *inboxCollectionManagementTests) theObjectWillBeAddedToTheTargetCollection(ctx context.Context) error {
	t := godog.T(ctx)

	targetID := RootID.AddPath("target-collection")
	target := build.Collection(build.WithID(targetID))

	st := Storage(WithLocalMocks(Author, target))
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

	targetLoad, err := st.Load(targetID)
	if err != nil {
		t.Errorf("Unexpected error received when loading local target: %v", err)
	}

	object := build.Link(
		build.WithID(RemoteRootID.AddPath("link")),
		build.WithHref("http://example.com"),
	)
	checks := BuildAssertions(
		HasID(targetID),
		HasType(vocab.CollectionType),
		HasExactItems(object),
		HasTotalItems(1),
	)

	checks.Run(t, targetLoad)
	return nil
}

func (i *inboxCollectionManagementTests) aRemoveActivityIsReceived() error {
	// NOTE(marius): we use a local collection for a remote activity
	targetID := RootID.AddPath("target-collection")
	objectID := RemoteRootID.AddPath("link")

	i.Activity = build.Remove(
		build.WithID(RemoteAuthorID.AddPath("remove-from-remote-target")),
		build.WithActor(RemoteAuthorID),
		build.WithObject(objectID),
		build.WithTarget(targetID),
	)
	i.Author = *RemoteAuthor
	i.ReceivedIn = vocab.Inbox.IRI(Author)
	return nil
}

func (i *inboxCollectionManagementTests) theObjectWillBeRemovedFromTheOriginCollection(ctx context.Context) error {
	t := godog.T(ctx)

	objectID := RemoteRootID.AddPath("link")
	object := build.Link(
		build.WithID(objectID),
		build.WithHref("http://example.com"),
	)

	targetID := RootID.AddPath("target-collection")
	target := build.Collection(
		build.WithID(targetID),
		build.WithTotalItems(1),
		build.WithItems(object),
	)

	st := Storage(WithLocalMocks(Author, target))
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

	targetLoad, err := st.Load(targetID)
	if err != nil {
		t.Errorf("Unexpected error received when loading local target: %v", err)
	}

	checks := BuildAssertions(
		HasID(targetID),
		HasType(vocab.CollectionType),
		DoesNotHaveItem(object),
		HasTotalItems(0),
	)

	checks.Run(t, targetLoad)
	return nil
}

func Test_Inbox_CollectionManagement(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"inbox-collection-management.feature"}

	i := new(inboxCollectionManagementTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Inbox Collection Management Activities",
		ScenarioInitializer: i.initScenario,
		Options:             &opts,
	}.Run()
}
