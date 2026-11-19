package integration

import (
	"testing"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
)

type inboxGeneralTests struct {
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (i *inboxGeneralTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)

	ctx.Step(`^an Activity is received$`, i.anActivityIsReceived)
	ctx.Step(`^an Activity with to, bto, cc and bcc recipients is received$`, i.anActivityWithToBtoCcAndBccRecipientsIsReceived)
	ctx.Step(`^the activity is disseminated to local recipients$`, i.theActivityIsDisseminatedToLocalRecipients)
	ctx.Step(`^the activity is forwarded to remote actors if part of local collections$`, i.theActivityIsForwardedToRemoteActorsIfPartOfLocalCollections)
}

func (i *inboxGeneralTests) anActivityIsReceived() error {
	return godog.ErrPending
}

func (i *inboxGeneralTests) anActivityWithToBtoCcAndBccRecipientsIsReceived() error {
	return godog.ErrPending
}

func (i *inboxGeneralTests) theActivityIsDisseminatedToLocalRecipients() error {
	return godog.ErrPending
}

func (i *inboxGeneralTests) theActivityIsForwardedToRemoteActorsIfPartOfLocalCollections() error {
	return godog.ErrPending
}

func Test_Inbox_General(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"inbox-general.feature"}

	i := new(inboxGeneralTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Inbox General",
		ScenarioInitializer: i.initScenario,
		Options:             &opts,
	}.Run()
}
