package integration

import (
	"testing"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
)

type inboxReactionsTests struct {
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (i *inboxReactionsTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)
}

func Test_Inbox_Reactions(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"inbox-reactions.feature"}

	i := new(inboxReactionsTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Inbox Reactions",
		ScenarioInitializer: i.initScenario,
		Options:             &opts,
	}.Run()
}
