package processing

import (
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// NOTE(marius): this should be moved to the handlers package, where we are actually
//  interested in its functionality
type S2SProcessor interface {
	ProcessServerActivity(pub.Item) (pub.Item, error)
}

// ProcessServerActivity processes an Activity received in a server to server request
func (p defaultProcessor) ProcessServerActivity(it pub.Item) (pub.Item, error) {
	if it == nil {
		return nil, errors.Newf("Unable to process nil activity")
	}

	if pub.IntransitiveActivityTypes.Contains(it.GetType()) {
		return it, pub.OnIntransitiveActivity(it, func(act *pub.IntransitiveActivity) error {
			var err error
			it, err = processServerIntransitiveActivity(p, act)
			return err
		})
	}
	return it, pub.OnActivity(it, func(act *pub.Activity) error {
		var err error
		it, err = processServerActivity(p, act)
		return err
	})
}

func processServerActivity(p defaultProcessor, act *pub.Activity) (*pub.Activity, error) {
	return processClientActivity(p, act)
}

func processServerIntransitiveActivity(p defaultProcessor, it pub.Item) (pub.Item, error) {
	return processClientIntransitiveActivity(p, it)
}
