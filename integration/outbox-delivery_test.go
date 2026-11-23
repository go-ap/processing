package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
	"github.com/go-ap/processing"
	"github.com/google/go-cmp/cmp"
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

	ctx.Step(`^an Activity with to, bto, cc and bcc recipients is received$`, o.anActivityWithToBtoCcAndBccRecipientsIsReceived)
	ctx.Step(`^it gets delivered to all recipients$`, o.itGetsDeliveredToAllRecipients)

	ctx.Step(`^an Activity without an ID is received$`, o.anActivityWithoutAnIDIsReceived)
	ctx.Step(`^it gets delivered with an ID$`, o.itGetsDeliveredWithAnID)

	ctx.Step(`^an Activity with non-public remote recipient is received$`, o.anActivityWithNonpublicRemoteRecipientIsReceived)
	ctx.Step(`^the recipient is dereferenced using the author's credentials$`, o.theRecipientIsDereferencedUsingTheAuthorsCredentials)

	ctx.Step(`^an Activity with a recipient being a local collection is received$`, o.anActivityWithARecipientBeingALocalCollectionIsReceived)
	ctx.Step(`^all the collection members receive the Activity$`, o.allTheCollectionMembersReceiveTheActivity)

	ctx.Step(`^a Create Activity is received$`, o.aCreateActivityIsReceived)
	ctx.Step(`^the recipients receive a non flattened Activity Object$`, o.theRecipientsReceiveANonFlattenedActivityObject)

	ctx.Step(`^an Add Activity is received$`, o.anAddActivityIsReceived)
	ctx.Step(`^the recipients receive a non flattened Target$`, o.theRecipientsReceiveANonFlattenedTarget)

	ctx.Step(`^an Activity with duplicate recipients is received$`, o.anActivityWithDuplicateRecipientsIsReceived)
	ctx.Step(`^the recipients are deduplicated$`, o.theRecipientsAreDeduplicated)

	ctx.Step(`^an Activity with its Actor as a recipient is received$`, o.anActivityWithItsActorAsARecipientIsReceived)
	ctx.Step(`^the Actor doesn't get a copy in their inbox$`, o.theActorDoesntGetACopyInTheirInbox)

	ctx.Step(`^a Block Activity is received$`, o.aBlockActivityIsReceived)
	ctx.Step(`^the Blocked object gets removed from the recipients list$`, o.theBlockedObjectGetsRemovedFromTheRecipientsList)

	ctx.Step(`^an Activity with recipients that have a sharedInbox is received$`, o.anActivityWithRecipientsThatHaveASharedInboxIsReceived)
	ctx.Step(`^the Activity is disseminated to the sharedInbox, not to the recipients inboxes$`, o.theActivityIsDisseminatedToTheSharedInbox)

	ctx.Step(`^an Activity with recipients is received$`, o.anActivityWithRecipientsIsReceived)
	ctx.Step(`^the Activity is disseminated to the recipients that don't have sharedInboxes$`, o.theActivityIsDisseminatedToTheRecipientsThatDontHaveSharedInboxes)
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
	makePerson = func(id vocab.IRI, name string, initFns ...build.InitFn) *vocab.Actor {
		return build.Person(append([]build.InitFn{
			build.WithID(id),
			build.WithName(name),
			build.WithAudience(vocab.PublicNS),
			build.WithInbox(),
			build.WithOutbox(),
			build.WithPreferredUsername(name),
			build.WithPublicKey(PublicKey(rsaKey.Public(), id)),
		}, initFns...)...)
	}

	remoteActorCCID  = vocab.IRI("http://example.com/actor-cc")
	remoteActorBtoID = vocab.IRI("http://example.com/actor-bto")
	remoteActorBCCID = vocab.IRI("http://example.com/actor-bcc")
)

