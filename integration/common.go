package integration

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/filters"
	"github.com/go-ap/processing"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

	TestFns TestAssertions
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
		for _, testFn := range tt.TestFns {
			testFn.Run(t, act, err)
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

type itemCheckFn func(*testing.T, vocab.Item)

func (icFn itemCheckFn) Run(t *testing.T, it vocab.Item, _ error) {
	icFn(t, it)
}

type errCheckFn func(*testing.T, error)

func (eFn errCheckFn) Run(t *testing.T, _ vocab.Item, err error) {
	eFn(t, err)
}

type ItemErrorTest interface {
	Run(t *testing.T, _ vocab.Item, err error)
}

type TestAssertions []ItemErrorTest

func HasError(wantErr error) errCheckFn {
	return func(t *testing.T, err error) {
		t.Run("Error", func(t *testing.T) {
			if !cmp.Equal(err, wantErr, EquateWeakErrors) {
				t.Errorf("Unexpected error = %s", cmp.Diff(wantErr, err, EquateWeakErrors))
			}
		})
	}
}

func HasID(maybeIRI any) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		want := iriArg(maybeIRI)
		t.Run("ID", func(t *testing.T) {
			if got := it.GetID(); !want.Equal(got) {
				t.Errorf("Received %s, expected %s", got, want)
			}
		})
	}
}

func HasInbox(want vocab.IRI) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		t.Run("Inbox", func(t *testing.T) {
			if got := vocab.Inbox.IRI(it); !got.Equal(want) {
				t.Errorf("Received %s, expected %s", got, want)
			}
		})
	}
}

func HasOutbox(want vocab.IRI) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		t.Run("Outbox", func(t *testing.T) {
			if got := vocab.Outbox.IRI(it); !got.Equal(want) {
				t.Errorf("Received %s, expected %s", got, want)
			}
		})
	}
}

func iriArg(maybeIRI any) vocab.IRI {
	switch ii := maybeIRI.(type) {
	case vocab.IRI:
		return ii
	case vocab.Item:
		return ii.GetLink()
	case func() vocab.Item:
		return ii().GetLink()
	case func() vocab.IRI:
		return ii()
	default:
	}
	panic(fmt.Errorf("%T argument can't be converted to an %T", maybeIRI, vocab.IRI("")))
}

func HasReplies(maybeIRI any) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		want := iriArg(maybeIRI)
		t.Run("Replies", func(t *testing.T) {
			if got := vocab.Replies.IRI(it); !got.Equal(want) {
				t.Errorf("Received %s, expected %s", got, want)
			}
		})
	}
}

func HasType(typ vocab.Typer) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		t.Run("Type", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if !vocab.TypesEqual(ob.Type, typ) {
					t.Errorf("Received %v, expected %v", ob.Type, typ)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Object: %+v", err)
			}
		})
	}
}

func HasFormerType(typ vocab.Typer) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		t.Run("FormerType", func(*testing.T) {
			err := vocab.OnTombstone(it, func(ts *vocab.Tombstone) error {
				if !vocab.TypesEqual(ts.FormerType, typ) {
					t.Errorf("Received %s, expected %s", ts.FormerType, typ)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Tombstone: %+v", err)
			}
		})
	}
}

func nlv[T ~string | vocab.NaturalLanguageValues](c T) vocab.NaturalLanguageValues {
	var result vocab.NaturalLanguageValues
	switch v := any(c).(type) {
	case string:
		result = vocab.DefaultLangValue(v)
	case []byte:
		result = vocab.DefaultLangValue(string(v))
	case vocab.NaturalLanguageValues:
		result = v
	}
	return result
}

