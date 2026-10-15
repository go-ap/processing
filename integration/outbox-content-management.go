package integration

import (
	"testing"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
)

func RunOutboxContentManagementTests(t *testing.T) {
	t.Run("Content Management", func(t *testing.T) {
		RunOutboxCreateTests(t)
		RunOutboxUpdateTests(t)
		RunOutboxDeleteTests(t)
	})
}

func RunOutboxCreateTests(t *testing.T) {
	mp := MockPlumbing{}
	P := mp.InitP(WithMocks(Author))

	tests := TestSuite{
		Name: "Create",
		Tests: []RunnableTest{
			TestPair{
				Name:       "Actor gets copied to Object.AttributedTo",
				Author:     Author,
				ReceivedIn: vocab.Outbox.IRI(Author),
				Activity: build.Create(
					build.WithActor(Author.ID),
					build.WithObject(build.Note(build.WithName("Lorem Ipsum"))),
				),
				TestFns: []ItemErrorTest{
					HasActor(HasID(Author.ID)),
					HasObject(HasAttributedTo(Author.ID)),
				},
			},
			TestPair{
				Name:       "Merge Recipients To/CC/Bto/BCC",
				Author:     Author,
				ReceivedIn: vocab.Outbox.IRI(Author),
				Activity: build.Create(
					build.WithType(vocab.CreateType),
					build.WithActor(Author.ID),
					build.WithTo(vocab.IRI("http://example.com/to/a1")),
					build.WithCC(vocab.IRI("http://example.com/cc/a1")),
					build.WithBto(vocab.IRI("http://example.com/bto/a1")),
					build.WithBCC(vocab.IRI("http://example.com/bcc/a1")),
					build.WithObject(build.Group(
						build.WithType(vocab.GroupType),
						build.WithName("Lorem"),
						build.WithTo(vocab.IRI("http://example.com/to/o1")),
						build.WithCC(vocab.IRI("http://example.com/cc/o1")),
						build.WithBto(vocab.IRI("http://example.com/bto/o1")),
						build.WithBCC(vocab.IRI("http://example.com/bcc/o1")),
						build.WithPreferredUsername("lipsum"),
					)),
				),
				TestFns: []ItemErrorTest{
					HasTo(vocab.IRIs{"http://example.com/to/a1", "http://example.com/to/o1"}),
					HasBto(vocab.IRIs{"http://example.com/bto/a1", "http://example.com/bto/o1"}),
					HasCC(vocab.IRIs{"http://example.com/cc/a1", "http://example.com/cc/o1"}),
					HasBCC(vocab.IRIs{"http://example.com/bcc/a1", "http://example.com/bcc/o1"}),
					HasObject(
						HasTo(vocab.IRIs{"http://example.com/to/a1", "http://example.com/to/o1"}),
						HasBto(vocab.IRIs{"http://example.com/bto/a1", "http://example.com/bto/o1"}),
						HasCC(vocab.IRIs{"http://example.com/cc/a1", "http://example.com/cc/o1"}),
						HasBCC(vocab.IRIs{"http://example.com/bcc/a1", "http://example.com/bcc/o1"}),
					),
				},
			},
		},
	}

	t.Run(tests.Label(), tests.TestFn(P))
}

func RunOutboxUpdateTests(t *testing.T) {
	mp := MockPlumbing{}

	noteID := RootID.AddPath("to-update")
	toUpdate := build.Note(
		build.WithID(noteID),
		build.WithName("Lipsum name"),
		build.WithSummary("Lipsum summary"),
		build.WithContent("<p>Lorem ipsum dolor sic amet</p>"),
		build.WithAttributedTo(AuthorID),
		build.WithMediaType("text/html"),
		build.WithPublished(time.Date(1984, time.February, 29, 0, 0, 0, 0, time.UTC)),
		build.WithReplies(),
		build.WithTo(vocab.ItemCollection{vocab.PublicNS}),
		build.WithSourceContent("Lorem ipsum dolor sic amet"),
		build.WithSourceMediaType("text/markdown"),
	)

	P := mp.InitP(WithMocks(Author, toUpdate))

	updated1 := toUpdate
	updated1.Content = vocab.DefaultLangValue("<p>Content</p>")

	tests := TestSuite{
		Name: "Update",
		Tests: []RunnableTest{
			TestPair{
				Name:       "Partial update",
				Author:     Author,
				ReceivedIn: vocab.Outbox.IRI(Author),
				Activity:   build.Update(build.WithActor(Author.ID), build.WithObject(updated1)),
				TestFns: []ItemErrorTest{
					HasID(mp.ShiftIRI),
					HasType(vocab.UpdateType),
					WasPublished(mp.PopTime),
					HasObject(
						WasUpdated(mp.PopTime),
						// Original name
						HasName(vocab.DefaultLangValue("Lipsum name")),
						// Original summary
						HasSummary(vocab.DefaultLangValue("Lipsum summary")),
						// Updated content
						HasContent(vocab.DefaultLangValue("<p>Content</p>")),
					),
				},
			},
		},
	}

	t.Run(tests.Label(), tests.TestFn(P))
}

func RunOutboxDeleteTests(t *testing.T) {
	mp := MockPlumbing{}

	toDelete := build.Article(
		build.WithID(RootID.AddPath("to-delete")),
		build.WithName("Lipsum name"),
		build.WithSummary("Lipsum summary"),
		build.WithContent("<p>Lorem ipsum dolor sic amet</p>"),
		build.WithAttributedTo(AuthorID),
		build.WithMediaType("text/html"),
		build.WithPublished(time.Date(1984, time.February, 29, 0, 0, 0, 0, time.UTC)),
		build.WithReplies(),
		build.WithTo(vocab.PublicNS),
		build.WithSourceContent("Lorem ipsum dolor sic amet"),
		build.WithSourceMediaType("text/markdown"),
	)
	P := mp.InitP(WithMocks(Author, toDelete))

	tests := TestSuite{
		Name: "Delete",
		Tests: []RunnableTest{
			TestPair{
				Name:       "Delete simple Article",
				Author:     Author,
				ReceivedIn: vocab.Outbox.IRI(Author),
				Activity:   build.Delete(build.WithActor(Author.ID), build.WithObject(toDelete.ID)),
				TestFns: []ItemErrorTest{
					HasID(mp.ShiftIRI),
					HasType(vocab.DeleteType),
					HasActor(HasID(Author.ID)),
					WasPublished(mp.PopTime),
					HasObject(
						HasID(toDelete.ID),
						HasType(vocab.TombstoneType),
						HasFormerType(toDelete.Type),
						HasTo(toDelete.To),
						WasDeleted(mp.PopTime),
					),
				},
			},
		},
	}

	t.Run(tests.Label(), tests.TestFn(P))
}
