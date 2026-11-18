package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
	"github.com/go-ap/processing"
)

type deliveryOutboxTests struct {
	// NOTE(marius): elements needed to initialize the processor.P
	mp MockPlumbing
	st *LocalStorage
	mr *MockRemote

	// NOTE(marius): test to be run
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (o *deliveryOutboxTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		o.mp = MockPlumbing{}
		o.mr = nil
		o.st = nil
		return ctx, nil
	})

	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)

	ctx.Step(`^an Activity is received$`, o.anActivityIsReceived)
	ctx.Step(`^it gets delivered to remote recipients$`, o.itGetsDeliveredToRemoteRecipients)

	ctx.Step(`^an Activity is received with to, bto, cc and bcc recipients is received$`, o.anActivityIsReceivedWithToBtoCcAndBccRecipientsIsReceived)
	ctx.Step(`^it gets delivered to all recipients$`, o.itGetsDeliveredToAllRecipients)

	ctx.Step(`^an Activity without an ID is received$`, o.anActivityWithoutAnIDIsReceived)
	ctx.Step(`^it gets delivered with an ID$`, o.itGetsDeliveredWithAnID)

	ctx.Step(`^an Activity with non-public remote recipient is received$`, o.anActivityWithNonpublicRemoteRecipientIsReceived)
	ctx.Step(`^the recipient is dereferenced using the author's credentials$`, o.theRecipientIsDereferencedUsingTheAuthorsCredentials)

	ctx.Step(`^an Activity with a recipient being a local collection is received$`, o.anActivityWithARecipientBeingALocalCollectionIsReceived)
	ctx.Step(`^all the collection members receive the Activity$`, o.allTheCollectionMembersReceiveTheActivity)

	ctx.Step(`^a Block Activity is received$`, o.aBlockActivityIsReceived)
	ctx.Step(`^a Create Activity is received$`, o.aCreateActivityIsReceived)
	ctx.Step(`^an Activity with duplicate recipients is received$`, o.anActivityWithDuplicateRecipientsIsReceived)
	ctx.Step(`^an Activity with its Actor as a recipient is received$`, o.anActivityWithItsActorAsARecipientIsReceived)
	ctx.Step(`^an Activity with recipients is received$`, o.anActivityWithRecipientsIsReceived)
	ctx.Step(`^an Activity with recipients that have a sharedInbox is received$`, o.anActivityWithRecipientsThatHaveASharedInboxIsReceived)
	ctx.Step(`^an Add Activity is received$`, o.anAddActivityIsReceived)
	ctx.Step(`^not to the recipients inboxes$`, o.notToTheRecipientsInboxes)
	ctx.Step(`^the Activity is disseminated to the recipients that don't have sharedInboxes$`, o.theActivityIsDisseminatedToTheRecipientsThatDontHaveSharedInboxes)
	ctx.Step(`^the Activity is disseminated to the sharedInbox$`, o.theActivityIsDisseminatedToTheSharedInbox)
	ctx.Step(`^the Actor doesn't get a copy in their inbox$`, o.theActorDoesntGetACopyInTheirInbox)
	ctx.Step(`^the Blocked object gets removed from the recipients list$`, o.theBlockedObjectGetsRemovedFromTheRecipientsList)
	ctx.Step(`^the recipients are deduplicated$`, o.theRecipientsAreDeduplicated)
	ctx.Step(`^the recipients receive a non flattened Activity Object$`, o.theRecipientsReceiveANonFlattenedActivityObject)
	ctx.Step(`^the recipients receive a non flattened Target$`, o.theRecipientsReceiveANonFlattenedTarget)
}

func (o *deliveryOutboxTests) anActivityIsReceived(ctx context.Context) error {
	o.st = Storage(WithLocalMocks(Author, RemoteAuthor))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Create(
		build.WithActor(Author.ID),
		build.WithTo(RemoteAuthor.ID),
		build.WithObject(build.Note(
			build.WithID("http://example.localdomain/note"),
			build.WithContent("Lorem ipsum"),
		)),
	)
	return nil
}

func (o *deliveryOutboxTests) itGetsDeliveredToRemoteRecipients(ctx context.Context) error {
	t := godog.T(ctx)

	o.mr = Remote(
		WithHandlers(
			Handler(vocab.Inbox.IRI(RemoteAuthorID), ReceivedActivityIdenticalTo(t, o.Activity)),
		),
	)

	p := o.mp.InitP(processing.WithStorage(o.st), WithRemote(o.mr))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return err
	}
	return nil
}

