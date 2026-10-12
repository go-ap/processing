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
				Name:       "Actor without ID",
				Author:     Author,
				ReceivedIn: vocab.Outbox.IRI(Author),
				Activity: vocab.Create{
					Type:  vocab.CreateType,
					Actor: Author.ID,
					Object: &vocab.Person{
						Name:              vocab.DefaultLangValue("Lorem"),
						PreferredUsername: vocab.DefaultLangValue("lipsum"),
					},
				},
				WantFn: func() vocab.Item {
					return &vocab.Create{
						ID:        mp.ShiftIRI(),
						Type:      vocab.CreateType,
						Actor:     Author.ID,
						Published: mp.PopTime(),
						Object: &vocab.Person{
							ID:                mp.ShiftIRI(),
							AttributedTo:      Author.ID,
							Published:         mp.PopTime(),
							Name:              vocab.DefaultLangValue("Lorem"),
							PreferredUsername: vocab.DefaultLangValue("lipsum"),
						},
					}
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
				WantFn: func() vocab.Item {
					upd := &vocab.Update{
						ID:        mp.ShiftIRI(),
						Type:      vocab.UpdateType,
						Actor:     Author.ID,
						Published: mp.PopTime(),
					}
					updated1.Updated = mp.PopTime()
					upd.Object = &updated1
					return upd
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
				WantFn: func() vocab.Item {
					del := &vocab.Delete{
						ID:        mp.ShiftIRI(),
						Type:      vocab.DeleteType,
						Actor:     Author.ID,
						Published: mp.PopTime(),
						Object: &vocab.Tombstone{
							ID:         toDelete.ID,
							Type:       vocab.TombstoneType,
							To:         toDelete.To,
							FormerType: toDelete.Type,
							Deleted:    mp.PopTime(),
						},
					}
					return del
				},
			},
		},
	}

	t.Run(tests.Label(), tests.TestFn(P))
}