func (o *deliveryOutboxTests) anActivityWithToBtoCcAndBccRecipientsIsReceived(ctx context.Context) error {
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

	return o.mr.Success(t)
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

	if err = o.mr.Success(t); err != nil {
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

	return o.mr.Success(t)
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

	return o.mr.Success(t)
}

func (o *deliveryOutboxTests) aCreateActivityIsReceived(ctx context.Context) error {
	o.st = Storage(WithLocalMocks(Author))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Create(
		build.WithActor(Author.ID),
		build.WithObject(build.Note(
			build.WithTo(RemoteRootID.AddPath("actor1")),
			build.WithID("http://example.localdomain/note"),
			build.WithContent("Lorem ipsum"),
		)),
	)

	return nil
}

func (o *deliveryOutboxTests) theRecipientsReceiveANonFlattenedActivityObject(ctx context.Context) error {
	t := godog.T(ctx)

	actor1 := makePerson(RemoteRootID.AddPath("actor1"), "Actor One")
	o.mr = Remote(
		WithClientActorKey(Author, ed2559Key),
		// NOTE(marius): serve actor when being dereferenced in the followers collection
		WithHandlers(Handler(actor1.ID, ServeItemHandler(t, nil, actor1))),
		// NOTE(marius): disseminate the test activity
		WithHandlers(
			Handler(vocab.Inbox.IRI(actor1), ReceivedActivityMatches(t, func(t TestingT, item vocab.Item) {
				_ = vocab.OnActivity(item, func(act *vocab.Activity) error {
					if vocab.IsIRI(act.Object) || vocab.IsIRIs(act.Object) {
						t.Errorf("the received activity object was not a full object: %v", act.Object)
					}
					return nil
				})
			})),
		),
	)

	p := o.mp.InitP(processing.WithStorage(o.st), WithRemote(o.mr))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return err
	}

	return o.mr.Success(t)
}

func (o *deliveryOutboxTests) anAddActivityIsReceived(ctx context.Context) error {
	target := build.Collection(build.WithID(RemoteRootID.AddPath("target")))
	o.st = Storage(WithLocalMocks(Author, target))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Add(
		build.WithActor(Author.ID),
		build.WithTo(RemoteRootID.AddPath("actor1")),
		build.WithObject(build.Note(
			build.WithID("http://example.localdomain/note"),
			build.WithContent("Lorem ipsum"),
		)),
		build.WithTarget(target.ID),
	)

	return nil
}

func (o *deliveryOutboxTests) theRecipientsReceiveANonFlattenedTarget(ctx context.Context) error {
	t := godog.T(ctx)

	actor1 := makePerson(RemoteRootID.AddPath("actor1"), "Actor One")
	o.mr = Remote(
		WithClientActorKey(Author, ed2559Key),
		// NOTE(marius): serve actor when being dereferenced in the followers collection
		WithHandlers(Handler(actor1.ID, ServeItemHandler(t, nil, actor1))),
		// NOTE(marius): disseminate the test activity
		WithHandlers(
			Handler(vocab.Inbox.IRI(actor1), ReceivedActivityMatches(t, func(t TestingT, item vocab.Item) {
				_ = vocab.OnActivity(item, func(act *vocab.Activity) error {
					if vocab.IsIRI(act.Target) || vocab.IsIRIs(act.Target) {
						t.Errorf("the received activity target was not a full object: %v", act.Target)
					}
					return nil
				})
			})),
		),
	)

	p := o.mp.InitP(processing.WithStorage(o.st), WithRemote(o.mr))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return err
	}

	return o.mr.Success(t)
}

func (o *deliveryOutboxTests) anActivityWithDuplicateRecipientsIsReceived(ctx context.Context) error {
	note := build.Note(
		build.WithID("http://example.localdomain/note"),
		build.WithAttributedTo(AuthorID),
		build.WithContent("Lorem ipsum"),
	)
	o.st = Storage(WithLocalMocks(Author, note))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Update(
		build.WithActor(Author.ID),
		build.WithTo(RemoteRootID.AddPath("actor-to")),
		build.WithCC(RemoteRootID.AddPath("actor-cc"), RemoteRootID.AddPath("actor-to")),
		build.WithBto(RemoteRootID.AddPath("actor-to"), RemoteRootID.AddPath("actor-hidden")),
		build.WithBCC(RemoteRootID.AddPath("actor-to"), RemoteRootID.AddPath("actor-cc"), RemoteRootID.AddPath("actor-hidden")),
		build.WithObject(build.Note(
			build.WithID("http://example.localdomain/note"),
			build.WithName("Lipsum title"),
		)),
	)

	return nil
}

