package integration

import (
	"testing"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
)

type inboxRelationshipTests struct {
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (i *inboxRelationshipTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)
}

func Test_Inbox_Relationship(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"inbox-relationship.feature"}

	i := new(inboxRelationshipTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Inbox Relationship",
		ScenarioInitializer: i.initScenario,
		Options:             &opts,
	}.Run()
}