var (
	makePerson = func(id vocab.IRI, name string) *vocab.Actor {
		return build.Person(
			build.WithID(id),
			build.WithName(name),
			build.WithAudience(vocab.PublicNS),
			build.WithInbox(),
			build.WithOutbox(),
			build.WithPreferredUsername(name),
			build.WithPublicKey(PublicKey(rsaKey.Public(), id)),
		)
	}

	remoteActorCCID  = vocab.IRI("http://example.com/actor-cc")
	remoteActorBtoID = vocab.IRI("http://example.com/actor-bto")
	remoteActorBCCID = vocab.IRI("http://example.com/actor-bcc")
)

func (o *deliveryOutboxTests) anActivityIsReceivedWithToBtoCcAndBccRecipientsIsReceived(ctx context.Context) error {
	remoteActorCC := makePerson(remoteActorCCID, "Actor CC")
	remoteActorBto := makePerson(remoteActorBtoID, "Actor Bto")
	remoteActorBCC := makePerson(remoteActorBCCID, "Actor BCC")

	o.st = Storage(WithLocalMocks(Author, RemoteAuthor, remoteActorCC, remoteActorBto, remoteActorBCC))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Create(
		build.WithActor(Author.ID),
		build.WithTo(RemoteAuthor.ID),
		build.WithCC(remoteActorCC),
		build.WithBto(remoteActorBto),
		build.WithBCC(remoteActorBCC),
		build.WithObject(build.Note(
			build.WithID("http://example.localdomain/note"),
			build.WithContent("Lorem ipsum"),
		)),
	)

	return nil
}

func (o *deliveryOutboxTests) itGetsDeliveredToAllRecipients(ctx context.Context) error {
	t := godog.T(ctx)

	o.mr = Remote(
		WithHandlers(Handler(vocab.Inbox.IRI(RemoteAuthorID), ReceivedActivityIdenticalTo(t, o.Activity))),
		WithHandlers(Handler(vocab.Inbox.IRI(remoteActorCCID), ReceivedActivityIdenticalTo(t, o.Activity))),
		WithHandlers(Handler(vocab.Inbox.IRI(remoteActorBtoID), ReceivedActivityIdenticalTo(t, o.Activity))),
		WithHandlers(Handler(vocab.Inbox.IRI(remoteActorBCCID), ReceivedActivityIdenticalTo(t, o.Activity))),
	)

	p := o.mp.InitP(processing.WithStorage(o.st), WithRemote(o.mr))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return err
	}

	return o.mr.Success()
}

func (o *deliveryOutboxTests) anActivityWithoutAnIDIsReceived(ctx context.Context) error {
	o.st = Storage(WithLocalMocks(Author, RemoteAuthor))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Create(
		build.WithActor(Author.ID),
		build.WithTo(RemoteAuthor.ID),
		build.WithObject(build.Note(
			build.WithID("http://example.localdomain/note"),
			build.WithContent("Lorem ipsum"),
		)),
	)

	return nil
}

func (o *deliveryOutboxTests) itGetsDeliveredWithAnID(ctx context.Context) error {
	t := godog.T(ctx)

	o.mr = Remote(
		WithHandlers(Handler(vocab.Inbox.IRI(RemoteAuthorID), ReceivedActivityIdenticalTo(t, o.Activity))),
	)

	p := o.mp.InitP(processing.WithStorage(o.st), WithRemote(o.mr))

	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return err
	}

	if err = o.mr.Success(); err != nil {
		return err
	}
	return vocab.OnActivity(o.Activity, func(act *vocab.Activity) error {
		if act.ID == "" {
			return fmt.Errorf("remote activity has empty ID: %s", act.ID)
		}
		return nil
	})
}

var nonPublicActor = func() *vocab.Actor {
	act := makePerson("http://example.com/~hidden", "Hidden")
	act.Audience = nil
	return act
}()

func (o *deliveryOutboxTests) anActivityWithNonpublicRemoteRecipientIsReceived(ctx context.Context) error {
	o.st = Storage(WithLocalMocks(Author))
	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Create(
		build.WithActor(Author.ID),
		build.WithTo(nonPublicActor.ID),
		build.WithObject(build.Note(
			build.WithID("http://example.localdomain/note"),
			build.WithContent("Lorem ipsum"),
		)),
	)

	return nil
}

func (o *deliveryOutboxTests) theRecipientIsDereferencedUsingTheAuthorsCredentials(ctx context.Context) error {
	t := godog.T(ctx)
	o.mr = Remote(
		WithClientActorKey(Author, ed2559Key),
		// NOTE(marius): the check is done in the ServeItemHandler, where the only authorized actor to access the
		// nonPublicActor is the Author actor through an HTTP-Signature header.
		WithHandlers(Handler(nonPublicActor.ID, ServeItemHandler(t, Auth(Author, ed2559Key), nonPublicActor))),
	)

	p := o.mp.InitP(processing.WithStorage(o.st), WithRemote(o.mr))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return err
	}

	return o.mr.Success()
}