func (o *deliveryOutboxTests) theRecipientsAreDeduplicated(ctx context.Context) error {
	t := godog.T(ctx)

	actorTo := makePerson(RemoteRootID.AddPath("actor-to"), "Actor To")
	actorCC := makePerson(RemoteRootID.AddPath("actor-cc"), "Actor CC")
	// NOTE(marius): this actor should be still retrieved in the process of disseminating to its inbox,
	// (THIS IS NOT ACTUALLY TRUE ATM) but will not show up in the incoming recipients, as it appears only in blind
	// recipient lists.
	actorHidden := makePerson(RemoteRootID.AddPath("actor-hidden"), "Actor Hidden")

	expectedTo := vocab.ItemCollection{actorTo.ID}
	expectedBto := vocab.ItemCollection{actorHidden.ID}
	expectedCC := vocab.ItemCollection{actorCC.ID}
	expectedBCC := vocab.ItemCollection{}

	o.mr = Remote(
		WithClientActorKey(Author, ed2559Key),
		// NOTE(marius): serve actor when being dereferenced in the followers collection
		WithHandlers(Handler(actorTo.ID, ServeItemHandler(t, nil, actorTo))),
		WithHandlers(Handler(actorCC.ID, ServeItemHandler(t, nil, actorCC))),
		WithHandlers(Handler(actorHidden.ID, ServeItemHandler(t, nil, actorHidden))),
		// NOTE(marius): disseminate the test activity
		WithHandlers(
			Handler(vocab.Inbox.IRI(actorTo), ReceivedActivityMatches(t, func(t TestingT, item vocab.Item) {
				_ = vocab.OnActivity(item, func(act *vocab.Activity) error {
					if !cmp.Equal(act.To, expectedTo) {
						t.Errorf("the incoming activity's To recipient list = %s", cmp.Diff(expectedTo, act.To))
					}
					if !cmp.Equal(act.CC, expectedCC) {
						t.Errorf("the incoming activity's CC recipient list = %s", cmp.Diff(expectedCC, act.CC))
					}
					if !cmp.Equal(act.Bto, expectedBto) {
						t.Errorf("the incoming activity's Bto recipient list = %s", cmp.Diff(expectedBto, act.Bto))
					}
					if !cmp.Equal(act.BCC, expectedBCC) {
						t.Errorf("the incoming activity's BCC recipient list = %s", cmp.Diff(expectedBCC, act.BCC))
					}
					return nil
				})
			})),
		),
	)

	p := o.mp.InitP(processing.WithStorage(o.st), WithRemote(o.mr))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return err
	}

	return o.mr.Success(t)
}

func (o *deliveryOutboxTests) anActivityWithItsActorAsARecipientIsReceived(ctx context.Context) error {
	note := build.Note(
		build.WithID("http://example.localdomain/note"),
		build.WithContent("Lorem ipsum"),
	)
	o.st = Storage(WithLocalMocks(Author, note))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Like(
		build.WithActor(Author.ID),
		build.WithTo(AuthorID),
		build.WithObject(vocab.IRI("http://example.localdomain/note")),
	)

	return nil
}

func (o *deliveryOutboxTests) theActorDoesntGetACopyInTheirInbox(ctx context.Context) error {
	p := o.mp.InitP(processing.WithStorage(o.st))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return err
	}

	t := godog.T(ctx)

	inb, err := o.st.Load(vocab.Inbox.IRI(o.Author))
	if err != nil {
		t.Errorf("unexpected error when loading Author's inbox: %v", err)
	}
	return vocab.OnOrderedCollection(inb, func(col *vocab.OrderedCollection) error {
		itemIRIs := col.OrderedItems.IRIs()
		if itemIRIs.Contains(o.Activity.GetLink()) {
			t.Errorf("the Blocked actor's inbox contains the activity: %v", itemIRIs)
		}
		return nil
	})
}

func (o *deliveryOutboxTests) aBlockActivityIsReceived(ctx context.Context) error {
	o.st = Storage(WithLocalMocks(Author))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Block(
		build.WithActor(Author.ID),
		build.WithTo(RemoteRootID.AddPath("non-blocked"), RemoteAuthorID),
		build.WithObject(RemoteAuthorID),
	)

	return nil
}

func (o *deliveryOutboxTests) theBlockedObjectGetsRemovedFromTheRecipientsList(ctx context.Context) error {
	t := godog.T(ctx)

	nonBlockedActor := makePerson(RemoteRootID.AddPath("non-blocked"), "Non Blocked")
	o.mr = Remote(
		WithClientActorKey(Author, ed2559Key), WithClientTestingT(t),
		WithHandlers(Handler(RemoteAuthor.ID, ServeItemHandler(t, nil, RemoteAuthor))),
		WithHandlers(Handler(nonBlockedActor.ID, ServeItemHandler(t, nil, nonBlockedActor))),
		WithHandlers(
			Handler(vocab.Inbox.IRI(nonBlockedActor), ReceivedActivityMatches(t, func(t TestingT, item vocab.Item) {
				_ = vocab.OnActivity(item, func(act *vocab.Activity) error {
					if act.Recipients().IRIs().Contains(RemoteAuthor.ID) {
						t.Errorf("Blocked actor has received the block")
					}
					return nil
				})
			})),
		),
	)

	p := o.mp.InitP(processing.WithStorage(o.st), WithRemote(o.mr))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return err
	}

	return o.mr.Success(t)
}

