package integration

import (
	"testing"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
)

func RunOutboxCollectionManagementTests(t *testing.T) {
	t.Run("Collection Management", func(t *testing.T) {
		RunOutboxAddTests(t)
		RunOutboxMoveTests(t)
		RunOutboxRemoveTests(t)
	})
}

func RunOutboxAddTests(t *testing.T) {
	t.Run("Add", func(t *testing.T) {
		mp := MockPlumbing{}

		targetID := RootID.AddPath("target")
		target := build.OrderedCollection(build.WithID(targetID))

		noteID := RootID.AddPath("to-move")
		toAdd := build.Note(
			build.WithID(noteID),
			build.WithName("Lipsum name"),
			build.WithAttributedTo(AuthorID),
			build.WithReplies(),
			build.WithTo(vocab.ItemCollection{vocab.PublicNS}),
		)

		p := mp.InitP(WithLocalMocks(Author, toAdd, target))
		tests := TestSuite{
			Name: "Add",
			Tests: []RunnableTest{
				TestPair{
					Name:       "note to target",
					Author:     *Author,
					ReceivedIn: vocab.Outbox.IRI(Author),
					Activity: build.Add(
						build.WithActor(Author.ID),
						build.WithObject(noteID),
						build.WithTarget(targetID),
					),
					TestFns: TestAssertions{
						HasID(mp.ShiftIRI),
						HasType(vocab.AddType),
						WasPublished(mp.PopTime),
						HasObject(HasID(noteID)),
					},
				},
			},
		}

		t.Run(tests.Label(), tests.TestFn(p))
	})
}

func RunOutboxMoveTests(t *testing.T) {
	t.Run("Move", func(t *testing.T) {
		t.Skipf("TODO")
		mp := MockPlumbing{}
		P := mp.InitP(WithLocalMocks(Author))

		tests := TestSuite{}
		t.Run(tests.Label(), tests.TestFn(P))
	})
}

func RunOutboxRemoveTests(t *testing.T) {
	t.Run("Remove", func(t *testing.T) {
		t.Skipf("TODO")
		mp := MockPlumbing{}
		P := mp.InitP(WithLocalMocks(Author))

		tests := TestSuite{}
		t.Run(tests.Label(), tests.TestFn(P))
	})
}
