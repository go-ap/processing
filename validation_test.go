package processing

import (
	"sync"
	"testing"

	"git.sr.ht/~mariusor/lw"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/client"
)

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

var (
	tInfFn = func(t *testing.T) client.LogFn {
		return func(s string, el ...interface{}) {
			t.Logf(s, el...)
		}
	}
	tErrFn = func(t *testing.T) client.LogFn {
		return func(s string, el ...interface{}) {
			t.Errorf(s, el...)
		}
	}
)

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
	tInfFn(t.T)(s, a...)
}

func (t tLogger) Debugf(s string, a ...any) {
	tInfFn(t.T)(s, a...)
}

func (t tLogger) Infof(s string, a ...any) {
	tInfFn(t.T)(s, a...)
}

func (t tLogger) Warnf(s string, a ...any) {
	tErrFn(t.T)(s, a...)
}

func (t tLogger) Errorf(s string, a ...any) {
	tErrFn(t.T)(s, a...)
}

func (t tLogger) Critf(s string, a ...any) {
	tErrFn(t.T)(s, a...)
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
			wantErr: false,
		},
		{
			name:    "IP 127.0.2.1 with port :8443",
			arg:     vocab.IRI("https://127.0.2.1:8443"),
			wantErr: false,
		},
		{
			name:    "localhost",
			arg:     vocab.IRI("https://localhost"),
			wantErr: false,
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
	localAddressCache = ipCache{addr: sync.Map{}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := P{baseIRI: tt.baseIRI, l: tLogger{t}}

			if err := p.validateLocalIRI(tt.arg); (err != nil) != tt.wantErr {
				t.Errorf("validateLocalIRI() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
