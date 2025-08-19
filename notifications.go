package processing

import (
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

	saveToCollections := func(objects ...vocab.Item) error {
		errs := make([]error, 0, len(objects))
		colToAdd := make(map[vocab.IRI][]vocab.IRI)
		for _, object := range objects {
			likes := vocab.Shares.IRI(object)
			colToAdd[likes] = append(colToAdd[likes], act.GetLink())
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

	// NOTE(marius): we add the activity to the object's shares collection
	return act, saveToCollections(collectionFromItem(act.Object)...)
}

func (p P) UndoAnnounceActivity(act *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(act.Object) {
		return act, InvalidActivityObject("is nil for %T[%s]", act, act.GetType())
	}

	maybeAnnounce, err := vocab.ToActivity(act.Object)
	if err != nil {
		return act, InvalidActivityObject("expecting %q activity, received %q", vocab.AnnounceType, act.Object.GetType())
	}
	if !p.IsLocal(maybeAnnounce.Object) {
		// NOTE(marius): we ignore not local objects
		return act, nil
	}
	// NOTE(marius): we remove the original Announce activity from its object's shares collection
	err = p.s.RemoveFrom(vocab.Shares.Of(maybeAnnounce.Object).GetLink(), maybeAnnounce)
	return act, err
}
