package processing

import (
	"git.sr.ht/~mariusor/lw"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// ValidateClientNotificationActivity
func (p P) ValidateClientNotificationActivity(act *vocab.Activity) error {
	if vocab.IsNil(act.Object) {
		return InvalidActivityObject("is nil")
	}

	if ob, err := p.DereferenceItem(act.Object); err != nil {
		return err
	} else {
		act.Object = ob
	}
	return nil
}

func collectionFromItem(it vocab.Item) vocab.ItemCollection {
	if vocab.IsNil(it) {
		return nil
	}

	var result vocab.ItemCollection
	if !vocab.IsItemCollection(it) {
		result = vocab.ItemCollection{it}
	}
	_ = vocab.OnItemCollection(it, func(col *vocab.ItemCollection) error {
		result = *col
		return nil
	})
	return result
}

// NotificationActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-notification
//
// The Notification use case primarily deals with calling attention to particular objects or notifications.
//
// Upon receipt of an Announce activity in an inbox, a server SHOULD increment the object's count of shares
// by adding the received activity to the shares collection if this collection is present.
// Note: The Announce activity is effectively what is known as "sharing", "reposting", or "boosting" in other social
// networks.
//
// https://www.w3.org/TR/activitypub/#announce-activity-inbox
func (p P) NotificationActivity(act *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(act.Object) {
		return act, InvalidActivityObject("is nil for %T[%s]", act, act.GetType())
	}

	// NOTE(marius): this covers only "Announce" activities, as it's currently
	// the only activity type matching the Notification group.
	if !p.IsLocal(act.Object) {
		// NOTE(marius): we ignore not local objects
		return act, nil
	}
	good := vocab.ActivityVocabularyTypes{vocab.AnnounceType}
	if !good.Match(act.Type) {
		return act, errors.BadRequestf("Activity has wrong type %s, expected %v", act.Type, good)
	}

	// NOTE(marius): because we add the Announce activity to the shares collection, we need to save it locally first.
	it, err := p.s.Save(act)
	likeWasSavedLocally := err == nil

	objects := make(vocab.ItemCollection, 0)
	_ = vocab.OnItem(act.Object, func(item vocab.Item) error {
		objects = append(objects, item)
		return nil
	})

	saveToCollections := func(objects vocab.ItemCollection) error {
		errs := make([]error, 0)
		colToAdd := make(map[vocab.IRI][]vocab.IRI)

		for _, object := range objects {
			if !likeWasSavedLocally {
				p.l.WithContext(lw.Ctx{"iri": act.ID, "typ": act.Type}).Warnf("Activity was not saved locally, unable to add it to collections.")
				break
			}
			likes := vocab.Shares.IRI(object)
			colToAdd[likes] = append(colToAdd[likes], it.GetLink())
		}
		for col, iris := range colToAdd {
			for _, iri := range iris {
				if err := p.AddItemToCollection(col, iri); err != nil {
					errs = append(errs, errors.Annotatef(err, "Unable to save %s to collection %s", iris, col))
				}
			}
		}
		return errors.Join(errs...)
	}

	// NOTE(marius): we're only saving to the Liked and Likes collections for Likes in order to conform to the spec.
	_ = saveToCollections(objects)
	return act, nil
}

func (p *P) UndoAnnounceActivity(announce *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(announce.Object) {
		return announce, InvalidActivityObject("is nil for %T[%s]", announce, announce.GetType())
	}

	maybeAnnounce, err := vocab.ToActivity(announce.Object)
	if err != nil {
		return announce, InvalidActivityObject("expecting %q activity, received %q", vocab.AnnounceType, announce.Object.GetType())
	}
	if !p.IsLocal(maybeAnnounce.Object) {
		// NOTE(marius): we ignore not local objects
		return announce, nil
	}
	// NOTE(marius): we remove the original Announce activity from its object's shares collection
	err = p.s.RemoveFrom(vocab.Shares.Of(maybeAnnounce.Object).GetLink(), maybeAnnounce)
	return announce, err
}
