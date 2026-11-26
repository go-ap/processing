package integration

import (
	"context"
	"testing"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
	"github.com/go-ap/processing"
)

type inboxRelationshipTests struct {
	Activity vocab.Item
	Author   vocab.Actor

	ReceivedIn vocab.IRI
	Receiver   *vocab.Actor

	st *LocalStorage
}

func (i *inboxRelationshipTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)

	ctx.Step(`^receiving a Follow activity in a Group actor's inbox$`, i.receivingAFollowActivityInAGroupActorsInbox)
	ctx.Step(`^it gets accepted$`, i.itGetsAccepted)
	ctx.Step(`^we add the Follow's activity actor to the receiver's following collection$`, i.weAddTheFollowsActivityActorToTheReceiversFollowingCollection)

	ctx.Step(`^receiving a Follow activity$`, i.receivingAFollowActivity)
	ctx.Step(`^we respond with an Accept activity$`, i.weRespondWithAnAcceptActivity)

	ctx.Step(`^we send an Accept activity in response$`, i.weSendAnAcceptActivityInResponse)
}

func (i *inboxRelationshipTests) receivingAFollowActivityInAGroupActorsInbox() error {
	groupActorID := AuthorID.AddPath("group")
	i.Receiver = build.Group(
		build.WithID(groupActorID),
		build.WithName("something to like"),
		build.WithInbox(),
		build.WithFollowers(),
	)
	followers := build.Collection(build.WithID(vocab.Followers.IRI(i.Receiver)))

	i.st = Storage(WithLocalMocks(Author, followers, i.Receiver))
	i.Activity = build.Follow(
		build.WithID(RemoteAuthorID.AddPath("follow")),
		build.WithActor(RemoteAuthorID),
		build.WithObject(groupActorID),
	)
	i.Author = *RemoteAuthor
	i.ReceivedIn = vocab.Inbox.IRI(groupActorID)

	return nil
}

func (i *inboxRelationshipTests) itGetsAccepted(ctx context.Context) error {
	t := godog.T(ctx)

	ms := Remote(WithHandlers(Handler(RemoteAuthorID, ServeItemHandler(t, nil, RemoteAuthor))))

	mp := MockPlumbing{}
	p := mp.InitP(processing.WithStorage(i.st), WithRemote(ms))
	_, err := p.ProcessActivity(i.Activity, i.Author, i.ReceivedIn)
	if err != nil {
		t.Errorf("Unexpected error received: %v", err)
	}

	return ms.Success(t)
}

func (i *inboxRelationshipTests) weAddTheFollowsActivityActorToTheReceiversFollowingCollection(ctx context.Context) error {
	t := godog.T(ctx)

	followersID := vocab.Followers.IRI(i.Receiver)
	followersLoad, err := i.st.Load(followersID)
	if err != nil {
		t.Errorf("Unexpected error received when loading local followers collection: %v", err)
	}

	checks := BuildAssertions(
		HasID(followersID),
		HasType(vocab.CollectionType),
		HasExactItems(RemoteAuthor),
		HasTotalItems(1),
	)

	checks.Run(t, followersLoad)
	return nil
}

func (i *inboxRelationshipTests) receivingAFollowActivity(ctx context.Context) error {
	i.Receiver = Author
	followers := build.Collection(build.WithID(vocab.Followers.IRI(i.Receiver)))

	i.st = Storage(WithLocalMocks(Author, followers))
	i.Activity = build.Follow(
		build.WithID(RemoteAuthorID.AddPath("follow")),
		build.WithActor(RemoteAuthorID),
		build.WithObject(AuthorID),
	)
	i.Author = *RemoteAuthor
	i.ReceivedIn = vocab.Inbox.IRI(Author)

	return nil
}

func (i *inboxRelationshipTests) weRespondWithAnAcceptActivity(ctx context.Context) error {
	t := godog.T(ctx)

	inboxIRI := vocab.Inbox.IRI(Author)
	inbox, err := i.st.Load(inboxIRI)
	if err != nil {
		t.Errorf("Unexpected error received when loading local followers collection: %v", err)
	}

	BuildAssertions(
		HasID(inboxIRI),
		HasType(vocab.OrderedCollectionType),
		HasExactItems(i.Activity),
		HasTotalItems(1),
	).Run(t, inbox)

	accept := build.Accept(
		build.WithActor(AuthorID),
		build.WithObject(i.Activity.GetID()),
	)

	// NOTE(marius): check that the remote server receives the Accept activity if we process it in
	// the Author's outbox.
	ms := Remote(
		WithClientActorKey(Author, ed2559Key),
		WithHandlers(Handler(vocab.Inbox.IRI(RemoteAuthor), ReceivedActivityIdenticalTo(t, accept))),
	)

	mp := MockPlumbing{}
	p := mp.InitP(processing.WithStorage(i.st), WithRemote(ms))
	_, err = p.ProcessActivity(accept, *Author, vocab.Outbox.IRI(Author))
	if err != nil {
		t.Errorf("Unexpected error received when sending Accept activity: %v", err)
	}

	if err = ms.Success(t); err != nil {
		return err
	}

	followersID := vocab.Followers.IRI(Author)
	followersLoad, err := i.st.Load(followersID)
	if err != nil {
		t.Errorf("Unexpected error received when loading local followers collection: %v", err)
	}

	checks := BuildAssertions(
		HasID(followersID),
		HasType(vocab.CollectionType),
		HasExactItems(RemoteAuthor),
		HasTotalItems(1),
	)

	checks.Run(t, followersLoad)
	return nil
}

func (i *inboxRelationshipTests) weSendAnAcceptActivityInResponse(ctx context.Context) error {
	t := godog.T(ctx)

	ms := Remote(WithHandlers(Handler(RemoteAuthorID, ServeItemHandler(t, nil, RemoteAuthor))))

	mp := MockPlumbing{}
	p := mp.InitP(processing.WithStorage(i.st), WithRemote(ms))
	_, err := p.ProcessActivity(i.Activity, i.Author, i.ReceivedIn)
	if err != nil {
		t.Errorf("Unexpected error received: %v", err)
	}

	if err = ms.Success(t); err != nil {
		return err
	}

	accept := build.Accept(
		build.WithActor(AuthorID),
		build.WithObject(i.Activity.GetID()),
	)
	// NOTE(marius): check that the remote server receives the Accept activity if we process it in
	// the Author's outbox.
	ms = Remote(
		WithClientActorKey(Author, ed2559Key),
		WithHandlers(Handler(vocab.Inbox.IRI(RemoteAuthor), ReceivedActivityIdenticalTo(t, accept))),
	)

	p = mp.InitP(processing.WithStorage(i.st), WithRemote(ms))
	_, err = p.ProcessActivity(accept, *Author, vocab.Outbox.IRI(Author))
	if err != nil {
		t.Errorf("Unexpected error received when sending Accept activity: %v", err)
	}

	if err = ms.Success(t); err != nil {
		return err
	}

	return nil
}

func Test_Inbox_Relationship(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"inbox-relationship.feature"}

	i := new(inboxRelationshipTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Inbox Relationship",
		ScenarioInitializer: i.initScenario,
		Options:             &opts,
	}.Run()
}
