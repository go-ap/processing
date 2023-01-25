package processing

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	vocab "github.com/go-ap/activitypub"
)

func TestContentManagementActivity(t *testing.T) {
	t.Skipf("TODO")
}

func TestCreateActivity(t *testing.T) {
	t.Skipf("TODO")
}

func TestUpdateActivity(t *testing.T) {
	t.Skipf("TODO")
}

func Test_addNewActorCollections(t *testing.T) {
	type args struct {
		p *vocab.Actor
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	t.Skipf("TODO")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := addNewActorCollections(tt.args.p); (err != nil) != tt.wantErr {
				t.Errorf("addNewActorCollections() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_addNewItemCollections(t *testing.T) {
	type args struct {
		it vocab.Item
	}
	tests := []struct {
		name    string
		args    args
		want    vocab.Item
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	t.Skipf("TODO")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := addNewItemCollections(tt.args.it)
			if (err != nil) != tt.wantErr {
				t.Errorf("addNewItemCollections() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("addNewItemCollections() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_addNewObjectCollections(t *testing.T) {
	type args struct {
		o *vocab.Object
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	t.Skipf("TODO")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := addNewObjectCollections(tt.args.o); (err != nil) != tt.wantErr {
				t.Errorf("addNewObjectCollections() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getCollection(t *testing.T) {
	type args struct {
		it vocab.Item
		c  vocab.CollectionPath
	}
	tests := []struct {
		name string
		args args
		want vocab.CollectionInterface
	}{
		// TODO: Add test cases.
	}
	t.Skipf("TODO")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getCollection(tt.args.it, tt.args.c); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getCollection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updateCreateActivityObject(t *testing.T) {
	type args struct {
		l   WriteStore
		o   vocab.Item
		act *vocab.Activity
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	t.Skipf("TODO")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := updateCreateActivityObject(tt.args.l, tt.args.o, tt.args.act); (err != nil) != tt.wantErr {
				t.Errorf("updateCreateActivityObject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_updateObjectForCreate(t *testing.T) {
	type args struct {
		l   WriteStore
		o   *vocab.Object
		act *vocab.Activity
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	t.Skipf("TODO")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := updateObjectForCreate(tt.args.l, tt.args.o, tt.args.act); (err != nil) != tt.wantErr {
				t.Errorf("updateObjectForCreate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_updateObjectForUpdate(t *testing.T) {
	type args struct {
		l   WriteStore
		o   *vocab.Object
		act *vocab.Activity
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	t.Skipf("TODO")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := updateObjectForUpdate(tt.args.l, tt.args.o); (err != nil) != tt.wantErr {
				t.Errorf("updateObjectForUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_updateUpdateActivityObject(t *testing.T) {
	type args struct {
		l   WriteStore
		o   vocab.Item
		act *vocab.Activity
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	t.Skipf("TODO")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := updateUpdateActivityObject(tt.args.l, tt.args.o); (err != nil) != tt.wantErr {
				t.Errorf("updateUpdateActivityObject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

var defaultActor = &vocab.Actor{
	ID: vocab.IRI("https://example.com/user/jdoe"),
}

func Test_defaultIDGenerator(t *testing.T) {
	var publishedAt = time.Now()

	type args struct {
		it     vocab.Item
		partOf vocab.Item
		by     vocab.Item
	}
	tests := []struct {
		name string
		args args
		want vocab.ID
	}{
		{
			name: "plain inbox",
			args: args{
				&vocab.Object{Published: publishedAt},
				vocab.Inbox.IRI(defaultActor),
				nil,
			},
			want: vocab.Inbox.IRI(defaultActor).AddPath(fmt.Sprintf("%d", publishedAt.UnixMilli())),
		},
		{
			name: "empty collection",
			args: args{
				&vocab.Object{
					AttributedTo: defaultActor,
					Published:    publishedAt,
				},
				nil,
				nil,
			},
			want: vocab.Outbox.IRI(vocab.IRI("https://example.com")).AddPath(fmt.Sprintf("%d", publishedAt.UnixMilli())),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := defaultIDGenerator("https://example.com")(tt.args.it, tt.args.partOf, tt.args.by); got != tt.want {
				t.Errorf("defaultIDGenerator() = %v, want %v", got, tt.want)
			}
		})
	}
}
