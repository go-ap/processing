package processing

import (
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// AddToRemoteCollections handles the dissemination of the received it Activity to the local collections,
// it is addressed to:
//   - the recipients' Inboxes
func (p P) AddToRemoteCollections(it vocab.Item, recipients ...vocab.Item) error {
	remoteRecipients := make(vocab.IRIs, 0)
	for _, rec := range recipients {
		recIRI := rec.GetLink()
		if p.IsLocal(recIRI) || remoteRecipients.Contains(recIRI) {
			continue
		}
		remoteRecipients.Append(recIRI)
	}
	return p.disseminateToRemoteCollection(it, remoteRecipients...)
}

func (p P) disseminateToRemoteCollection(act vocab.Item, iris ...vocab.IRI) error {
	if len(iris) == 0 {
		return nil
	}
	if vocab.IsNil(act) {
		return InvalidActivity("is nil")
	}
	p.l.Debugf("Starting remote actor's dissemination.")
	if !p.IsLocalIRI(act.GetLink()) {
		return errors.Newf("trying to disseminate local activity to local collection %s", act.GetLink())
	}
	keyLoader, ok := p.s.(KeyLoader)
	if !ok {
		return errors.Newf("local storage %T does not support loading private keys", p.s)
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
		if !IsInbox(col) {
			p.l.Errorf("trying to disseminate to remote collection that's not an Inbox: %s", col)
		}

		if p.c == nil {
			p.l.Errorf("Unable to push to remote collections, S2S client is nil for %s", act.GetLink())
			continue
		}
		// TODO(marius): Move this function to either the go-ap/auth package, or in FedBOX itself.
		//   We should probably change the signature for client.RequestSignFn to accept an Actor/IRI as a param.
		vocab.OnIntransitiveActivity(act, func(act *vocab.IntransitiveActivity) error {
			p.l.Debugf("Signing request for actor %s", act.Actor.GetLink())
			p.c.SignFn(s2sSignFn(keyLoader, act.Actor, signerWithDigest(p.l)))
			return nil
		})
		p.l.Infof("Pushing to remote actor's collection %s", col)
		if _, _, err := p.c.ToCollection(col, act); err != nil {
			if errors.IsConflict(err) {
				continue
			}
			g = append(g, err)
		}
	}
	if len(g) > 0 {
		return g
	}
	p.l.Debugf("Finished remote actor's dissemination successfully.")
	return nil
}

// AddToLocalCollections handles the dissemination of the received it Activity to the local collections,
// it is addressed to:
//   - the author's Outbox
//   - the recipients' Inboxes
func (p P) AddToLocalCollections(it vocab.Item, recipients ...vocab.Item) error {
	localRecipients := make(vocab.IRIs, 0)
	for _, rec := range recipients {
		recIRI := rec.GetLink()
		if !p.IsLocal(recIRI) || localRecipients.Contains(recIRI) {
			continue
		}
		localRecipients = append(localRecipients, recIRI)
	}
	return p.disseminateToLocalCollections(it, localRecipients...)
}

func (p P) disseminateToLocalCollections(ob vocab.Item, iris ...vocab.IRI) error {
	if len(iris) == 0 {
		return nil
	}
	g := make(groupError, 0)
	for _, col := range iris {
		if !p.IsLocalIRI(col) {
			g = append(g, errors.Newf("trying to save to remote collection %s", col))
			continue
		}
		if vocab.IsIRI(ob) {
			var err error
			infoFn("object requires de-referencing from remote IRI %s", ob.GetLink())
			ob, err = p.dereferenceIRIBasedOnInbox(ob, col)
			if err != nil {
				g = append(g, errors.Annotatef(err, "unable to load remote object: %s", col))
				continue
			}
		}
		infoFn("Saving to local actor's collection %s", col)
		if err := p.AddItemToCollection(col, ob); err != nil {
			g = append(g, err)
		}
	}
	if len(g) > 0 {
		return g
	}
	return nil
}

// AddItemToCollection attempts to append "it" to collection "col"
//
// If the collection is not local, it doesn't do anything
// If the item is a non-local IRI, it tries to dereference it, and then save a local representation of it.
func (p P) AddItemToCollection(col vocab.IRI, it vocab.Item) error {
	if !p.IsLocalIRI(col) {
		return nil
	}
	if !p.IsLocal(it) {
		if vocab.IsIRI(it) {
			deref, err := p.c.LoadIRI(it.GetLink())
			if err != nil {
				errFn("unable to load remote object [%s]: %s", it.GetLink(), err.Error())
			} else {
				it = deref
			}
			if _, err := p.s.Save(it); err != nil {
				errFn("unable to save remote object [%s] locally: %s", it.GetLink(), err.Error())
			}
		}
	}
	err := p.s.AddTo(col, it)
	if err != nil {
		errFn("unable to add object to collection {%s->%s}: %+s", it.GetLink(), col, err)
		if errors.IsConflict(err) {
			err = nil
		}
	}
	return err
}

func disseminateActivityObjectToLocalReplyToCollections(p P, act *vocab.Activity) error {
	return vocab.OnObject(act.Object, func(o *vocab.Object) error {
		replyToCollections, err := p.BuildReplyToCollections(o)
		if err != nil {
			errFn(errors.Annotatef(err, "unable to build replyTo collections").Error())
		}
		if err := p.AddToLocalCollections(o, replyToCollections...); err != nil {
			errFn(errors.Annotatef(err, "unable to add object to local replyTo collections").Error())
		}
		return nil
	})
}
