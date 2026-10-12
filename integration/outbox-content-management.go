package integration

import (
	"testing"

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
				Name:       "Actor",
				Author:     Author,
				ReceivedIn: vocab.Outbox.IRI(Author),
				Activity: vocab.Create{
					Type:   vocab.CreateType,
					Actor:  Author.ID,
					Object: &vocab.Person{},
				},
				WantFn: func() vocab.Item {
					return &vocab.Create{
						ID:        mp.ShiftIRI(),
						Type:      vocab.CreateType,
						Actor:     Author.ID,
						Published: mp.PopTime(),
						Object: &vocab.Person{
							ID:           mp.ShiftIRI(),
							AttributedTo: Author.ID,
							Published:    mp.PopTime(),
						},
					}
				},
				WantErr: nil,
			},
		},
	}

	t.Run(tests.Label(), tests.TestFn(P))
}

func RunOutboxUpdateTests(t *testing.T) {
	mp := MockPlumbing{}
	P := mp.InitP(WithMocks(Author))

	tests := TestSuite{
		Name:  "Update",
		Tests: nil,
	}

	t.Run(tests.Label(), tests.TestFn(P))
}

func RunOutboxDeleteTests(t *testing.T) {
	mp := MockPlumbing{}
	P := mp.InitP(WithMocks(Author))

	tests := TestSuite{
		Name:  "Delete",
		Tests: nil,
	}

	t.Run(tests.Label(), tests.TestFn(P))
}
