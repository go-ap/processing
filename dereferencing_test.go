package processing

import (
	"reflect"
	"testing"

	vocab "github.com/go-ap/activitypub"
)

func Test_aggregateIRIs(t *testing.T) {
	tests := []struct {
		name      string
		col       vocab.CollectionInterface
		wantIRIs  vocab.IRIs
		wantItems vocab.ItemCollection
		wantErr   bool
	}{
		{
			name:      "empty",
			col:       &vocab.IRIs{},
			wantItems: vocab.ItemCollection{},
			wantIRIs:  vocab.IRIs{},
			wantErr:   false,
		},
		{
			name:      "one iri",
			col:       &vocab.IRIs{"http://example.com"},
			wantItems: vocab.ItemCollection{vocab.IRI("http://example.com")},
			wantIRIs:  vocab.IRIs{"http://example.com"},
			wantErr:   false,
		},
		{
			name:      "one Item(IRI)",
			col:       &vocab.ItemCollection{vocab.IRI("http://example.com")},
			wantItems: vocab.ItemCollection{vocab.IRI("http://example.com")},
			wantIRIs:  vocab.IRIs{"http://example.com"},
			wantErr:   false,
		},
		{
			name:      "one Item",
			col:       &vocab.ItemCollection{vocab.Object{ID: "http://example.com"}},
			wantItems: vocab.ItemCollection{vocab.Object{ID: "http://example.com"}},
			wantIRIs:  vocab.IRIs{},
			wantErr:   false,
		},
		{
			name:      "one Item, one IRI",
			col:       &vocab.ItemCollection{vocab.Object{ID: "http://example.com"}, vocab.IRI("http://example.com/test")},
			wantItems: vocab.ItemCollection{vocab.Object{ID: "http://example.com"}, vocab.IRI("http://example.com/test")},
			wantIRIs:  vocab.IRIs{"http://example.com/test"},
			wantErr:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toDeref := make(vocab.IRIs, 0)
			toKeep := make(vocab.ItemCollection, 0)

			toCall := aggregateIRIs(&toDeref, &toKeep)
			if err := toCall(tt.col); (err != nil) != tt.wantErr {
				t.Errorf("aggregateIRIs() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !reflect.DeepEqual(toKeep, tt.wantItems) {
				t.Errorf("aggregateIRIs() = %#v, want %#v", toKeep, tt.wantItems)
			}
			if !reflect.DeepEqual(toDeref, tt.wantIRIs) {
				t.Errorf("aggregateIRIs() = %#v, want %#v", toDeref, tt.wantIRIs)
			}
		})
	}
}
