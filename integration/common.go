package integration

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/processing"
	"github.com/google/go-cmp/cmp"
)

var (
	outbox bool = true
	inbox  bool = true
)

type TestPair struct {
	Name       string
	Author     vocab.Actor
	ReceivedIn vocab.IRI

	Activity vocab.Item

	WantFn  func() vocab.Item
	WantErr error
}

type TestSuite struct {
	Name  string
	Tests []RunnableTest
}

type RunnableTest interface {
	Label() string
	// TestFn is one way to run the tests directly from a func(*testing.T)
	TestFn(*processing.P) func(*testing.T)
}

func (tt TestPair) Label() string {
	return tt.Name
}

func (tt TestPair) TestFn(P *processing.P) func(*testing.T) {
	return func(t *testing.T) {
		act, err := P.ProcessActivity(tt.Activity, tt.Author, tt.ReceivedIn)
		if !cmp.Equal(err, tt.WantErr, EquateWeakErrors) {
			t.Errorf("ProcessActivity() error = %s", cmp.Diff(tt.WantErr, err, EquateWeakErrors))
		}
		if want := tt.WantFn(); !cmp.Equal(act, want, cmp.Comparer(vocab.ItemsEqual)) {
			t.Errorf("ProcessActivity() activity = %s", cmp.Diff(want, act, cmp.Comparer(vocab.ItemsEqual)))
		}
	}
}

func (ts TestSuite) Label() string {
	return ts.Name
}

func (ts TestSuite) TestFn(P *processing.P) func(t *testing.T) {
	return func(t *testing.T) {
		for _, tt := range ts.Tests {
			t.Run(tt.Label(), tt.TestFn(P))
		}
	}
}

func (ts TestSuite) Run(t *testing.T, P *processing.P) {
	for _, tt := range ts.Tests {
		t.Run(tt.Label(), tt.TestFn(P))
	}
}

var (
	_, ed2559Key, _ = ed25519.GenerateKey(rand.Reader)

	RootID = vocab.IRI("http://example.com")

	AuthorID = RootID.AddPath("~jdoe")

	Author = vocab.Actor{
		ID:                AuthorID,
		Type:              vocab.PersonType,
		Name:              vocab.DefaultLangValue("Jane Doe"),
		Published:         time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC),
		Audience:          vocab.ItemCollection{vocab.PublicNS},
		Inbox:             vocab.Inbox.IRI(AuthorID),
		Outbox:            vocab.Outbox.IRI(AuthorID),
		Following:         vocab.Following.IRI(AuthorID),
		Followers:         vocab.Followers.IRI(AuthorID),
		Liked:             vocab.Liked.IRI(AuthorID),
		Likes:             vocab.Likes.IRI(AuthorID),
		Replies:           vocab.Replies.IRI(AuthorID),
		Shares:            vocab.Shares.IRI(AuthorID),
		PreferredUsername: vocab.DefaultLangValue("jdoe"),
		Endpoints: &vocab.Endpoints{
			SharedInbox: vocab.Inbox.IRI(RootID),
			ProxyURL:    RootID.AddPath("proxy"),
		},
		PublicKey: PublicKey(ed2559Key.Public(), AuthorID),
	}
)

func PublicKey(k any, id vocab.IRI) vocab.PublicKey {
	pubEnc, _ := x509.MarshalPKIXPublicKey(k)
	pubEncoded := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubEnc})

	return vocab.PublicKey{
		ID:           id + "#main",
		Owner:        id,
		PublicKeyPem: string(pubEncoded),
	}
}

func areErrors(a, b any) bool {
	_, ok1 := a.(error)
	_, ok2 := b.(error)
	return ok1 && ok2
}

func compareErrors(x, y any) bool {
	xe := x.(error)
	ye := y.(error)
	return (errors.Is(xe, ye) || errors.Is(ye, xe)) && xe.Error() == ye.Error()
}

var EquateWeakErrors = cmp.FilterValues(areErrors, cmp.Comparer(compareErrors))
