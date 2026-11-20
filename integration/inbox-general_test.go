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

type inboxGeneralTests struct {
	st *LocalStorage

	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (i *inboxGeneralTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)

	ctx.Step(`^an Activity is received$`, i.anActivityIsReceived)
	ctx.Step(`^the activity is disseminated to local recipients$`, i.theActivityIsDisseminatedToLocalRecipients)

	ctx.Step(`^an Activity with to, bto, cc and bcc recipients is received$`, i.anActivityWithToBtoCcAndBccRecipientsIsReceived)

	ctx.Step(`^an Activity with a local collection as a recipient is received$`, i.anActivityWithALocalCollectionAsARecipientIsReceived)
	ctx.Step(`^the activity is forwarded to the remote actors part of the collection$`, i.theActivityIsForwardedToTheRemoteActorsPartOfCollection)
}

func (i *inboxGeneralTests) anActivityIsReceived() error {
	i.Author = *RemoteAuthor

	i.st = Storage(WithLocalMocks(Author, RemoteAuthor))

	i.ReceivedIn = vocab.Inbox.IRI(Author)
	i.Activity = build.Announce(
		build.WithID("http://example.com/announce"),
		build.WithActor(RemoteAuthor.ID),
		build.WithObject(build.Link(
			build.WithHref("http://example.com"),
			build.WithName("Lorem ipsum"),
		)),
	)

	return nil
}

func (i *inboxGeneralTests) anActivityWithToBtoCcAndBccRecipientsIsReceived() error {
	i.Author = *RemoteAuthor

	bccActor := makePerson(RootID.AddPath("local-actor-bcc"), "Local BCC")
	btoActor := makePerson(RootID.AddPath("local-actor-bto"), "Local Bto")

	i.st = Storage(WithLocalMocks(Author, RemoteAuthor, bccActor, btoActor))

	i.ReceivedIn = vocab.Inbox.IRI(Author)
	i.Activity = build.Announce(
		build.WithID("http://example.com/announce"),
		build.WithBto(btoActor),
		build.WithBCC(bccActor),
		build.WithActor(RemoteAuthor.ID),
		build.WithObject(build.Link(
			build.WithHref("http://example.com"),
			build.WithName("Lorem ipsum"),
		)),
	)

	return nil
}

func (i *inboxGeneralTests) theActivityIsDisseminatedToLocalRecipients() error {
	mp := MockPlumbing{}
	p := mp.InitP(processing.WithStorage(i.st))

	maybeAct, err := p.ProcessActivity(i.Activity, i.Author, i.ReceivedIn)
	if err != nil {
		return fmt.Errorf("received unexpected error: %w", err)
	}

	if !cmp.Equal(i.Activity, maybeAct) {
		return fmt.Errorf("returned activity is different from the received one %s", cmp.Diff(i.Activity, maybeAct))
	}

	act, err := vocab.ToActivity(maybeAct)
	if err != nil {
		return fmt.Errorf("received unexpected error: %w", err)
	}
	allRecipients := act.Recipients()
	allInboxes := make(vocab.IRIs, 0, len(allRecipients)+1)
	for _, rec := range allRecipients {
		_ = allInboxes.Append(vocab.Inbox.IRI(rec))
	}
	for _, inboxIRI := range append(allInboxes, i.ReceivedIn) {
		maybeInbox, err := i.st.Load(inboxIRI)
		if err != nil {
			return fmt.Errorf("unable to retrieve inbox %s: %w", inboxIRI, err)
		}
		err = vocab.OnCollectionIntf(maybeInbox, func(col vocab.CollectionInterface) error {
			items := col.Collection()
			if items.Contains(i.Activity) || items.IRIs().Contains(i.Activity.GetID()) {
				return nil
			}
			return fmt.Errorf("inbox %s does not contain expected activity: %s", inboxIRI, i.Activity.GetID())
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (i *inboxGeneralTests) anActivityWithALocalCollectionAsARecipientIsReceived() error {
	i.Author = *RemoteAuthor

	actor1 := makePerson(RemoteAuthorID.AddPath("actor1"), "Actor 1")
	actor2 := makePerson(RemoteAuthorID.AddPath("actor2"), "Actor 2")
	localActor := makePerson(RootID.AddPath("actor"), "Local Actor")

	followers := build.OrderedCollection(
		build.WithID(vocab.Followers.IRI(Author)),
		build.WithTotalItems(3),
		build.WithItems(actor1, actor2, localActor),
	)

	i.st = Storage(WithLocalMocks(Author, RemoteAuthor, actor1, actor2, localActor, followers))

	i.ReceivedIn = vocab.Inbox.IRI(Author)
	i.Activity = build.Announce(
		build.WithID("http://example.com/announce"),
		build.WithCC(followers.ID),
		build.WithActor(RemoteAuthor.ID),
		build.WithObject(build.Link(
			build.WithHref("http://example.com"),
			build.WithName("Lorem ipsum"),
		)),
	)

	return nil
}

func (i *inboxGeneralTests) theActivityIsForwardedToTheRemoteActorsPartOfCollection(ctx context.Context) error {
	mp := MockPlumbing{}

	t := godog.T(ctx)
	actor1ID := RemoteAuthorID.AddPath("actor1")
	actor2ID := RemoteAuthorID.AddPath("actor2")

	rem := Remote(
		WithHandlers(Handler(vocab.Inbox.IRI(actor1ID), ReceivedActivityIdenticalTo(t, i.Activity))),
		WithHandlers(Handler(vocab.Inbox.IRI(actor2ID), ReceivedActivityIdenticalTo(t, i.Activity))),
	)

	p := mp.InitP(
		processing.WithStorage(i.st),
		//WithClientActorKey(RemoteAuthor, rsaKey),
		WithRemote(rem),
	)

	maybeAct, err := p.ProcessActivity(i.Activity, i.Author, i.ReceivedIn)
	if err != nil {
		return fmt.Errorf("received unexpected error: %w", err)
	}

	if !cmp.Equal(i.Activity, maybeAct) {
		return fmt.Errorf("returned activity is different from the received one %s", cmp.Diff(i.Activity, maybeAct))
	}

	//act, err := vocab.ToActivity(maybeAct)
	//if err != nil {
	//	return fmt.Errorf("received unexpected error: %w", err)
	//}

	maybeInbox, err := i.st.Load(i.ReceivedIn)
	if err != nil {
		return fmt.Errorf("unable to retrieve inbox %s: %w", i.ReceivedIn, err)
	}

	err = vocab.OnCollectionIntf(maybeInbox, func(col vocab.CollectionInterface) error {
		items := col.Collection()
		if items.Contains(i.Activity) || items.IRIs().Contains(i.Activity.GetID()) {
			return nil
		}
		return fmt.Errorf("inbox %s does not contain expected activity: %s", i.ReceivedIn, i.Activity.GetID())
	})
	if err != nil {
		return err
	}

	return rem.Success(t)
}

func Test_Inbox_General(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"inbox-general.feature"}

	i := new(inboxGeneralTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Inbox General",
		ScenarioInitializer: i.initScenario,
		Options:             &opts,
	}.Run()
}
