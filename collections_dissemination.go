package processing

import (
	"context"
	"time"

	"git.sr.ht/~mariusor/lw"
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
		_ = remoteRecipients.Append(recIRI)
	}
	if len(remoteRecipients) > 0 {
		p.l.Debugf("Starting dissemination to remote collections.")
		defer p.l.Debugf("Finished dissemination to remote collections.")
	}

	if !p.IsLocalIRI(it.GetLink()) {
		return errors.Newf("trying to disseminate remote activity %s to remote collections:", it.GetLink())
	}
	return p.disseminateToRemoteCollections(it, remoteRecipients...)
}

const (
	jitterDelay = 200 * time.Millisecond

	baseWaitTime = time.Second
	multiplier   = 1.4

	retries = 5
)

func retryFn(fn ssm.Fn) ssm.Fn {
	return ssm.Retry(retries, ssm.BackOff(baseWaitTime, ssm.Jitter(jitterDelay, ssm.Linear(multiplier)), fn))
}

func (p P) disseminateToRemoteCollections(it vocab.Item, iris ...vocab.IRI) error {
	if len(iris) == 0 {
		return nil
	}
	if vocab.IsNil(it) {
		return InvalidActivity("is nil")
	}

	if p.c == nil {
		return errors.NotImplementedf("unable to push to remote collection, S2S client is nil for %s", it.GetLink())
	}

	states := make([]ssm.Fn, 0, len(iris))
	for _, col := range iris {
		if p.IsLocalIRI(col) {
			p.l.Warnf("Invalid attempt to disseminate to local collection %s", col)
			continue
		}

		currentRetry := 0
		state := retryFn(func(ctx context.Context) ssm.Fn {
			// NOTE(marius): we expect that the client has already been set up for being able to POST requests
			// to remote servers. This means that it has been constructed using a HTTP client that includes
			// an HTTP-Signature RoundTripper.
			defer func() { currentRetry += 1 }()
			ll := p.l.WithContext(lw.Ctx{"to": col, "retry": currentRetry})
			if _, _, err := p.c.ToCollection(col, it); err != nil {
				ll.Warnf("Unable to disseminate activity %s", err)
				switch {
				case errors.IsConflict(err):
					// Resource already exists
					ll.Warnf("Conflict %s", col)
				case errors.IsNotFound(err):
					// Actor inbox was not found, either an authorization issue, or an invalid actor
					ll.Warnf("Not found %s", col)
				case errors.IsUnauthorized(err):
					// Authorization issue
					ll.Warnf("Unauthorized from remote server collection %s", col)
				case errors.IsForbidden(err):
					// Authorization issue
					ll.Warnf("Forbidden from remote server collection %s", col)
				case errors.IsMethodNotAllowed(err):
					// Server does not federate. See https://www.w3.org/TR/activitypub/#delivery
					ll.Warnf("TODO add mechanism for saving instances that need to be skipped due to unsupported S2S")
				default:
					return ssm.ErrorEnd(err)
				}
			} else {
				ll.Debugf("Pushed to remote actor's collection")
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
		if !p.IsLocal(recIRI) {
			continue
		}
		_ = localRecipients.Append(recIRI)
	}
	if len(localRecipients) > 0 {
		p.l.Debugf("Starting dissemination to local collections.")
		defer p.l.Debugf("Finished dissemination to local collections.")
	}
	return p.disseminateToLocalCollections(it, localRecipients...)
}

func (p P) disseminateToLocalCollections(it vocab.Item, iris ...vocab.IRI) error {
	if len(iris) == 0 {
		return nil
	}

	states := make([]ssm.Fn, 0, len(iris))
	// NOTE(marius): We rely on Go1.22 for range improvements where col is a copy, not a reference
	for _, col := range iris {
		ll := p.l.WithContext(lw.Ctx{"to": col})
		if !p.IsLocalIRI(col) {
			ll.Warnf("Trying to save to remote collection %s", col)
			continue
		}
		if vocab.IsIRI(it) {
			var err error
			ll.Tracef("Object requires de-referencing from remote IRI %s", it.GetLink())
			// NOTE(marius): check comment inside dereferenceIRIBasedOnInbox() method
			if it, err = p.dereferenceIRIBasedOnInbox(it, col); err != nil {
				ll.Warnf("Unable to load remote object %s: %s", it, err)
				continue
			}
		}
		state := func(ctx context.Context) ssm.Fn {
			ll.Debugf("Saving to local actor's collection")
			if err := p.AddItemToCollection(col, it); err != nil {
				ll.Warnf("Unable to disseminate activity %s", err)
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
			if _, err = p.s.Save(it); err != nil {
				p.l.Warnf("unable to save remote object [%s] locally: %s", it.GetLink(), err.Error())
			}
		}
	}
	err := p.s.AddTo(col, it)
	if err != nil {
		if errors.IsConflict(err) {
			return nil
		}
		p.l.WithContext(lw.Ctx{"err": err.Error(), "col": col.GetLink(), "it": it.GetLink()}).Warnf("unable to add object to collection")
	}
	return err
}

func disseminateActivityObjectToLocalReplyToCollections(p P, act *vocab.Activity) error {
	return vocab.OnObject(act.Object, func(o *vocab.Object) error {
		replyToCollections := p.BuildReplyToCollections(o)
		if err := p.AddToLocalCollections(o, replyToCollections...); err != nil {
			p.l.Warnf(errors.Annotatef(err, "unable to add object to local replyTo collections").Error())
		}
		return nil
	})
}