func HasPreferredUsername[T ~string | vocab.NaturalLanguageValues](name T) itemCheckFn {
	v := nlv(name)
	return func(t *testing.T, it vocab.Item) {
		t.Run("PreferredUsername", func(t *testing.T) {
			err := vocab.OnActor(it, func(act *vocab.Actor) error {
				if !cmp.Equal(v, act.PreferredUsername) {
					t.Errorf("Diff %s", cmp.Diff(v, act.PreferredUsername))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Failed Actor %s check: %v", it.GetID(), err)
			}
		})
	}
}

func HasSummary[T ~string | vocab.NaturalLanguageValues](cont T) itemCheckFn {
	v := nlv(cont)
	return func(t *testing.T, it vocab.Item) {
		t.Run("Summary", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if !cmp.Equal(v, ob.Summary) {
					t.Errorf("Diff %s", cmp.Diff(v, ob.Summary))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Failed Object %s check: %v", it.GetID(), err)
			}
		})
	}
}

func HasName[T ~string | vocab.NaturalLanguageValues](cont T) itemCheckFn {
	v := nlv(cont)
	return func(t *testing.T, it vocab.Item) {
		t.Run("Name", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if !cmp.Equal(v, ob.Name) {
					t.Errorf("Diff %s", cmp.Diff(v, ob.Name))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Failed Object %s check: %v", it.GetID(), err)
			}
		})
	}
}

func HasContent[T ~string | vocab.NaturalLanguageValues](cont T) itemCheckFn {
	v := nlv(cont)
	return func(t *testing.T, it vocab.Item) {
		t.Run("Content", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if !cmp.Equal(v, ob.Content) {
					t.Errorf("Diff %s", cmp.Diff(v, ob.Content))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Failed Object %s check: %v", it.GetID(), err)
			}
		})
	}
}

func HasMediaType(mt vocab.MimeType) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		t.Run("MediaType", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if !cmp.Equal(mt, ob.MediaType) {
					t.Errorf("Diff %s", cmp.Diff(mt, ob.MediaType))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Failed Object %s check: %v", it.GetID(), err)
			}
		})
	}
}

func HasSource[T ~string | vocab.NaturalLanguageValues](cont T, mt vocab.MimeType) itemCheckFn {
	v := nlv(cont)
	return func(t *testing.T, it vocab.Item) {
		t.Run("Source", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if !cmp.Equal(v, ob.Source.Content) {
					t.Errorf("Content Diff %s", cmp.Diff(v, ob.Source.Content))
				}
				if !cmp.Equal(mt, ob.Source.MediaType) {
					t.Errorf("MediaType Diff %s", cmp.Diff(mt, ob.Source.MediaType))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Failed Object %s check: %v", it.GetID(), err)
			}
		})
	}
}

func toNormalizedItemCol(it vocab.Item) vocab.ItemCollection {
	col := make(vocab.ItemCollection, 0)
	_ = vocab.OnItem(it, func(item vocab.Item) error {
		return col.Append(item)
	})
	return col
}

func HasTo(to ...vocab.Item) itemCheckFn {
	ti := toNormalizedItemCol(vocab.ItemCollection(to))
	return func(t *testing.T, it vocab.Item) {
		t.Run("To", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if tt := toNormalizedItemCol(ob.To); !cmp.Equal(ti, tt) {
					t.Errorf("Received %s", cmp.Diff(ti, tt))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Object: %+v", err)
			}
		})
	}
}

func HasCC(cc ...vocab.Item) itemCheckFn {
	ci := toNormalizedItemCol(vocab.ItemCollection(cc))
	return func(t *testing.T, it vocab.Item) {
		t.Run("CC", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if ccc := toNormalizedItemCol(ob.CC); !cmp.Equal(ci, ccc) {
					t.Errorf("Received %s", cmp.Diff(ci, ccc))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Object: %+v", err)
			}
		})
	}
}

func HasBto(to ...vocab.Item) itemCheckFn {
	ti := toNormalizedItemCol(vocab.ItemCollection(to))
	return func(t *testing.T, it vocab.Item) {
		t.Run("Bto", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if tt := toNormalizedItemCol(ob.Bto); !cmp.Equal(ti, tt) {
					t.Errorf("Received %s", cmp.Diff(ti, tt))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Object: %+v", err)
			}
		})
	}
}

func HasBCC(to ...vocab.Item) itemCheckFn {
	ti := toNormalizedItemCol(vocab.ItemCollection(to))
	return func(t *testing.T, it vocab.Item) {
		t.Run("BCC", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if tt := toNormalizedItemCol(ob.BCC); !cmp.Equal(ti, tt) {
					t.Errorf("Received %s", cmp.Diff(ti, tt))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Object: %+v", err)
			}
		})
	}
}

