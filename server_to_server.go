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

	if _, err := p.s.Save(it); err != nil {
		return it, err
	}

	if colSaver, ok := p.s.(CollectionStore); ok {
		if _, err := AddToCollections(p, colSaver, it); err != nil {
			return it, err
		}
	}
	return it, nil
}