func (o *deliveryOutboxTests) anActivityWithARecipientBeingALocalCollectionIsReceived(ctx context.Context) error {

	authorFollowers := build.OrderedCollection(
		build.WithID(vocab.Followers.IRI(Author)),
		build.WithTotalItems(2),
		// NOTE(marius): the two actors are remote
		build.WithItems(RemoteRootID.AddPath("actor1"), RemoteRootID.AddPath("actor2")),
	)
	o.st = Storage(WithLocalMocks(Author, authorFollowers))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Create(
		build.WithActor(Author.ID),
		build.WithCC(authorFollowers.ID),
		build.WithObject(build.Note(
			build.WithID("http://example.localdomain/note"),
			build.WithContent("Lorem ipsum"),
		)),
	)

	return nil
}

func (o *deliveryOutboxTests) allTheCollectionMembersReceiveTheActivity(ctx context.Context) error {
	t := godog.T(ctx)

	actor1 := makePerson(RemoteRootID.AddPath("actor1"), "Actor One")
	actor2 := makePerson(RemoteRootID.AddPath("actor2"), "Actor Two")
	o.mr = Remote(
		WithClientActorKey(Author, ed2559Key),
		// NOTE(marius): serve actors when being dereferenced in the followers collection
		WithHandlers(Handler(actor1.ID, ServeItemHandler(t, nil, actor1))),
		WithHandlers(Handler(actor2.ID, ServeItemHandler(t, nil, actor2))),
		// NOTE(marius): disseminate to them the test activity
		WithHandlers(Handler(vocab.Inbox.IRI(actor1), ReceivedActivityIdenticalTo(t, o.Activity))),
		WithHandlers(Handler(vocab.Inbox.IRI(actor2), ReceivedActivityIdenticalTo(t, o.Activity))),
	)

	p := o.mp.InitP(processing.WithStorage(o.st), WithRemote(o.mr))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return err
	}

	return o.mr.Success()
}

func (o *deliveryOutboxTests) aBlockActivityIsReceived(ctx context.Context) error {
	return godog.ErrPending
}

func (o *deliveryOutboxTests) aCreateActivityIsReceived(ctx context.Context) error {
	return godog.ErrPending
}

func (o *deliveryOutboxTests) anActivityWithDuplicateRecipientsIsReceived(ctx context.Context) error {
	return godog.ErrPending
}

func (o *deliveryOutboxTests) anActivityWithItsActorAsARecipientIsReceived(ctx context.Context) error {
	return godog.ErrPending
}

func (o *deliveryOutboxTests) anActivityWithRecipientsIsReceived(ctx context.Context) error {
	return godog.ErrPending
}

func (o *deliveryOutboxTests) anActivityWithRecipientsThatHaveASharedInboxIsReceived(ctx context.Context) error {
	return godog.ErrPending
}

func (o *deliveryOutboxTests) anAddActivityIsReceived(ctx context.Context) error {
	return godog.ErrPending
}

func (o *deliveryOutboxTests) notToTheRecipientsInboxes(ctx context.Context) error {
	return godog.ErrPending
}

func (o *deliveryOutboxTests) theActivityIsDisseminatedToTheRecipientsThatDontHaveSharedInboxes(ctx context.Context) error {
	return godog.ErrPending
}

func (o *deliveryOutboxTests) theActivityIsDisseminatedToTheSharedInbox(ctx context.Context) error {
	return godog.ErrPending
}

func (o *deliveryOutboxTests) theActorDoesntGetACopyInTheirInbox(ctx context.Context) error {
	return godog.ErrPending
}

func (o *deliveryOutboxTests) theBlockedObjectGetsRemovedFromTheRecipientsList(ctx context.Context) error {
	return godog.ErrPending
}

func (o *deliveryOutboxTests) theRecipientsAreDeduplicated(ctx context.Context) error {
	return godog.ErrPending
}

func (o *deliveryOutboxTests) theRecipientsReceiveANonFlattenedActivityObject(ctx context.Context) error {
	return godog.ErrPending
}

func (o *deliveryOutboxTests) theRecipientsReceiveANonFlattenedTarget(ctx context.Context) error {
	return godog.ErrPending
}

func Test_Outbox_Delivery(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"outbox-delivery.feature"}

	c := new(deliveryOutboxTests)
	godog.TestSuite{
		Name:                "ActivityPub Outbox Delivery",
		ScenarioInitializer: c.initScenario,
		Options:             &opts,
	}.Run()
}