func WasUpdated(maybeTime any) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		want := timeArg(maybeTime)
		t.Run("Updated", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if !cmp.Equal(want, ob.Updated, cmpopts.EquateApproxTime(fudgeDuration)) {
					t.Errorf("Failed Updated date check for %s, received %s, expected %s", ob.ID, ob.Updated, want)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Object: %+v", err)
			}
		})
	}
}

// NOTE(marius): this is becoming slightly ridiculous
const fudgeDuration = time.Minute

func timeArg(maybeTime any) time.Time {
	switch tt := maybeTime.(type) {
	case time.Time:
		return tt
	case func() time.Time:
		return tt()
	default:
	}
	panic(fmt.Errorf("%T argument can't be converted to an %T", maybeTime, time.Time{}))
}

func WasPublished(maybeTime any) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		want := timeArg(maybeTime)
		t.Run("Published", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if !cmp.Equal(want, ob.Published, cmpopts.EquateApproxTime(fudgeDuration)) {
					t.Errorf("Received %s, expected %s", ob.Published, want)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Object: %+v", err)
			}
		})
	}
}

func WasDeleted(maybeTime any) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		want := timeArg(maybeTime)
		t.Run("Deleted", func(*testing.T) {
			err := vocab.OnTombstone(it, func(ts *vocab.Tombstone) error {
				if !cmp.Equal(want, ts.Deleted, cmpopts.EquateApproxTime(fudgeDuration)) {
					t.Errorf("Received %s, expected %s", ts.Deleted, want)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Object: %+v", err)
			}
		})
	}
}

func HasAudience(u ...vocab.Item) itemCheckFn {
	ui := toNormalizedItemCol(vocab.ItemCollection(u))
	return func(t *testing.T, it vocab.Item) {
		t.Run("Audience", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if aud := toNormalizedItemCol(ob.Audience); !cmp.Equal(aud, ui) {
					t.Errorf("%s", cmp.Diff(ui, aud))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Object: %+v", err)
			}
		})
	}
}

func HasInReplyTo(u ...vocab.Item) itemCheckFn {
	ui := toNormalizedItemCol(vocab.ItemCollection(u))
	return func(t *testing.T, it vocab.Item) {
		t.Run("InReplyTo", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if ir := toNormalizedItemCol(ob.InReplyTo); !cmp.Equal(ir, ui) {
					t.Errorf("%s", cmp.Diff(ui, ir))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Object: %+v", err)
			}
		})
	}
}

func HasAttributedTo(maybeItems ...any) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		checks := itemCheckArgs(maybeItems...)
		t.Run("AttributedTo", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				for _, fn := range checks {
					fn(t, ob.AttributedTo)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Object: %+v", err)
			}
		})
	}
}

func HasContext(u ...vocab.Item) itemCheckFn {
	ui := toNormalizedItemCol(vocab.ItemCollection(u))
	return func(t *testing.T, it vocab.Item) {
		t.Run("Context", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if at := toNormalizedItemCol(ob.Context); !cmp.Equal(at, ui) {
					t.Errorf("%s", cmp.Diff(ui, at))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Object: %+v", err)
			}
		})
	}
}

func HasURL(u ...vocab.Item) itemCheckFn {
	ui := toNormalizedItemCol(vocab.ItemCollection(u))
	return func(t *testing.T, it vocab.Item) {
		t.Run("URL", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if uu := toNormalizedItemCol(ob.URL); !cmp.Equal(uu, ui) {
					t.Errorf("%s", cmp.Diff(ui, uu))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Object: %+v", err)
			}
		})
	}
}

