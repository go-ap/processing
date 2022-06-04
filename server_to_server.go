package processing

import (
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

type S2SProcessor interface {
	ProcessServerActivity(vocab.Item) (vocab.Item, error)
}

// ProcessServerActivity processes an Activity received in a server to server request
func (p defaultProcessor) ProcessServerActivity(it vocab.Item) (vocab.Item, error) {
	if it == nil {
		return nil, errors.Newf("Unable to process nil activity")
	}

	if vocab.IntransitiveActivityTypes.Contains(it.GetType()) {
		return it, vocab.OnIntransitiveActivity(it, func(act *vocab.IntransitiveActivity) error {
			var err error
			it, err = processServerIntransitiveActivity(p, act)
			return err
		})
	}
	return it, vocab.OnActivity(it, func(act *vocab.Activity) error {
		var err error
		it, err = processServerActivity(p, act)
		return err
	})
}

func processServerActivity(p defaultProcessor, act *vocab.Activity) (*vocab.Activity, error) {
	return processClientActivity(p, act)
}

func processServerIntransitiveActivity(p defaultProcessor, it vocab.Item) (vocab.Item, error) {
	return processClientIntransitiveActivity(p, it)
}
