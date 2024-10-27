package processing

import (
	"context"
	"time"

	"git.sr.ht/~mariusor/ssm"
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
	p.l.Debugf("Starting dissemination to remote collections.")
	defer p.l.Debugf("Finished dissemination to remote collections.")
	return p.disseminateToRemoteCollection(it, remoteRecipients...)
}

const (
	jitterDelay = 200 * time.Millisecond

	baseWaitTime = time.Second
	multiplier   = 1.4

	retries = 5
)

func retryFn(fn ssm.Fn) ssm.Fn {
	return ssm.Retry(retries, ssm.BackOff(ssm.Jitter(jitterDelay, ssm.Linear(baseWaitTime, multiplier)), fn))
}

func (p P) disseminateToRemoteCollection(it vocab.Item, iris ...vocab.IRI) error {
	if len(iris) == 0 {
		return nil
	}
	if vocab.IsNil(it) {
		return InvalidActivity("is nil")
	}
	if !p.IsLocalIRI(it.GetLink()) {
		return errors.Newf("trying to disseminate remote activity %s to remote collections", it.GetLink())
	}

	keyLoader, ok := p.s.(KeyLoader)
	if !ok {
		return errors.Newf("local storage %T does not support loading private keys", p.s)
	}

	// TODO(marius): the processing module needs a method to see if an IRI is local or not
	//    For each recipient we need to save the incoming activity to the actor's Inbox if the actor is local
	//    Or disseminate it using S2S if the actor is not local
	states := make([]ssm.Fn, 0, len(iris))
	for _, col := range iris {
		state := retryFn(func(ctx context.Context) ssm.Fn {
			if p.IsLocalIRI(col) {
				p.l.Warnf("Trying to disseminate to local collection %s", col)
				return ssm.End
			}
			if !IsInbox(col) {
				p.l.Warnf("Trying to disseminate to remote collection that's not an Inbox: %s", col)
				return ssm.End
			}

			if p.c == nil {
				p.l.Warnf("Unable to push to remote collection, S2S client is nil for %s", it.GetLink())
				return ssm.End
			}
			// TODO(marius): Move this function to either the go-ap/auth package, or in FedBOX itself.
			//   We should probably change the signature for client.RequestSignFn to accept an Actor/IRI as a param.
			err := vocab.OnIntransitiveActivity(it, func(act *vocab.IntransitiveActivity) error {
				p.l.Tracef("Signing request for actor %s", act.Actor.GetLink())
				p.c.SignFn(s2sSignFn(keyLoader, act.Actor, signerWithDigest(p.l)))
				return nil
			})
			if err != nil {
				p.l.Warnf("Unable to sign request %s", err)
			}
			p.l.Infof("Pushing to remote actor's collection %s", col)

			_, _, err = p.c.ToCollection(col, it)
			if err != nil {
				p.l.Warnf("Unable to disseminate activity %s", err)
				switch {
				case errors.IsConflict(err):
					// Resource already exists
				case errors.IsNotFound(err):
					// Actor inbox was not found, either an authorization issue, or an invalid actor
				case errors.IsUnauthorized(err):
					// Authorization issue
				case errors.IsMethodNotAllowed(err):
					// Server does not federate. See https://www.w3.org/TR/activitypub/#delivery
					p.l.Warnf("TODO add mechanism for saving instances that need to be skipped due to unsupported S2S")
				default:
					return ssm.ErrorEnd(err)
				}
			}
			return ssm.End
		})
		states = append(states, state)
	}
	return ssm.RunParallel(context.Background(), states...)
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
	p.l.Debugf("Starting dissemination to local collections.")
	defer p.l.Debugf("Finished dissemination to local collections.")
	return p.disseminateToLocalCollections(it, localRecipients...)
}

func (p P) disseminateToLocalCollections(it vocab.Item, iris ...vocab.IRI) error {
	if len(iris) == 0 {
		return nil
	}

	states := make([]ssm.Fn, 0, len(iris))
	for _, col := range iris {
		if !p.IsLocalIRI(col) {
			p.l.Warnf("Trying to save to remote collection %s", col)
			continue
		}
		if vocab.IsIRI(it) {
			var err error
			p.l.Tracef("Object requires de-referencing from remote IRI %s", it.GetLink())
			// NOTE(marius): check comment inside dereferenceIRIBasedOnInbox() method
			if it, err = p.dereferenceIRIBasedOnInbox(it, col); err != nil {
				p.l.Warnf("Unable to load remote object %s: %s", it, err)
				continue
			}
		}
		state := func(ctx context.Context) ssm.Fn {
			ss := col.GetLink().String()
			p.l.Infof("Saving to local actor's collection %s", ss)
			if err := p.AddItemToCollection(col, it); err != nil {
				p.l.Warnf("Unable to disseminate activity %s", err)
			}
			return ssm.End
		}
		states = append(states, state)
	}

	return ssm.Run(context.Background(), states...)
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
				p.l.Warnf("unable to load remote object [%s]: %s", it.GetLink(), err.Error())
			} else {
				it = deref
			}
			if _, err := p.s.Save(it); err != nil {
				p.l.Warnf("unable to save remote object [%s] locally: %s", it.GetLink(), err.Error())
			}
		}
	}
	err := p.s.AddTo(col, it)
	if err != nil {
		p.l.Warnf("unable to add object to collection {%s->%s}: %+s", it.GetLink(), col, err)
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
			p.l.Warnf(errors.Annotatef(err, "unable to build replyTo collections").Error())
		}
		if err := p.AddToLocalCollections(o, replyToCollections...); err != nil {
			p.l.Warnf(errors.Annotatef(err, "unable to add object to local replyTo collections").Error())
		}
		return nil
	})
}
