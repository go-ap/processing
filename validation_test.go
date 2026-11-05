package processing

import (
	"fmt"
	"testing"

	"git.sr.ht/~mariusor/lw"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/activitypub/build"
	conformance "github.com/go-ap/storage-conformance-suite"
)

func ExampleP_ValidateActivity_in_inbox() {
	allIRIsAreLocal := func(_ vocab.IRI) bool { return true }

	storage := new(conformance.MemStorage)

	v := New(
		WithStorage(storage),
		// No IRIs are local for this example.
		WithLocalIRIChecker(allIRIsAreLocal),
		WithIDGenerator(testIDGenerator()),
		// The client is used when it's required to dereference properties of the
		// incoming that are referred only by their IDs into full ActivityPub
		// objects. Some instances require that remote fetching is done using
		// requests containing valid HTTP-Signatures, so take that into
		// consideration. See the go-ap/client module for how to initialize it for
		// such cases.
		WithClient(mockClient()),
	)

	local := build.Person(
		build.WithID("http://social.example.com/alice"),
		build.WithInbox(vocab.IRI("http://social.example.com/alice/inbox")),
		build.WithOutbox(vocab.IRI("http://social.example.com/alice/outbox")),
	)
	// We save the receiving actor and its collections locally
	inbox := build.OrderedCollection(build.WithID(vocab.Inbox.IRI(local)))
	outbox := build.OrderedCollection(build.WithID(vocab.Outbox.IRI(local)))
	_, _ = storage.Save(local)
	_, _ = storage.Save(inbox)
	_, _ = storage.Save(outbox)

	// This is an actor extracted from the HTTP request authorization
	// headers.
	// For how to do that see the go-ap/authorize module, which can extract
	// HTTP-Signatures, and OAuth2 bearer tokens.
	authorized := build.Person(
		build.WithID("http://example.com/~jdoe"),
		build.WithInbox(vocab.IRI("http://example.com/~jdoe/inbox")),
		build.WithOutbox(vocab.IRI("http://example.com/~jdoe/outbox")),
	)

	// The authorized actor should be available locally to avoid an
	// outbound fetch request for it.
	_, _ = storage.Save(authorized)

	// The incoming activity doesn't have an ID nor an Object,
	// so it will fail validation.
	activity := build.Create(
		build.WithActor(authorized),
	)

	// The processor needs to know the URL on which the current request has
	// been operated on.
	// As this is a Social API request, it needs to correspond to the
	// authorized actor's /outbox.
	receivedIn := vocab.Inbox.IRI(authorized)

	err := v.ValidateActivity(activity, *authorized, receivedIn)
	fmt.Printf("error: %v\n", err)

	// Output:
	// error: Activity is not valid: empty activity id
}

func ExampleP_ValidateActivity_in_outbox() {
	allIRIsAreLocal := func(_ vocab.IRI) bool { return true }

	storage := new(conformance.MemStorage)
	p := New(
		WithStorage(storage),
		// We know that all relevant IRIs are local.
		WithLocalIRIChecker(allIRIsAreLocal),
		WithIDGenerator(testIDGenerator()),
	)

	// This is an actor extracted from the HTTP request authorization
	// headers.
	// For how to do that see the go-ap/authorize module, which can extract
	// HTTP-Signatures, and OAuth2 bearer tokens.
	actor := build.Person(
		build.WithID("http://example.com/~jdoe"),
		build.WithOutbox(vocab.IRI("http://example.com/~jdoe/outbox")),
	)
	outbox := build.OrderedCollection(build.WithID(vocab.Outbox.IRI(actor)))
	inbox := build.OrderedCollection(build.WithID(vocab.Inbox.IRI(actor)))
	// We save the relevant actor and its collections locally
	_, _ = storage.Save(actor)
	_, _ = storage.Save(outbox)
	_, _ = storage.Save(inbox)

	// The incoming activity is valid as a client to server activity.
	activity := build.Create(
		build.WithActor(actor),
		build.WithObject(
			build.Note(build.WithID("http://example.com/~jdoe/notes/1")),
		),
	)

	// The processor needs to know the URL on which the current request has
	// been operated on.
	// As this is a Social API request, it needs to correspond to the
	// authorized actor's /outbox.
	receivedIn := vocab.Outbox.IRI(actor)

	err := p.ValidateActivity(activity, *actor, receivedIn)
	fmt.Printf("error: %v\n", err)

	// Output:
	// error: <nil>
}

