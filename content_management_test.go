package processing

import (
	"fmt"
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/handlers"
	"testing"
	"time"
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
	t.Skipf("TODO")
}

func Test_addNewItemCollections(t *testing.T) {
	t.Skipf("TODO")
}

func Test_addNewObjectCollections(t *testing.T) {
	t.Skipf("TODO")
}

func Test_getCollection(t *testing.T) {
	t.Skipf("TODO")
}

func Test_updateCreateActivityObject(t *testing.T) {
	t.Skipf("TODO")
}

func Test_updateObjectForCreate(t *testing.T) {
	t.Skipf("TODO")
}

func Test_updateObjectForUpdate(t *testing.T) {
	t.Skipf("TODO")
}

func Test_updateUpdateActivityObject(t *testing.T) {
	t.Skipf("TODO")
}

var defaultActor = &pub.Actor{
	ID: pub.IRI("https://example.com/user/jdoe"),
}
var publishedAt = time.Now()

func Test_defaultIDGenerator(t *testing.T) {
	type args struct {
		it     pub.Item
		partOf pub.Item
		by     pub.Item
	}
	tests := []struct {
		name string
		args args
		want pub.ID
	}{
		{
			name: "plain inbox",
			args: args{
				&pub.Object{
					Published: publishedAt,
				},
				handlers.Inbox.IRI(defaultActor),
				nil,
			},
			want: handlers.Inbox.IRI(defaultActor).AddPath(fmt.Sprintf("%d", publishedAt.UnixNano() / 1000)),
		},
		{
			name: "empty collection",
			args: args{
				&pub.Object{
					AttributedTo: defaultActor,
					Published: publishedAt,
				},
				nil,
				nil,
			},
			want: handlers.Outbox.IRI(defaultActor).AddPath(fmt.Sprintf("%d", publishedAt.UnixNano() / 1000)),
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