func HasTag(u ...vocab.Item) itemCheckFn {
	ui := toNormalizedItemCol(vocab.ItemCollection(u))
	return func(t *testing.T, it vocab.Item) {
		t.Run("Tag", func(t *testing.T) {
			err := vocab.OnObject(it, func(ob *vocab.Object) error {
				if tag := toNormalizedItemCol(ob.Tag); !cmp.Equal(tag, ui) {
					t.Errorf("%s", cmp.Diff(ui, tag))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Object: %+v", err)
			}
		})
	}
}

func HasProxyURL(iri vocab.IRI) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		t.Run("ProxyURL", func(t *testing.T) {
			err := vocab.OnActor(it, func(act *vocab.Actor) error {
				if act.Endpoints == nil {
					t.Errorf("Actor's Endpoints is nil")
					return nil
				}
				if !cmp.Equal(iri, act.Endpoints.ProxyURL) {
					t.Errorf("Received %s, expected %s", act.Endpoints.ProxyURL, iri)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Actor: %+v", err)
			}
		})
	}
}

func HasSharedInbox(iri vocab.Item) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		t.Run("SharedInbox", func(t *testing.T) {
			err := vocab.OnActor(it, func(act *vocab.Actor) error {
				if act.Endpoints == nil {
					t.Errorf("Actor's Endpoints is nil")
					return nil
				}
				if !cmp.Equal(iri, act.Endpoints.SharedInbox) {
					t.Errorf("Received %s, expected %s", act.Endpoints.SharedInbox, iri)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Actor: %+v", err)
			}
		})
	}
}

func HasPublicKey(pub vocab.PublicKey) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		t.Run("PublicKey", func(t *testing.T) {
			err := vocab.OnActor(it, func(act *vocab.Actor) error {
				if !cmp.Equal(act.PublicKey, pub) {
					t.Errorf("Received %s", cmp.Diff(pub, act.PublicKey))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Actor: %+v", err)
			}
		})
	}
}

func HasStreams(maybeItems ...any) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		checks := itemCheckArgs(maybeItems...)
		t.Run("Streams", func(t *testing.T) {
			err := vocab.OnActor(it, func(act *vocab.Actor) error {
				for _, check := range checks {
					check(t, act.Streams)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Actor: %+v", err)
			}
		})
	}
}

func HasEndpoints(e *vocab.Endpoints) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		t.Run("Endpoints", func(t *testing.T) {
			err := vocab.OnActor(it, func(act *vocab.Actor) error {
				if !cmp.Equal(act.Endpoints, e) {
					t.Errorf("%s", cmp.Diff(e, act.Endpoints))
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Actor: %+v", err)
			}
		})
	}
}

func HasActor(maybeItems ...any) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		checks := itemCheckArgs(maybeItems...)
		t.Run("Actor", func(t *testing.T) {
			err := vocab.OnIntransitiveActivity(it, func(act *vocab.IntransitiveActivity) error {
				for _, check := range checks {
					check(t, act.Actor)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid IntransitiveActivity: %+v", err)
			}
		})
	}
}

func itemCheckArgs(maybeItems ...any) []itemCheckFn {
	items := make([]itemCheckFn, 0, len(maybeItems))
	for _, maybeItem := range maybeItems {
		switch ii := maybeItem.(type) {
		case []any:
			items = append(items, itemCheckArgs(ii...)...)
		case vocab.IRI:
			items = append(items, equalFlattened(ii))
		case vocab.IRIs:
			items = append(items, equalFlattened(ii))
		case vocab.Item:
			items = append(items, equalFlattened(ii))
		case vocab.ItemCollection:
			items = append(items, equalFlattened(ii))
		case func() vocab.Item:
			items = append(items, equalFlattened(ii()))
		case func() vocab.IRI:
			items = append(items, equalFlattened(ii()))
		case func() vocab.ItemCollection:
			items = append(items, equalFlattened(ii()))
		case func() vocab.IRIs:
			items = append(items, equalFlattened(ii()))
		case itemCheckFn:
			items = append(items, ii)
		default:
			panic(fmt.Errorf("%T argument can't be converted to an %T", maybeItem, itemCheckFn(nil)))
		}
	}
	return items
}

func equalFlattened(w vocab.Item) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		want := toNormalizedItemCol(w)
		got := toNormalizedItemCol(it)
		if !cmp.Equal(got, want) {
			t.Errorf("Received %s", cmp.Diff(want, got))
		}
	}
}

func HasObject(maybeItems ...any) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		checks := itemCheckArgs(maybeItems...)
		t.Run("Object", func(t *testing.T) {
			err := vocab.OnActivity(it, func(act *vocab.Activity) error {
				for _, check := range checks {
					check(t, act.Object)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Activity: %+v", err)
			}
		})
	}
}

func HasOrigin(maybeItems ...any) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		checks := itemCheckArgs(maybeItems...)
		t.Run("Origin", func(t *testing.T) {
			err := vocab.OnIntransitiveActivity(it, func(act *vocab.IntransitiveActivity) error {
				for _, check := range checks {
					check(t, act.Origin)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Activity: %+v", err)
			}
		})
	}
}