func TestActivityValidatorCtxt(t *testing.T) {
	t.Skipf("TODO")
}

func TestGenericValidator_IsLocalIRI(t *testing.T) {
	t.Skipf("TODO")
}

func TestGenericValidator_ValidateActivity(t *testing.T) {
	t.Skipf("TODO")
}

func TestGenericValidator_ValidateActor(t *testing.T) {
	t.Skipf("TODO")
}

func TestGenericValidator_ValidateAudience(t *testing.T) {
	t.Skipf("TODO")
}

func TestGenericValidator_ValidateLink(t *testing.T) {
	t.Skipf("TODO")
}

func TestGenericValidator_ValidateObject(t *testing.T) {
	t.Skipf("TODO")
}

func TestGenericValidator_ValidateTarget(t *testing.T) {
	t.Skipf("TODO")
}

type tLogger struct {
	*testing.T
}

func (t tLogger) WithContext(ctx ...lw.Ctx) lw.Logger {
	return t
}

func (t tLogger) New(c ...lw.Ctx) lw.Logger {
	return t
}

func (t tLogger) Tracef(s string, a ...any) {
	t.T.Logf(s, a...)
}

func (t tLogger) Debugf(s string, a ...any) {
	t.T.Logf(s, a...)
}

func (t tLogger) Infof(s string, a ...any) {
	t.T.Logf(s, a...)
}

func (t tLogger) Warnf(s string, a ...any) {
	t.T.Errorf(s, a...)
}

func (t tLogger) Errorf(s string, a ...any) {
	t.T.Errorf(s, a...)
}

func (t tLogger) Critf(s string, a ...any) {
	t.T.Errorf(s, a...)
}

var _ lw.Logger = tLogger{}

func Test_defaultValidator_validateLocalIRI(t *testing.T) {
	tests := []struct {
		name    string
		arg     vocab.IRI
		baseIRI vocab.IRIs
		wantErr bool
	}{
		{
			name:    "IP 127.0.2.1",
			arg:     vocab.IRI("https://127.0.2.1"),
			wantErr: true,
		},
		{
			name:    "IP 127.0.2.1 with port :8443",
			arg:     vocab.IRI("https://127.0.2.1:8443"),
			wantErr: true,
		},
		{
			name:    "localhost",
			arg:     vocab.IRI("https://localhost"),
			wantErr: true,
		},
		{
			name:    "example.com host",
			arg:     vocab.IRI("https://example.com"),
			wantErr: true,
		},
		{
			name: "example.com host with set baseIRIs",
			baseIRI: vocab.IRIs{
				vocab.IRI("https://example.com"),
			},
			arg:     vocab.IRI("https://example.com"),
			wantErr: false,
		},
		{
			name: "fedbox host with multiple baseIRIs",
			baseIRI: vocab.IRIs{
				vocab.IRI("http://localhost"),
				vocab.IRI("http://fedbox"),
				vocab.IRI("https://example.com"),
			},
			arg:     vocab.IRI("https://fedbox"),
			wantErr: false,
		},
		{
			name: "example.com host with multiple baseIRIs",
			baseIRI: vocab.IRIs{
				vocab.IRI("http://localhost"),
				vocab.IRI("http://fedbox"),
				vocab.IRI("https://example1.com"),
			},
			arg:     vocab.IRI("https://example.com"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := P{baseIRI: tt.baseIRI, l: tLogger{t}}

			if err := p.validateLocalIRI(tt.arg); (err != nil) != tt.wantErr {
				t.Errorf("validateLocalIRI() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
