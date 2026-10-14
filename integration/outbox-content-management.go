package integration

import (
	"testing"
	"time"

	vocab "github.com/go-ap/activitypub"
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
				Activity: vocab.Create{
					Type:  vocab.CreateType,
					Actor: Author.ID,
					Object: &vocab.Object{
						Type: vocab.NoteType,
						Name: vocab.DefaultLangValue("Lorem Ipsum"),
					},
				},
				TestFns: []ItemErrorTest{
					HasActor(Author.ID),
					HasObject(HasAttributedTo(Author.ID)),
				},
			},
			TestPair{
				Name:       "Merge Recipients To/CC/Bto/BCC",
				Author:     Author,
				ReceivedIn: vocab.Outbox.IRI(Author),
				Activity: vocab.Create{
					Type:  vocab.CreateType,
					Actor: Author.ID,
					To:    vocab.ItemCollection{vocab.IRI("http://example.com/to/a1")},
					CC:    vocab.ItemCollection{vocab.IRI("http://example.com/cc/a1")},
					Bto:   vocab.ItemCollection{vocab.IRI("http://example.com/bto/a1")},
					BCC:   vocab.ItemCollection{vocab.IRI("http://example.com/bcc/a1")},
					Object: &vocab.Group{
						Type:              vocab.GroupType,
						Name:              vocab.DefaultLangValue("Lorem"),
						To:                vocab.ItemCollection{vocab.IRI("http://example.com/to/o1")},
						CC:                vocab.ItemCollection{vocab.IRI("http://example.com/cc/o1")},
						Bto:               vocab.ItemCollection{vocab.IRI("http://example.com/bto/o1")},
						BCC:               vocab.ItemCollection{vocab.IRI("http://example.com/bcc/o1")},
						PreferredUsername: vocab.DefaultLangValue("lipsum"),
					},
				},
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

	toUpdate := vocab.Object{
		ID:           RootID.AddPath("to-update"),
		Type:         vocab.NoteType,
		Name:         vocab.DefaultLangValue("Lipsum name"),
		Summary:      vocab.DefaultLangValue("Lipsum summary"),
		Content:      vocab.DefaultLangValue("<p>Lorem ipsum dolor sic amet</p>"),
		AttributedTo: AuthorID,
		MediaType:    "text/html",
		Published:    time.Date(1984, time.February, 29, 0, 0, 0, 0, time.UTC),
		Replies:      vocab.Replies.IRI(RootID.AddPath("to-update")),
		To:           vocab.ItemCollection{vocab.PublicNS},
		Source: vocab.Source{
			Content:   vocab.DefaultLangValue("Lorem ipsum dolor sic amet"),
			MediaType: "text/markdown",
		},
	}

	P := mp.InitP(WithMocks(Author, &toUpdate))

	updated1 := toUpdate
	updated1.Name = vocab.DefaultLangValue("Name")
	updated1.Summary = vocab.DefaultLangValue("Summary")
	updated1.Content = vocab.DefaultLangValue("<p>Content</p>")
	updated1.Source.Content = vocab.DefaultLangValue("Content")

	tests := TestSuite{
		Name: "Update",
		Tests: []RunnableTest{
			TestPair{
				Name:       "Update Name/Summary/Content/Source",
				Author:     Author,
				ReceivedIn: vocab.Outbox.IRI(Author),
				Activity: vocab.Update{
					Type:   vocab.UpdateType,
					Actor:  Author.ID,
					Object: updated1,
				},
				TestFns: []ItemErrorTest{
					HasID(mp.ShiftIRI),
					HasType(vocab.UpdateType),
					HasActor(Author.ID),
					WasPublished(mp.PopTime),
					HasObject(
						HasID(updated1.ID),
						HasAttributedTo(Author.ID),
						WasUpdated(mp.PopTime),
						HasName(updated1.Name),
						HasSummary(updated1.Summary),
						HasContent(updated1.Content),
						HasSource(updated1.Source.Content, updated1.Source.MediaType),
					),
				},
			},
		},
	}

	t.Run(tests.Label(), tests.TestFn(P))
}

func RunOutboxDeleteTests(t *testing.T) {
	mp := MockPlumbing{}

	toDelete := vocab.Object{
		ID:           RootID.AddPath("to-delete"),
		Type:         vocab.ArticleType,
		Name:         vocab.DefaultLangValue("Lipsum name"),
		Summary:      vocab.DefaultLangValue("Lipsum summary"),
		Content:      vocab.DefaultLangValue("<p>Lorem ipsum dolor sic amet</p>"),
		AttributedTo: AuthorID,
		MediaType:    "text/html",
		Published:    time.Date(1984, time.February, 29, 0, 0, 0, 0, time.UTC),
		Replies:      vocab.Replies.IRI(RootID.AddPath("to-update")),
		To:           vocab.ItemCollection{vocab.PublicNS},
		Source: vocab.Source{
			Content:   vocab.DefaultLangValue("Lorem ipsum dolor sic amet"),
			MediaType: "text/markdown",
		},
	}
	P := mp.InitP(WithMocks(Author, toDelete))

	tests := TestSuite{
		Name: "Delete",
		Tests: []RunnableTest{
			TestPair{
				Name:       "Delete simple Article",
				Author:     Author,
				ReceivedIn: vocab.Outbox.IRI(Author),
				Activity: vocab.Update{
					Type:   vocab.DeleteType,
					Actor:  Author.ID,
					Object: toDelete.ID,
				},
				TestFns: []ItemErrorTest{
					HasID(mp.ShiftIRI),
					HasType(vocab.DeleteType),
					HasActor(Author.ID),
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
