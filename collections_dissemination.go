package processing

import (
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// AddToRemoteCollections handles the dissemination of the received it Activity to the local collections,
// it is addressed to:
//  - the recipients' Inboxes
func (p P) AddToRemoteCollections(it vocab.Item, recipients vocab.ItemCollection) error {
	remoteRecipients := make(vocab.IRIs, 0)
	for _, recInb := range recipients {
		if !p.IsLocal(recInb) {
			remoteRecipients = append(remoteRecipients, recInb.GetLink())
		}
	}
	return disseminateToRemoteCollection(p, it, remoteRecipients...)
}

// AddToLocalCollections handles the dissemination of the received it Activity to the local collections,
// it is addressed to:
//  - the author's Outbox
//  - the recipients' Inboxes
func (p P) AddToLocalCollections(it vocab.Item, recipients vocab.ItemCollection) error {
	localRecipients := make(vocab.IRIs, 0)
	for _, recInb := range recipients {
		if p.IsLocal(recInb) {
			localRecipients = append(localRecipients, recInb.GetLink())
		}
	}
	return disseminateToLocalCollections(p, it, localRecipients...)
}

func disseminateToRemoteCollection(p P, act vocab.Item, iris ...vocab.IRI) error {
	if len(iris) == 0 {
		return nil
	}
	keyLoader, ok := p.s.(KeyLoader)
	if !ok {
		return errors.Newf("local storage %T does not support loading private keys", p.s)
	}
	if !p.IsLocalIRI(act.GetLink()) {
		return errors.Newf("trying to disseminate local activity to local collection %s", act.GetLink())
	}
	// TODO(marius): the processing module needs a method to see if an IRI is local or not
	//    For each recipient we need to save the incoming activity to the actor's Inbox if the actor is local
	//    Or disseminate it using S2S if the actor is not local
	g := make(groupError, 0)
	for _, col := range iris {
		if p.IsLocalIRI(col) {
			g = append(g, errors.Newf("trying to disseminate to local collection %s", col))
			continue
		}

		// TODO(marius): Move this function to either the go-ap/auth package, or in FedBOX itself.
		//   We should probably change the signature for client.RequestSignFn to accept an Actor/IRI as a param.
		vocab.OnIntransitiveActivity(act, func(act *vocab.IntransitiveActivity) error {
			p.c.SignFn(s2sSignFn(keyLoader, act.Actor))
			return nil
		})
		infoFn("Pushing to remote actor's collection %s", col)
		if _, _, err := p.c.ToCollection(col, act); err != nil {
			g = append(g, err)
		}
	}
	if len(g) > 0 {
		return g
	}
	return nil
}

func disseminateToLocalCollections(p P, act vocab.Item, iris ...vocab.IRI) error {
	if len(iris) == 0 {
		return nil
	}
	colSaver, ok := p.s.(CollectionStore)
	if !ok {
		return errors.Newf("local storage %T does not support appending to collections", p.s)
	}
	g := make(groupError, 0)
	for _, col := range iris {
		if !p.IsLocalIRI(col) {
			g = append(g, errors.Newf("trying to save to remote collection %s", col))
			continue
		}
		infoFn("Saving to local actor's collection %s", col)
		if err := colSaver.AddTo(col, act.GetLink()); err != nil {
			g = append(g, err)
		}
	}
	if len(g) > 0 {
		return g
	}
	return nil
}
