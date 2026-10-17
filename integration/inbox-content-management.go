package integration

import (
	"testing"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
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

	original := build.Note(
		build.WithID(RootID.AddPath("to-update")),
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

	P := mp.InitP(
		WithMocks(Author, original),
		processing.WithClient(client.New()),
	)

	updated := build.Object(
		build.WithID(original.ID),
		build.WithContent("<p>Content</p>"),
	)

	tests := TestSuite{
		Name: "Update",
		Tests: []RunnableTest{
			TestPair{
				Name:       "Completely replace object",
				Author:     *Author,
				ReceivedIn: vocab.Inbox.IRI(Author),
				Activity:   build.Update(build.WithActor(Author.ID), build.WithObject(updated)),
				TestFns: []ItemErrorTest{
					HasID(mp.ShiftIRI),
					HasType(vocab.UpdateType),
					WasPublished(mp.PopTime),
					HasObject(
						HasID(original.ID),
						HasType(nil),
						WasUpdated(mp.PopTime),
						// Name was replaced with nil
						HasName[vocab.NaturalLanguageValues](nil),
						// Summary was replaced with nil
						HasSummary[vocab.NaturalLanguageValues](nil),
						// Content was updated
						HasContent(vocab.DefaultLangValue("<p>Content</p>")),
					),
				},
			},
		},
	}

	t.Run(tests.Label(), tests.TestFn(P))
}