func (o *deliveryOutboxTests) anActivityWithRecipientsThatHaveASharedInboxIsReceived(ctx context.Context) error {
	o.st = Storage(WithLocalMocks(Author))

	sharedInboxActor1ID := RemoteRootID.AddPath("actor-1")
	sharedInboxActor2ID := RemoteRootID.AddPath("actor-2")

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Flag(
		build.WithActor(Author.ID),
		build.WithTo(sharedInboxActor1ID, sharedInboxActor2ID),
		// NOTE(marius): the sharedInbox mechanism is supposed to be used only for public activities
		// The spec says: "used for wide delivery of publicly addressed activities"
		// https://www.w3.org/TR/activitypub/#sharedInbox:
		build.WithCC(vocab.PublicNS),
		build.WithObject(RemoteAuthorID),
	)

	return nil
}

func (o *deliveryOutboxTests) theActivityIsDisseminatedToTheSharedInbox(ctx context.Context) error {
	t := godog.T(ctx)

	sharedInboxActor1 := makePerson(
		RemoteRootID.AddPath("actor-1"), "Actor One", build.WithSharedInbox(vocab.Inbox.IRI(RemoteRootID)),
	)
	sharedInboxActor2 := makePerson(
		RemoteRootID.AddPath("actor-2"), "Actor Two", build.WithSharedInbox(vocab.Inbox.IRI(RemoteRootID)),
	)
	o.mr = Remote(
		WithClientActorKey(Author, ed2559Key), WithClientTestingT(t),
		WithHandlers(Handler(sharedInboxActor1.ID, ServeItemHandler(t, nil, sharedInboxActor1))),
		WithHandlers(Handler(sharedInboxActor2.ID, ServeItemHandler(t, nil, sharedInboxActor2))),
		WithHandlers(Handler(RemoteAuthor.ID, ServeItemHandler(t, nil, RemoteAuthor))),
		WithHandlers(Handler(vocab.Inbox.IRI(RemoteRootID), ReceivedActivityIdenticalTo(t, o.Activity))),
	)

	p := o.mp.InitP(processing.WithStorage(o.st), WithRemote(o.mr))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return err
	}

	return o.mr.Success(t)
}

func (o *deliveryOutboxTests) anActivityWithRecipientsIsReceived(ctx context.Context) error {
	sharedInboxActor1 := makePerson(
		RemoteRootID.AddPath("actor-1"), "Actor One", build.WithSharedInbox(vocab.Inbox.IRI(RemoteRootID)),
	)
	sharedInboxActor2 := makePerson(
		RemoteRootID.AddPath("actor-2"), "Actor Two", build.WithSharedInbox(vocab.Inbox.IRI(RemoteRootID)),
	)
	// NOTE(marius): this actor won't have a sharedInbox
	actor3 := makePerson(RemoteRootID.AddPath("actor-3"), "Actor Three")

	o.st = Storage(WithLocalMocks(Author, sharedInboxActor1, sharedInboxActor2, actor3, RemoteAuthor))
	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Ignore(
		build.WithActor(Author.ID),
		build.WithTo(sharedInboxActor1.ID, sharedInboxActor2.ID, actor3.ID),
		build.WithCC(vocab.PublicNS),
		build.WithObject(RemoteAuthorID),
	)

	return nil
}

func (o *deliveryOutboxTests) theActivityIsDisseminatedToTheRecipientsThatDontHaveSharedInboxes(ctx context.Context) error {
	t := godog.T(ctx)

	actor3ID := RemoteRootID.AddPath("actor-3")
	o.mr = Remote(
		WithClientActorKey(Author, ed2559Key), WithClientTestingT(t),
		WithHandlers(Handler(vocab.Inbox.IRI(RemoteRootID), ReceivedActivityIdenticalTo(t, o.Activity))),
		WithHandlers(Handler(vocab.Inbox.IRI(actor3ID), ReceivedActivityIdenticalTo(t, o.Activity))),
	)

	p := o.mp.InitP(processing.WithStorage(o.st), WithRemote(o.mr))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return err
	}

	return o.mr.Success(t)
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
