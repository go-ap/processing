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

type outboxReactionsTests struct {
	// NOTE(marius): elements needed to initialize the processor.P
	mp MockPlumbing
	st *LocalStorage

	// NOTE(marius): test to be run
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (o *outboxReactionsTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Before(func(ctx context.Context, sc *godog.Scenario) (context.Context, error) {
		o.mp = MockPlumbing{}
		o.st = nil
		return ctx, nil
	})

	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)

	// NOTE(marius): Follow
	ctx.Step(`^a Follow activity between two local Actors$`, o.aFollowActivityBetweenTwoLocalActors)
	ctx.Step(`^the Actor is \*not\* added to the Followers collection$`, o.theActorIsNotAddedToTheFollowersCollection)

	// NOTE(marius): Like
	ctx.Step(`^receiving a Like activity$`, o.receivingALikeActivity)
	ctx.Step(`^the activity is added to the object's Likes collection$`, o.theActivityIsAddedToTheObjectsLikesCollection)
	ctx.Step(`^the object is added to the actor's Liked collection$`, o.theObjectIsAddedToTheActorsLikedCollection)
}

func (o *outboxReactionsTests) aFollowActivityBetweenTwoLocalActors(ctx context.Context) error {
	toFollow := build.Person(
		build.WithID("http://example.com/~notDoe"),
		build.WithFollowers(),
	)

	o.st = Storage(WithLocalMocks(Author, toFollow))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Follow(build.WithActor(Author.ID), build.WithObject(toFollow.ID))

	return nil
}

func (o *outboxReactionsTests) theActorIsNotAddedToTheFollowersCollection(ctx context.Context) error {
	p := o.mp.InitP(processing.WithStorage(o.st))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return fmt.Errorf("unable to process activity %w", err)
	}
	t := godog.T(ctx)

	followersIRI := vocab.Followers.IRI(o.Author)
	followersCol, err := o.st.Load(followersIRI)
	if err != nil {
		t.Errorf("unable to load followers collection %s: %v", followersIRI, err)
		return fmt.Errorf("unable to load followers collection %s: %w", followersIRI, err)
	}
	toFollow := vocab.IRI("http://example.com/~notDoe")
	err = vocab.OnOrderedCollection(followersCol, func(col *vocab.OrderedCollection) error {
		if col.OrderedItems.IRIs().Contains(toFollow) {
			return fmt.Errorf("unable to find actor %s in followers collection %s", toFollow, followersIRI)
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (o *outboxReactionsTests) receivingALikeActivity() error {
	toLikeID := Author.ID.AddPath("to-like")
	toLike := build.Note(
		build.WithID(toLikeID),
		build.WithContent("Lorem ipsum dolor sic amet"),
		build.WithLikes(),
	)

	o.st = Storage(WithLocalMocks(Author, toLike))

	o.Author = *Author
	o.ReceivedIn = vocab.Outbox.IRI(Author)
	o.Activity = build.Like(build.WithActor(Author.ID), build.WithObject(toLike.ID))

	return nil
}

func (o *outboxReactionsTests) theActivityIsAddedToTheObjectsLikesCollection(ctx context.Context) error {
	toLikeID := Author.ID.AddPath("to-like")

	t := godog.T(ctx)

	likesIRI := vocab.Likes.IRI(toLikeID)
	likesCol, err := o.st.Load(likesIRI)
	if err != nil {
		t.Errorf("unable to load liked collection %s: %v", likesIRI, err)
		return err
	}

	likeID := o.Activity.GetID()
	err = vocab.OnOrderedCollection(likesCol, func(col *vocab.OrderedCollection) error {
		if !col.OrderedItems.IRIs().Contains(likeID) {
			return fmt.Errorf("unable to find %s", likeID)
		}
		return nil
	})
	if err != nil {
		t.Errorf("failed assertion for collection %s: %v", likesIRI, err)
		return err
	}

	return nil
}

func (o *outboxReactionsTests) theObjectIsAddedToTheActorsLikedCollection(ctx context.Context) error {
	p := o.mp.InitP(processing.WithStorage(o.st))
	_, err := p.ProcessActivity(o.Activity, o.Author, o.ReceivedIn)
	if err != nil {
		return fmt.Errorf("unable to process activity %w", err)
	}
	t := godog.T(ctx)

	toLikeID := Author.ID.AddPath("to-like")

	likedIRI := vocab.Liked.IRI(o.Author)
	likedCol, err := o.st.Load(likedIRI)
	if err != nil {
		t.Errorf("unable to load liked collection %s: %v", likedIRI, err)
		return err
	}

	err = vocab.OnOrderedCollection(likedCol, func(col *vocab.OrderedCollection) error {
		if !col.OrderedItems.IRIs().Contains(toLikeID) {
			return fmt.Errorf("unable to find %s", toLikeID)
		}
		return nil
	})
	if err != nil {
		t.Errorf("failed assertion for collection %s: %v", likedIRI, err)
		return err
	}

	return nil
}

func Test_Outbox_Reactions(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"outbox-reactions.feature"}

	c := new(outboxReactionsTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Outbox Reactions",
		ScenarioInitializer: c.initScenario,
		Options:             &opts,
	}.Run()
}
