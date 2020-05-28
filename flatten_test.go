package processing

import (
	pub "github.com/go-ap/activitypub"
	"reflect"
	"testing"
)

func TestFlattenPersonProperties(t *testing.T) {
	t.Skipf("TODO")
}

func TestFlattenProperties(t *testing.T) {
	t.Skipf("TODO")
}

func TestAddNewObjectCollections(t *testing.T) {
	t.Skipf("TODO")
}

func TestFlattenItemCollection(t *testing.T) {
	t.Skipf("TODO")
}

func TestFlattenCollection(t *testing.T) {
	t.Skipf("TODO")
}

func TestFlattenOrderedCollection(t *testing.T) {
	t.Skipf("TODO")
}

func TestFlattenIntransitiveActivityProperties(t *testing.T) {
	type args struct {
		act *pub.IntransitiveActivity
	}
	tests := []struct {
		name string
		args args
		want *pub.IntransitiveActivity
	}{
		{
			name: "blank",
			args: args{&pub.IntransitiveActivity{}},
			want: &pub.IntransitiveActivity{},
		},
		{
			name: "flatten-actor",
			args: args{&pub.IntransitiveActivity{Actor: &pub.Actor{ID: "example-actor-iri"}}},
			want: &pub.IntransitiveActivity{Actor: pub.IRI("example-actor-iri")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FlattenIntransitiveActivityProperties(tt.args.act); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FlattenIntransitiveActivityProperties() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFlattenActivityProperties(t *testing.T) {
	type args struct {
		act *pub.Activity
	}
	tests := []struct {
		name string
		args args
		want *pub.Activity
	}{
		{
			name: "blank",
			args: args{&pub.Activity{}},
			want: &pub.Activity{},
		},
		{
			name: "flatten-actor",
			args: args{&pub.Activity{Actor: &pub.Actor{ID: "example-actor-iri"}}},
			want: &pub.Activity{Actor: pub.IRI("example-actor-iri")},
		},
		{
			name: "flatten-object",
			args: args{&pub.Activity{Object: &pub.Object{ID: "example-actor-iri"}}},
			want: &pub.Activity{Object: pub.IRI("example-actor-iri")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FlattenActivityProperties(tt.args.act); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("FlattenActivityProperties() = %v, want %v", got, tt.want)
			}
		})
	}
}
