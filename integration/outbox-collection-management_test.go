package integration

import (
	"testing"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
	"github.com/go-ap/processing"
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

		st := Storage(WithLocalMocks(Author, toAdd, target))
		p := mp.InitP(processing.WithStorage(st))
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
						RequestObject(HasID(noteID)),
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

		st := Storage(WithLocalMocks(Author))
		P := mp.InitP(processing.WithStorage(st))

		tests := TestSuite{}
		t.Run(tests.Label(), tests.TestFn(P))
	})
}

func Test_Outbox_Remove(t *testing.T) {
	t.Run("Remove", func(t *testing.T) {
		t.Skipf("TODO")
		mp := MockPlumbing{}
		st := Storage(WithLocalMocks(Author))
		P := mp.InitP(processing.WithStorage(st))

		tests := TestSuite{}
		t.Run(tests.Label(), tests.TestFn(P))
	})
}
