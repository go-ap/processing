package processing

import (
	"net/netip"
	"testing"

	pub "github.com/go-ap/activitypub"
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

func Test_defaultValidator_validateLocalIRI(t *testing.T) {
	tests := []struct {
		name    string
		arg     pub.IRI
		baseIRI pub.IRIs
		wantErr bool
	}{
		{
			name:    "IP 127.0.2.1",
			arg:     pub.IRI("https://127.0.2.1"),
			wantErr: false,
		},
		{
			name:    "IP 127.0.2.1 with port :8443",
			arg:     pub.IRI("https://127.0.2.1:8443"),
			wantErr: false,
		},
		{
			name:    "localhost",
			arg:     pub.IRI("https://localhost"),
			wantErr: false,
		},
		{
			name:    "example.com host",
			arg:     pub.IRI("https://example.com"),
			wantErr: true,
		},
		{
			name: "example.com host with set baseIRIs",
			baseIRI: pub.IRIs{
				pub.IRI("https://example.com"),
			},
			arg:     pub.IRI("https://example.com"),
			wantErr: false,
		},
		{
			name: "fedbox host with multiple baseIRIs",
			baseIRI: pub.IRIs{
				pub.IRI("http://localhost"),
				pub.IRI("http://fedbox"),
				pub.IRI("https://example.com"),
			},
			arg:     pub.IRI("https://fedbox"),
			wantErr: false,
		},
		{
			name: "example.com host with multiple baseIRIs",
			baseIRI: pub.IRIs{
				pub.IRI("http://localhost"),
				pub.IRI("http://fedbox"),
				pub.IRI("https://example1.com"),
			},
			arg:     pub.IRI("https://example.com"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := defaultValidator{
				addr: ipCache{
					addr: make(map[string][]netip.Addr),
				},
				baseIRI: tt.baseIRI,
				infoFn:  tInfFn(t),
				errFn:   tErrFn(t),
			}
			if err := v.validateLocalIRI(tt.arg); (err != nil) != tt.wantErr {
				t.Errorf("validateLocalIRI() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
