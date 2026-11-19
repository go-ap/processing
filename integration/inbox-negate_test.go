package integration

import (
	"testing"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
)

type inboxNegateTests struct {
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (i *inboxNegateTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)

	ctx.Step(`^an Undo activity is received$`, i.anUndoActivityIsReceived)
	ctx.Step(`^processing is successful$`, i.processingIsSuccessful)
}

func (i *inboxNegateTests) anUndoActivityIsReceived() error {
	return godog.ErrPending
}

func (i *inboxNegateTests) processingIsSuccessful() error {
	return godog.ErrPending
}

func Test_Inbox_Negate(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"inbox-negate.feature"}

	i := new(inboxNegateTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Inbox Negate",
		ScenarioInitializer: i.initScenario,
		Options:             &opts,
	}.Run()
}
