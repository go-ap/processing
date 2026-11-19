package integration

import (
	"testing"

	"github.com/cucumber/godog"
	vocab "github.com/go-ap/activitypub"
)

type inboxCollectionManagementTests struct {
	Activity   vocab.Item
	Author     vocab.Actor
	ReceivedIn vocab.IRI
}

func (i *inboxCollectionManagementTests) initScenario(ctx *godog.ScenarioContext) {
	ctx.Step(`^nothing$`, nothing)
	ctx.Step(`^Skip$`, skip)

	ctx.Step(`^a Remove activity is received$`, i.aRemoveActivityIsReceived)
	ctx.Step(`^an Add activity is received$`, i.anAddActivityIsReceived)
	ctx.Step(`^the object will be added to the Target collection$`, i.theObjectWillBeAddedToTheTargetCollection)
	ctx.Step(`^the object will be removed from the Origin collection$`, i.theObjectWillBeRemovedFromTheOriginCollection)
}

func (i *inboxCollectionManagementTests) aRemoveActivityIsReceived() error {
	return godog.ErrPending
}

func (i *inboxCollectionManagementTests) anAddActivityIsReceived() error {
	return godog.ErrPending
}

func (i *inboxCollectionManagementTests) theObjectWillBeAddedToTheTargetCollection() error {
	return godog.ErrPending
}

func (i *inboxCollectionManagementTests) theObjectWillBeRemovedFromTheOriginCollection() error {
	return godog.ErrPending
}

func Test_Inbox_CollectionManagement(t *testing.T) {
	opts.TestingT = t
	opts.Paths = []string{"inbox-collection-management.feature"}

	i := new(inboxCollectionManagementTests)
	_ = godog.TestSuite{
		Name:                "ActivityPub Inbox Collection Management Activities",
		ScenarioInitializer: i.initScenario,
		Options:             &opts,
	}.Run()
}
