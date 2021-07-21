package processing

import (
	"fmt"
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
	"time"
)

// IDGenerator takes an ActivityStreams object, a collection to store it in, and the activity that has it as object:
//  "it" is the object we want to generate the ID for.
//  "partOf" represents the Collection that it is a part of.
//  "by" represents the Activity that generated the object
type IDGenerator func(it pub.Item, partOf pub.Item, by pub.Item) (pub.ID, error)

var createID IDGenerator

func defaultIDGenerator(base pub.IRI) IDGenerator {
	timeIDFn := func(t time.Time) string { return fmt.Sprintf("%d", t.UnixNano()/1000) }

	return func(it pub.Item, col pub.Item, _ pub.Item) (pub.ID, error) {
		var colIRI pub.IRI

		if col != nil && len(col.GetLink()) > 0 {
			colIRI = col.GetLink()
		}
		when := time.Now()
		pub.OnObject(it, func(o *pub.Object) error {
			if !o.Published.IsZero() {
				when = o.Published
			}
			if o.AttributedTo != nil {
				base = o.AttributedTo.GetLink()
			}
			if len(colIRI) == 0 {
				colIRI = handlers.Outbox.IRI(base)
			}
			return nil
		})
		if len(colIRI) == 0 {
			return pub.NilID, errors.Newf("invalid collection to generate the ID")
		}
		return colIRI.AddPath(timeIDFn(when)), nil
	}
}

func SetID(it pub.Item, partOf pub.Item, act pub.Item) error {
	if createID != nil {
		return pub.OnObject(it, func(o *pub.Object) error {
			var err error
			o.ID, err = createID(it, partOf, act)
			return err
		})
	}
	return errors.Newf("no package ID generator was set")
}

// ContentManagementActivity processes matching activities.
// The Content Management use case primarily deals with activities that involve the creation,
// modification or deletion of content.
// This includes, for instance, activities such as "John created a new note",
// "Sally updated an article", and "Joe deleted the photo".
func ContentManagementActivity(l s.WriteStore, act *pub.Activity, col handlers.CollectionType) (*pub.Activity, error) {
	var err error
	switch act.Type {
	case pub.CreateType:
		act, err = CreateActivity(l, act)
	case pub.UpdateType:
		act, err = UpdateActivity(l, act)
	case pub.DeleteType:
		act.Object, err = l.Delete(act.Object)
	}
	if err != nil && !isDuplicateKey(err) {
		//l.errFn(logrus.Fields{"IRI": act.GetLink(), "type": act.Type}, "unable to save activity's object")
		return act, err
	}

	return act, err
}

func getCollection(it pub.Item, c handlers.CollectionType) pub.CollectionInterface {
	return &pub.OrderedCollection{
		ID:   c.IRI(it).GetLink(),
		Type: pub.OrderedCollectionType,
	}
}

func addNewActorCollections(p *pub.Actor) error {
	if p.Inbox == nil {
		p.Inbox = getCollection(p, handlers.Inbox)
	}
	if p.Outbox == nil {
		p.Outbox = getCollection(p, handlers.Outbox)
	}
	if p.Followers == nil {
		p.Followers = getCollection(p, handlers.Followers)
	}
	if p.Following == nil {
		p.Following = getCollection(p, handlers.Following)
	}
	if p.Liked == nil {
		p.Liked = getCollection(p, handlers.Liked)
	}
	if p.Type == pub.PersonType {
		if p.Endpoints == nil {
			p.Endpoints = &pub.Endpoints{}
		}
		if p.Endpoints.OauthAuthorizationEndpoint == nil {
			p.Endpoints.OauthAuthorizationEndpoint = p.GetLink().AddPath("oauth", "authorize")
		}
		if p.Endpoints.OauthTokenEndpoint == nil {
			p.Endpoints.OauthTokenEndpoint = p.GetLink().AddPath("oauth", "token")
		}
	}
	return nil
}

func addNewObjectCollections(o *pub.Object) error {
	if o.Replies == nil {
		o.Replies = getCollection(o, handlers.Replies)
	}
	if o.Likes == nil {
		o.Likes = getCollection(o, handlers.Likes)
	}
	if o.Shares == nil {
		o.Shares = getCollection(o, handlers.Shares)
	}
	return nil
}

