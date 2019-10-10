package processing

import (
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
	now := time.Now().UTC()
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

	// Set the published date
	act.Published = now
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
			return updateActivityObject(l, &a.Parent, act, now)
		})
	} else if as.ActorTypes.Contains(obType) {
		activitypub.OnPerson(act.Object, func(p *activitypub.Person) error {
			return updateActivityObject(l, &p.Parent.Parent, act, now)
		})
	} else {
		activitypub.OnObject(act.Object, func(o *activitypub.Object) error {
			return updateActivityObject(l, &o.Parent, act, now)
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
		ob, err = UpdateItemProperties(it, ob)
		if err != nil {
			return act, err
		}
	}

	act.Object, err = l.UpdateObject(ob)
	return act, err
}
