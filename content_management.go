package processing

import (
	"fmt"
	"github.com/go-ap/activitypub"
	as "github.com/go-ap/activitystreams"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
	"time"
)

// ContentManagementActivity processes matching activities
// The Content Management use case primarily deals with activities that involve the creation,
// modification or deletion of content.
// This includes, for instance, activities such as "John created a new note",
// "Sally updated an article", and "Joe deleted the photo".
func ContentManagementActivity(l s.Saver, act *as.Activity, col handlers.CollectionType) (*as.Activity, error) {
	var err error
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for Activity")
	}
	switch act.Type {
	case as.CreateType:
		act, err = CreateActivity(l, act)
	case as.UpdateType:
		act, err = UpdateActivity(l, act)
	case as.DeleteType:
		// TODO(marius): Move this piece of logic to the validation mechanism
		if len(act.Object.GetLink()) == 0 {
			return act, errors.Newf("unable to update object without a valid object id")
		}
		act.Object, err = l.DeleteObject(act.Object)
	}
	if err != nil && !isDuplicateKey(err) {
		//l.errFn(logrus.Fields{"IRI": act.GetLink(), "type": act.Type}, "unable to save activity's object")
		return act, err
	}

	return act, err
}

// CreateActivity
// The Create activity is used when posting a new object. This has the side effect that the object embedded within the
// Activity (in the object property) is created.
// When a Create activity is posted, the actor of the activity SHOULD be copied onto the object's attributedTo field.
// A mismatch between addressing of the Create activity and its object is likely to lead to confusion.
// As such, a server SHOULD copy any recipients of the Create activity to its object upon initial distribution,
// and likewise with copying recipients from the object to the wrapping Create activity.
// Note that it is acceptable for the object's addressing to be changed later without changing the Create's addressing
// (for example via an Update activity).
func CreateActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	iri := act.Object.GetLink()
	if len(iri) == 0 {
		l.GenerateID(act.Object, act)
	}
	now := time.Now().UTC()
	obType := act.Object.GetType()
	// TODO(marius) Add function as.AttributedTo(it as.Item, auth as.Item)
	if as.ActivityTypes.Contains(obType) {
		activitypub.OnActivity(act.Object, func(a *as.Activity) error {
			return updateCreateActivityObject(l, &a.Parent, act, now)
		})
	} else if as.ActorTypes.Contains(obType) {
		activitypub.OnPerson(act.Object, func(p *activitypub.Person) error {
			return updateCreateActivityObject(l, &p.Parent.Parent, act, now)
		})
	} else {
		activitypub.OnObject(act.Object, func(o *activitypub.Object) error {
			return updateCreateActivityObject(l, &o.Parent, act, now)
		})
	}

	var err error
	if colSaver, ok := l.(s.CollectionSaver); ok {
		act.Object, err = AddNewObjectCollections(colSaver, act.Object)
		if err != nil {
			return act, errors.Annotatef(err, "unable to add object collections to object %s", act.Object.GetLink())
		}
	}

	act.Object, err = l.SaveObject(act.Object)

	return act, nil
}

// UpdateActivity
// The Update activity is used when updating an already existing object. The side effect of this is that the object
// MUST be modified to reflect the new structure as defined in the update activity,
// assuming the actor has permission to update this object.
func UpdateActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius): Move this piece of logic to the validation mechanism
	if len(act.Object.GetLink()) == 0 {
		return act, errors.Newf("unable to update object without a valid object id")
	}
	var err error

	ob := act.Object
	var cnt uint
	if as.ActivityTypes.Contains(ob.GetType()) {
		return act, errors.Newf("unable to update activity")
	}

	var found as.ItemCollection
	typ := ob.GetType()
	if loader, ok := l.(s.ActorLoader); ok && as.ActorTypes.Contains(typ) {
		found, cnt, _ = loader.LoadActors(ob)
	}
	if loader, ok := l.(s.ObjectLoader); ok && as.ObjectTypes.Contains(typ) {
		found, cnt, _ = loader.LoadObjects(ob)
	}
	if len(ob.GetLink()) == 0 {
		return act, err
	}
	if cnt == 0 || found == nil {
		return act, errors.NotFoundf("Unable to find %s %s", ob.GetType(), ob.GetLink())
	}
	if it := found.First(); it != nil {
		ob, err = CopyItemProperties(it, ob)
		if err != nil {
			return act, err
		}
	}

	act.Object, err = l.UpdateObject(ob)
	return act, err
}

func updateCreateActivityObject(l s.Saver, o *as.Object, act *as.Activity, now time.Time) error {
	// See https://www.w3.org/TR/ActivityPub/#create-activity-outbox
	// Copying the actor's IRI to the object's AttributedTo
	o.AttributedTo = act.Actor.GetLink()

	// Merging the activity's and the object's Audience
	if aud, err := as.ItemCollectionDeduplication(&act.Audience, &o.Audience); err == nil {
		o.Audience = FlattenItemCollection(aud)
		act.Audience = FlattenItemCollection(aud)
	}
	// Merging the activity's and the object's To addressing
	if to, err := as.ItemCollectionDeduplication(&act.To, &o.To); err == nil {
		o.To = FlattenItemCollection(to)
		act.To = FlattenItemCollection(to)
	}
	// Merging the activity's and the object's Bto addressing
	if bto, err := as.ItemCollectionDeduplication(&act.Bto, &o.Bto); err == nil {
		o.Bto = FlattenItemCollection(bto)
		act.Bto = FlattenItemCollection(bto)
	}
	// Merging the activity's and the object's Cc addressing
	if cc, err := as.ItemCollectionDeduplication(&act.CC, &o.CC); err == nil {
		o.CC = FlattenItemCollection(cc)
		act.CC = FlattenItemCollection(cc)
	}
	// Merging the activity's and the object's Bcc addressing
	if bcc, err := as.ItemCollectionDeduplication(&act.BCC, &o.BCC); err == nil {
		o.BCC = FlattenItemCollection(bcc)
		act.BCC = FlattenItemCollection(bcc)
	}

	if o.InReplyTo != nil {
		if colSaver, ok := l.(s.CollectionSaver); ok {
			if c, ok := o.InReplyTo.(as.ItemCollection); ok {
				for _, repl := range c {
					iri := as.IRI(fmt.Sprintf("%s/%s", repl.GetLink(), handlers.Replies))
					colSaver.AddToCollection(iri, o.GetLink())
				}
			} else {
				iri := as.IRI(fmt.Sprintf("%s/%s",  o.InReplyTo.GetLink(), handlers.Replies))
				colSaver.AddToCollection(iri, o.GetLink())
			}
		}
	}

	// TODO(marius): Move these to a ProcessObject function
	// Set the published date
	o.Published = now

	return nil
}
