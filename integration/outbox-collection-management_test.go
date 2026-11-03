package integration

import (
	"testing"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
)

func Test_Outbox_Add(t *testing.T) {
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
					Name:       "Object to Target",
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

func Test_Outbox_Move(t *testing.T) {
	t.Run("Move", func(t *testing.T) {
		t.Skipf("TODO")
		mp := MockPlumbing{}
		P := mp.InitP(WithLocalMocks(Author))

		tests := TestSuite{}
		t.Run(tests.Label(), tests.TestFn(P))
	})
}

func Test_Outbox_Remove(t *testing.T) {
	t.Run("Remove", func(t *testing.T) {
		t.Skipf("TODO")
		mp := MockPlumbing{}
		P := mp.InitP(WithLocalMocks(Author))

		tests := TestSuite{}
		t.Run(tests.Label(), tests.TestFn(P))
	})
}