func addNewItemCollections(it pub.Item) (pub.Item, error) {
	if pub.ActorTypes.Contains(it.GetType()) {
		pub.OnActor(it, addNewActorCollections)
	}
	pub.OnObject(it, addNewObjectCollections)
	return it, nil
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
func CreateActivity(l s.WriteStore, act *pub.Activity) (*pub.Activity, error) {
	if iri := act.Object.GetLink(); len(iri) == 0 {
		if err := SetID(act.Object, handlers.Outbox.IRI(act.Actor), act); err != nil {
			return act, nil
		}
	}
	err := updateCreateActivityObject(l, act.Object, act)
	if err != nil {
		return act, errors.Annotatef(err, "unable to create activity's object %s", act.Object.GetLink())
	}
	act.Object, err = addNewItemCollections(act.Object)
	if err != nil {
		return act, errors.Annotatef(err, "unable to add object collections to object %s", act.Object.GetLink())
	}
	act.Object, err = l.Save(act.Object)

	return act, err
}

// UpdateActivity
// The Update activity is used when updating an already existing object. The side effect of this is that the object
// MUST be modified to reflect the new structure as defined in the update activity,
// assuming the actor has permission to update this object.
func UpdateActivity(l s.WriteStore, act *pub.Activity) (*pub.Activity, error) {
	var err error
	ob := act.Object

	var found pub.Item
	if loader, ok := l.(s.ReadStore); ok {
		found, _ = loader.Load(ob.GetLink())
		if found.IsCollection() {
			pub.OnCollectionIntf(found, func(col pub.CollectionInterface) error {
				found = col.Collection().First()
				return nil
			})
		}
	}
	if pub.IsNil(found) {
		return act, errors.NotFoundf("Unable to find %s %s", ob.GetType(), ob.GetLink())
	}
	ob, err = pub.CopyItemProperties(found, ob)
	if err != nil {
		return act, err
	}

	if err := updateUpdateActivityObject(l, act.Object, act); err != nil {
		return act, errors.Annotatef(err, "unable to update activity's object %s", act.Object.GetLink())
	}
	act.Object, err = l.Save(ob)
	return act, err
}

func updateObjectForUpdate(l s.WriteStore, o *pub.Object, act *pub.Activity) error {
	if o.InReplyTo != nil {
		if colSaver, ok := l.(s.CollectionStore); ok {
			if c, ok := o.InReplyTo.(pub.ItemCollection); ok {
				for _, repl := range c {
					iri := handlers.Replies.IRI(repl.GetLink())
					colSaver.AddTo(iri, o.GetLink())
				}
			} else {
				iri := handlers.Replies.IRI(o.InReplyTo)
				colSaver.AddTo(iri, o.GetLink())
			}
		}
	}
	// We're trying to automatically save tags as separate objects instead of storing them inline in the current
	// Object.
	return createNewTags(l, o.Tag, act)
}

func updateUpdateActivityObject(l s.WriteStore, o pub.Item, act *pub.Activity) error {
	return pub.OnObject(o, func(o *pub.Object) error {
		return updateObjectForUpdate(l, o, act)
	})
}

func updateObjectForCreate(l s.WriteStore, o *pub.Object, act *pub.Activity) error {
	// See https://www.w3.org/TR/ActivityPub/#create-activity-outbox
	// Copying the actor's IRI to the object's AttributedTo
	o.AttributedTo = act.Actor.GetLink()

	// Merging the activity's and the object's Audience
	if aud := pub.ItemCollectionDeduplication(&act.Audience, &o.Audience); aud != nil {
		o.Audience = pub.FlattenItemCollection(aud)
		act.Audience = pub.FlattenItemCollection(aud)
	}
	// Merging the activity's and the object's To addressing
	if to := pub.ItemCollectionDeduplication(&act.To, &o.To); to != nil {
		o.To = pub.FlattenItemCollection(to)
		act.To = pub.FlattenItemCollection(to)
	}
	// Merging the activity's and the object's Bto addressing
	if bto := pub.ItemCollectionDeduplication(&act.Bto, &o.Bto); bto != nil {
		o.Bto = pub.FlattenItemCollection(bto)
		act.Bto = pub.FlattenItemCollection(bto)
	}
	// Merging the activity's and the object's Cc addressing
	if cc := pub.ItemCollectionDeduplication(&act.CC, &o.CC); cc != nil {
		o.CC = pub.FlattenItemCollection(cc)
		act.CC = pub.FlattenItemCollection(cc)
	}
	// Merging the activity's and the object's Bcc addressing
	if bcc := pub.ItemCollectionDeduplication(&act.BCC, &o.BCC); bcc != nil {
		o.BCC = pub.FlattenItemCollection(bcc)
		act.BCC = pub.FlattenItemCollection(bcc)
	}

	// TODO(marius): Move these to a ProcessObject function
	// Set the published date
	if o.Published.IsZero() {
		o.Published = time.Now().UTC()
	}
	return updateObjectForUpdate(l, o, act)
}

func updateCreateActivityObject(l s.WriteStore, o pub.Item, act *pub.Activity) error {
	return pub.OnObject(o, func(o *pub.Object) error {
		return updateObjectForCreate(l, o, act)
	})
}
