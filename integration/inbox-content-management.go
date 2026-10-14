package integration

import (
	"testing"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/client"
	"github.com/go-ap/processing"
)

func RunInboxContentManagementTests(t *testing.T) {
	t.Run("Content Management", func(t *testing.T) {
		RunInboxUpdateTests(t)
	})
}

func RunInboxUpdateTests(t *testing.T) {
	mp := MockPlumbing{}

	original := vocab.Object{
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

	P := mp.InitP(
		WithMocks(Author, &original),
		processing.WithClient(client.New()),
	)

	updated := vocab.Object{
		ID:      original.ID,
		Content: vocab.DefaultLangValue("<p>Content</p>"),
	}

	tests := TestSuite{
		Name: "Update",
		Tests: []RunnableTest{
			TestPair{
				Name:       "Completely replace object",
				Author:     Author,
				ReceivedIn: vocab.Inbox.IRI(Author),
				Activity: vocab.Update{
					Type:   vocab.UpdateType,
					Actor:  Author.ID,
					Object: updated,
				},
				TestFns: []ItemErrorTest{
					HasID(mp.ShiftIRI),
					HasType(vocab.UpdateType),
					WasPublished(mp.PopTime),
					HasObject(
						HasID(original.ID),
						HasType(nil),
						WasUpdated(mp.PopTime),
						// Name was replaced with nil
						HasName(vocab.NaturalLanguageValues(nil)),
						// Summary was replaced with nil
						HasSummary(vocab.NaturalLanguageValues(nil)),
						// Content was updated
						HasContent(vocab.DefaultLangValue("<p>Content</p>")),
					),
				},
			},
		},
	}

	t.Run(tests.Label(), tests.TestFn(P))
}
