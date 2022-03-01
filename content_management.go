package processing

import (
	"fmt"
	"time"

	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
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
		act, err = DeleteActivity(l, act)
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
//
// https://www.w3.org/TR/activitypub/#create-activity-outbox
//
// The Create activity is used when posting a new object. This has the side effect that the object embedded within the
// Activity (in the object property) is created.
// When a Create activity is posted, the actor of the activity SHOULD be copied onto the object's attributedTo field.
// A mismatch between addressing of the Create activity and its object is likely to lead to confusion.
// As such, a server SHOULD copy any recipients of the Create activity to its object upon initial distribution,
// and likewise with copying recipients from the object to the wrapping Create activity.
// Note that it is acceptable for the object's addressing to be changed later without changing the Create's addressing
// (for example via an Update activity).
//
// https://www.w3.org/TR/activitypub/#create-activity-inbox
//
// Receiving a Create activity in an inbox has surprisingly few side effects; the activity should appear in the actor's
// inbox and it is likely that the server will want to locally store a representation of this activity and its
// accompanying object. However, this mostly happens in general with processing activities delivered to an inbox anyway.
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
//
// https://www.w3.org/TR/activitypub/#update-activity-outbox
//
// The Update activity is used when updating an already existing object. The side effect of this is that the object
// MUST be modified to reflect the new structure as defined in the update activity,
// assuming the actor has permission to update this object.
//
// https://www.w3.org/TR/activitypub/#update-activity-inbox
//
// For server to server interactions, an Update activity means that the receiving server SHOULD update its copy of the
// object of the same id to the copy supplied in the Update activity. Unlike the client to server handling of the Update
// activity, this is not a partial update but a complete replacement of the object.
// The receiving server MUST take care to be sure that the Update is authorized to modify its object. At minimum,
// this may be done by ensuring that the Update and its object are of same origin.
func UpdateActivity(l s.WriteStore, act *pub.Activity) (*pub.Activity, error) {
	var err error
	ob := act.Object

	if loader, ok := l.(s.ReadStore); ok {
		if pub.IsItemCollection(ob) {
			foundCol := make(pub.ItemCollection, 0)
			pub.OnCollectionIntf(ob, func(col pub.CollectionInterface) error {
				for _, it := range col.Collection() {
					old, err := loader.Load(it.GetLink())
					if err != nil {
						continue
					}
					if old, err = updateSingleItem(l, old, it); err != nil {
						continue
					}
					foundCol = append(foundCol, old)
				}
				act.Object = foundCol
				return nil
			})
		} else {
			old, err := loader.Load(ob.GetLink())
			if err != nil {
				return act, err
			}
			if old, err = updateSingleItem(l, old, ob); err != nil {
				return act, err
			}
			act.Object = old
		}
	}
	return act, err
}

func updateSingleItem(l s.WriteStore, found pub.Item, with pub.Item) (pub.Item, error) {
	var err error
	if pub.IsNil(found) {
		return found, errors.NotFoundf("Unable to find %s %s", with.GetType(), with.GetLink())
	}
	if found.IsCollection() {
		return found, errors.Errorf("IRI %s does not point to a single object", with.GetLink())
	}

	found, err = pub.CopyItemProperties(found, with)
	if err != nil {
		return found, err
	}

	if err = updateUpdateActivityObject(l, found); err != nil {
		return with, errors.Annotatef(err, "unable to update activity's object %s", found.GetLink())
	}
	return l.Save(found)
}

func updateObjectForUpdate(l s.WriteStore, o *pub.Object) error {
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
	return createNewTags(l, o.Tag)
}

func updateUpdateActivityObject(l s.WriteStore, o pub.Item) error {
	return pub.OnObject(o, func(o *pub.Object) error {
		return updateObjectForUpdate(l, o)
	})
}

func updateObjectForCreate(l s.WriteStore, o *pub.Object, act *pub.Activity) error {
	// See https://www.w3.org/TR/ActivityPub/#create-activity-outbox
	// Copying the actor's IRI to the object's "AttributedTo"
	if pub.IsNil(o.AttributedTo) && !pub.IsNil(act.Actor) {
		o.AttributedTo = act.Actor.GetLink()
	}

	// Merging the activity's and the object's "Audience"
	if aud := pub.ItemCollectionDeduplication(&act.Audience, &o.Audience); aud != nil {
		o.Audience = pub.FlattenItemCollection(aud)
		act.Audience = pub.FlattenItemCollection(aud)
	}
	// Merging the activity's and the object's "To" addressing
	if to := pub.ItemCollectionDeduplication(&act.To, &o.To); to != nil {
		o.To = pub.FlattenItemCollection(to)
		act.To = pub.FlattenItemCollection(to)
	}
	// Merging the activity's and the object's "Bto" addressing
	if bto := pub.ItemCollectionDeduplication(&act.Bto, &o.Bto); bto != nil {
		o.Bto = pub.FlattenItemCollection(bto)
		act.Bto = pub.FlattenItemCollection(bto)
	}
	// Merging the activity's and the object's "Cc" addressing
	if cc := pub.ItemCollectionDeduplication(&act.CC, &o.CC); cc != nil {
		o.CC = pub.FlattenItemCollection(cc)
		act.CC = pub.FlattenItemCollection(cc)
	}
	// Merging the activity's and the object's "Bcc" addressing
	if bcc := pub.ItemCollectionDeduplication(&act.BCC, &o.BCC); bcc != nil {
		o.BCC = pub.FlattenItemCollection(bcc)
		act.BCC = pub.FlattenItemCollection(bcc)
	}

	// TODO(marius): Move these to a ProcessObject function
	// Set the published date
	if o.Published.IsZero() {
		o.Published = time.Now().UTC()
	}
	return updateObjectForUpdate(l, o)
}

func updateCreateActivityObject(l s.WriteStore, o pub.Item, act *pub.Activity) error {
	return pub.OnObject(o, func(o *pub.Object) error {
		return updateObjectForCreate(l, o, act)
	})
}

// DeleteActivity
//
// https://www.w3.org/TR/activitypub/#delete-activity-outbox
//
// The Delete activity is used to delete an already existing object. The side effect of this is that the server MAY
// replace the object with a Tombstone of the object that will be displayed in activities which reference the deleted
// object. If the deleted object is requested the server SHOULD respond with either the HTTP 410 Gone status code
// if a Tombstone object is presented as the response body, otherwise respond with a HTTP 404 Not Found.
//
// https://www.w3.org/TR/activitypub/#delete-activity-inbox
//
// The side effect of receiving this is that (assuming the object is owned by the sending actor / server) the server
// receiving the delete activity SHOULD remove its representation of the object with the same id, and MAY replace that
// representation with a Tombstone object.
//
// Note: that after an activity has been transmitted from an origin server to a remote server, there is nothing in the
//
// ActivityPub protocol that can enforce remote deletion of an object's representation.
func DeleteActivity(l s.WriteStore, act *pub.Activity) (*pub.Activity, error) {
	var err error
	ob := act.Object

	var found pub.Item
	if loader, ok := l.(s.ReadStore); ok {
		if found, err = loader.Load(ob.GetLink()); err != nil {
			return act, err
		}
	}
	if pub.IsNil(found) {
		return act, errors.NotFoundf("Unable to find %s %s", ob.GetType(), ob.GetLink())
	}
	if found.IsCollection() {
		return act, errors.Errorf("IRI %s does not point to a single object", ob.GetLink())
	}
	t := pub.Tombstone{
		ID:      act.Object.GetLink(),
		Type:    pub.TombstoneType,
		To:      pub.ItemCollection{pub.PublicNS},
		Deleted: time.Now().UTC(),
	}
	if found.GetType() != pub.TombstoneType {
		t.FormerType = found.GetType()
	}

	act.Object, err = l.Save(t)
	return act, err
}
