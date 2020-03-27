package processing

import (
	"fmt"
	pub "github.com/go-ap/activitypub"
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
func ContentManagementActivity(l s.Saver, act *pub.Activity, col handlers.CollectionType) (*pub.Activity, error) {
	var err error
	switch act.Type {
	case pub.CreateType:
		act, err = CreateActivity(l, act)
	case pub.UpdateType:
		act, err = UpdateActivity(l, act)
	case pub.DeleteType:
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
func CreateActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	iri := act.Object.GetLink()
	if len(iri) == 0 {
		l.GenerateID(act.Object, act)
	}
	now := time.Now().UTC()
	err := updateCreateActivityObject(l, act.Object, act, now)
	if err != nil {
		return act, errors.Annotatef(err, "unable to create activity's object %s", act.Object.GetLink())
	}
	act.Object, err = addNewObjectCollections(act.Object)
	if err != nil {
		return act, errors.Annotatef(err, "unable to add object collections to object %s", act.Object.GetLink())
	}
	act.Object, err = l.SaveObject(act.Object)

	return act, nil
}

// UpdateActivity
// The Update activity is used when updating an already existing object. The side effect of this is that the object
// MUST be modified to reflect the new structure as defined in the update activity,
// assuming the actor has permission to update this object.
func UpdateActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	var err error

	ob := act.Object
	var cnt uint

	var found pub.ItemCollection
	typ := ob.GetType()
	if loader, ok := l.(s.ActorLoader); ok && pub.ActorTypes.Contains(typ) {
		found, cnt, _ = loader.LoadActors(ob)
	}
	if loader, ok := l.(s.ObjectLoader); ok && pub.ObjectTypes.Contains(typ) {
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

func updateCreateActivityObject(l s.Saver, o pub.Item, act *pub.Activity, now time.Time) error {
	return pub.OnObject(o, func(o *pub.Object) error {
		// See https://www.w3.org/TR/ActivityPub/#create-activity-outbox
		// Copying the actor's IRI to the object's AttributedTo
		o.AttributedTo = act.Actor.GetLink()

		// Merging the activity's and the object's Audience
		if aud, err := pub.ItemCollectionDeduplication(&act.Audience, &o.Audience); err == nil {
			o.Audience = FlattenItemCollection(aud)
			act.Audience = FlattenItemCollection(aud)
		}
		// Merging the activity's and the object's To addressing
		if to, err := pub.ItemCollectionDeduplication(&act.To, &o.To); err == nil {
			o.To = FlattenItemCollection(to)
			act.To = FlattenItemCollection(to)
		}
		// Merging the activity's and the object's Bto addressing
		if bto, err := pub.ItemCollectionDeduplication(&act.Bto, &o.Bto); err == nil {
			o.Bto = FlattenItemCollection(bto)
			act.Bto = FlattenItemCollection(bto)
		}
		// Merging the activity's and the object's Cc addressing
		if cc, err := pub.ItemCollectionDeduplication(&act.CC, &o.CC); err == nil {
			o.CC = FlattenItemCollection(cc)
			act.CC = FlattenItemCollection(cc)
		}
		// Merging the activity's and the object's Bcc addressing
		if bcc, err := pub.ItemCollectionDeduplication(&act.BCC, &o.BCC); err == nil {
			o.BCC = FlattenItemCollection(bcc)
			act.BCC = FlattenItemCollection(bcc)
		}

		if o.InReplyTo != nil {
			if colSaver, ok := l.(s.CollectionSaver); ok {
				if c, ok := o.InReplyTo.(pub.ItemCollection); ok {
					for _, repl := range c {
						iri := pub.IRI(fmt.Sprintf("%s/%s", repl.GetLink(), handlers.Replies))
						colSaver.AddToCollection(iri, o.GetLink())
					}
				} else {
					iri := pub.IRI(fmt.Sprintf("%s/%s", o.InReplyTo.GetLink(), handlers.Replies))
					colSaver.AddToCollection(iri, o.GetLink())
				}
			}
		}

		// TODO(marius): Move these to a ProcessObject function
		// Set the published date
		o.Published = now
		return nil
	})
}