func HasTarget(maybeItems ...any) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		checks := itemCheckArgs(maybeItems...)
		t.Run("Target", func(t *testing.T) {
			err := vocab.OnIntransitiveActivity(it, func(act *vocab.IntransitiveActivity) error {
				for _, check := range checks {
					check(t, act.Target)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Activity: %+v", err)
			}
		})
	}
}

func HasAnyOf(maybeItems ...any) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		checks := itemCheckArgs(maybeItems...)
		t.Run("AnyOf", func(t *testing.T) {
			err := vocab.OnQuestion(it, func(act *vocab.Question) error {
				for _, check := range checks {
					check(t, act.AnyOf)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Question: %+v", err)
			}
		})
	}
}

func HasOneOf(maybeItems ...any) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		checks := itemCheckArgs(maybeItems...)
		t.Run("OneOf", func(t *testing.T) {
			err := vocab.OnQuestion(it, func(act *vocab.Question) error {
				for _, check := range checks {
					check(t, act.OneOf)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Question: %+v", err)
			}
		})
	}
}

func HasTotalItems(cnt int) itemCheckFn {
	return func(t *testing.T, it vocab.Item) {
		t.Run("TotalItems", func(t *testing.T) {
			err := vocab.OnOrderedCollection(it, func(col *vocab.OrderedCollection) error {
				if int(col.TotalItems) != cnt {
					t.Errorf("Received %d, expected %d", col.TotalItems, cnt)
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Collection: %+v", err)
			}
		})
	}
}

func DoesNotHaveItem(it vocab.Item) itemCheckFn {
	return func(t *testing.T, got vocab.Item) {
		t.Run("Does not contain: "+string(it.GetLink()), func(t *testing.T) {
			err := vocab.OnOrderedCollection(got, func(col *vocab.OrderedCollection) error {
				gotItems := col.OrderedItems
				maybeFound, _ := filters.Checks{filters.SameID(it.GetID())}.Run(gotItems).(vocab.ItemCollection)
				if maybeFound.Count() > 0 {
					t.Errorf("Item does exist in the collection: %s", it.GetLink())
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Collection: %+v", err)
			}
		})
	}
}

func HasItem(it vocab.Item) itemCheckFn {
	return func(t *testing.T, got vocab.Item) {
		t.Run("Contains: "+string(it.GetLink()), func(t *testing.T) {
			err := vocab.OnCollectionIntf(got, func(col vocab.CollectionInterface) error {
				gotItems := col.Collection()
				maybeFound, _ := filters.Checks{filters.SameID(it.GetID())}.Run(gotItems).(vocab.ItemCollection)
				if maybeFound.Count() == 0 {
					t.Errorf("Item does not exist in collection: %s", it.GetLink())
					return nil
				}
				gotIt := maybeFound.First()
				if !it.GetLink().Equal(gotIt.GetLink()) {
					t.Errorf("Item differs: %s", cmp.Diff(it.GetLink(), gotIt.GetLink()))
					return nil
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Collection: %+v", err)
			}
		})
	}
}

func HasExactItems(items ...vocab.Item) itemCheckFn {
	return func(t *testing.T, got vocab.Item) {
		t.Run("Items", func(t *testing.T) {
			err := vocab.OnOrderedCollection(got, func(col *vocab.OrderedCollection) error {
				gotItems := col.OrderedItems
				if len(gotItems) != len(items) {
					t.Errorf("The amount of received items %d does not match the expected count: %d", len(gotItems), len(items))
					return nil
				}
				for i, it := range items {
					maybeFound, _ := filters.Checks{filters.SameID(it.GetID())}.Run(gotItems).(vocab.ItemCollection)
					if maybeFound.Count() == 0 {
						t.Errorf("Item at pos %d does not exist in collection: %s", i, it.GetLink())
						return nil
					}
					gotIt := maybeFound.First()
					if !it.GetLink().Equal(gotIt.GetLink()) {
						t.Errorf("Item at pos %d differs: %s", i, cmp.Diff(it.GetLink(), gotIt.GetLink()))
						return nil
					}
				}
				return nil
			})
			if err != nil {
				t.Errorf("Invalid Collection: %+v", err)
			}
		})
	}
}
