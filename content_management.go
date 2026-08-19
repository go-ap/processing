package processing

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/client"
	"github.com/go-ap/errors"
	"github.com/go-ap/filters"
)

type (
	// IDGenerator takes an ActivityStreams item, the collection in which it will be stored in,
	// and the activity that has it as object:
	//  "it" is the item we want to generate the ID for.
	//  "partOf" represents the IRI of the Collection that "it" will be a part of.
	//  "by" represents the Activity that generated the object.
	IDGenerator func(it vocab.Item, byActivity vocab.Item) (vocab.ID, error)
	// IRIValidator designates the type for a function that can validate an IRI
	// It's currently used as the type for var isLocalIRI
	IRIValidator func(i vocab.IRI) bool
)

func setID(id vocab.IRI) func(ob *vocab.Object) error {
	return func(ob *vocab.Object) error {
		ob.ID = id
		return nil
	}
}

func emptyIDGenerator(it vocab.Item, maybeCreate vocab.Item) (vocab.ID, error) {
	id := vocab.NilID
	if vocab.IsNil(it) {
		return id, errors.Newf("unable to set ID on nil item")
	}
	when := time.Now()
	if id.Equal(vocab.NilID) && !vocab.IsNil(maybeCreate) {
		id = maybeCreate.GetLink().AddPath(timeIDFn(when))
	}
	if id.Equal(vocab.NilID) {
		return id, errors.Newf("unable to generate ID, both the storing collection and the generating activity are nil")
	}
	return id, nil
}

func defaultLocalIRICheck(i vocab.IRI) bool { return false }

func defaultKeyGenerator(_ *vocab.Actor) error { return nil }

var timeIDFn = func(t time.Time) string { return fmt.Sprintf("%d", t.UnixMilli()) }

func defaultIDGenerator(base vocab.IRI) IDGenerator {
	return func(it vocab.Item, byActivity vocab.Item) (vocab.ID, error) {
		when := time.Now().Truncate(time.Millisecond).UTC()
		_ = vocab.OnObject(it, func(o *vocab.Object) error {
			if !o.Published.IsZero() {
				when = o.Published
			}
			if o.AttributedTo != nil {
				base = o.AttributedTo.GetLink()
			}
			return nil
		})
		return base.AddPath(timeIDFn(when)), nil
	}
}

func SetIDIfMissing(it vocab.Item, parentActivity vocab.Item, createIDFn IDGenerator) error {
	if !vocab.IsItemCollection(it) {
		if len(it.GetID()) > 0 {
			return nil
		}
		id, err := createIDFn(it, parentActivity)
		if err != nil {
			return err
		}
		if it.GetID().Equal("") {
			// NOTE(marius): if the createIDFn didn't set the ID itself, we set it.
			// This feels a bit redundant at the moment, especially since we can get the ID with it.GetID() if we need it.
			// A potential solution is to remove the ID from the return value of the createIDFn and expect the funciton
			// to always set it.
			return vocab.OnObject(it, setID(id))
		}
		return nil
	}
	colCreateId := func(it vocab.Item, byActivity vocab.Item, idx int) (vocab.ID, error) {
		iri, err := createIDFn(it, byActivity)
		if err != nil {
			return iri, err
		}
		return iri.AddPath(strconv.Itoa(idx + 1)), nil
	}
	return vocab.OnItemCollection(it, func(col *vocab.ItemCollection) error {
		m := make([]error, 0)
		for i, c := range *col {
			if len(c.GetID()) > 0 {
				continue
			}
			if _, err := colCreateId(c, parentActivity, i); err != nil {
				m = append(m, err)
			}
		}
		return errors.Join(m...)
	})
}

// ContentManagementActivityFromClient processes matching activities.
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-crud
//
// The Content Management use case primarily deals with activities that involve the creation,
// modification or deletion of content.
// This includes, for instance, activities such as "John created a new note",
// "Sally updated an article", and "Joe deleted the photo".
func ContentManagementActivityFromClient(p *P, act *vocab.Activity) (*vocab.Activity, error) {
	var err error
	switch {
	case vocab.CreateType.Match(act.Type):
		act, err = CreateActivityFromClient(p, act)
	case vocab.UpdateType.Match(act.Type):
		act, err = p.UpdateActivity(act)
	case vocab.DeleteType.Match(act.Type):
		act, err = DeleteActivity(p.s, act)
	}
	if err != nil && !isDuplicateKey(err) {
		return act, err
	}

	if !vocab.DeleteType.Match(act.Type) {
		if !vocab.IsNil(act.Tag) {
			// NOTE(marius): try to save tags as set on the activity
			// Upd : see if this is something we still need. /Marius 26-04-01
			_ = p.createNewTags(act.Tag, act)
		}

		// NOTE(marius): for binary content objects (Image, Video, Audio) we use the content
		// property as a base64 encoded container using the data URI scheme (RFC2397).
		// So we need to remove that before we disseminate the object/activity to remote actors' inboxes.
		//
		// TODO(marius): add support for cleaning up the URL with the same rule as Content.
		//
		_ = vocab.OnItem(act.Object, cleanupMediaObjectFromItem)
	}

	return act, err
}

func contentHasBinaryData(nlv vocab.NaturalLanguageValues) bool {
	for _, nv := range nlv {
		if bytes.HasPrefix(nv, []byte("data:")) {
			return true
		}
	}
	return false
}

func cleanupMediaObject(o *vocab.Object) error {
	if contentHasBinaryData(o.Content) {
		// NOTE(marius): remove inline content from media ActivityPub objects
		o.Content = nil
		if vocab.IsNil(o.URL) {
			o.URL = o.ID
		} else {
			_ = vocab.OnItem(o.URL, func(u vocab.Item) error {
				// Add an explicit URL if missing.
				if _, ok := u.(vocab.IRI); ok {
					o.URL = o.ID
				}
				_ = vocab.OnObject(u, func(uu *vocab.Object) error {
					uu.ID = o.ID
					return nil
				})
				_ = vocab.OnLink(u, func(uu *vocab.Link) error {
					uu.Href = o.ID
					return nil
				})
				return nil
			})
		}
	}
	return vocab.OnItem(o.Attachment, cleanupMediaObjectFromItem)
}

func cleanupMediaObjectFromActivity(act *vocab.Activity) error {
	if err := vocab.OnItem(act.Object, cleanupMediaObjectFromItem); err != nil {
		return err
	}
	if err := vocab.OnItem(act.Target, cleanupMediaObjectFromItem); err != nil {
		return err
	}
	return nil
}

func cleanupMediaObjectFromItem(it vocab.Item) error {
	if vocab.IsNil(it) {
		return nil
	}
	if vocab.ActivityTypes.Match(it.GetType()) {
		return vocab.OnActivity(it, cleanupMediaObjectFromActivity)
	}
	return vocab.OnObject(it, cleanupMediaObject)
}

// validateCreateObjectIsNew checks if "ob" already exists in storage
// It is used to verify than when receiving a Create activity, we don't override by mistake existing objects.
func validateCreateObjectIsNew(p *P, ob vocab.Item) error {
	if vocab.IsNil(ob) {
		return errors.BadRequestf("the passed object is nil")
	}

	checkIfExists := func(it vocab.Item) bool {
		absent := true
		_ = vocab.OnObject(it, func(ob *vocab.Object) error {
			// NOTE(marius): it is valid to have an object without an ID when processing a C2S Create activity
			// it only means we'll be using our ID generator function to create one
			absent = len(ob.ID) == 0
			return nil
		})

		if !absent {
			it, _ = p.s.Load(it.GetLink())
			absent = vocab.IsNil(it)
		}
		return !absent
	}

	if vocab.IsItemCollection(ob) {
		return vocab.OnCollectionIntf(ob, func(col vocab.CollectionInterface) error {
			for _, ci := range col.Collection() {
				if checkIfExists(ci) {
					return errors.Conflictf("one of the passed objects already exists %s", ci.GetLink())
				}
			}
			return nil
		})
	}
	if checkIfExists(ob) {
		return errors.Conflictf("the passed object already exists %s", ob.GetLink())
	}
	return nil
}

// CreateActivityFromClient
//
// https://www.w3.org/TR/activitypub/#create-activity-outbox
//
// The "Create" activity is used when posting a new object. This has the side effect that the object embedded within the
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
// inbox, and it is likely that the server will want to locally store a representation of this activity and its
// accompanying object. However, this mostly happens in general with processing activities delivered to an inbox anyway.
func CreateActivityFromClient(p *P, act *vocab.Activity) (*vocab.Activity, error) {
	err := validateCreateObjectIsNew(p, act.Object)
	if err != nil {
		return act, err
	}

	if vocab.ActorTypes.Match(act.Object.GetType()) {
		// TODO(marius): @PreHook@ we can replace this with a pre-hook function on Create activities to create they keys
		if err = vocab.OnActor(act.Object, p.actorKeyGenFn); err != nil {
			return act, errors.Annotatef(err, "unable to generate private/public key pair for object %s", act.Object.GetLink())
		}
	}

	// TODO(marius): @PreHook@ we can replace this functionality with a function that creates the collections
	if err = p.CreateCollectionsForObject(act.Object); err != nil {
		return act, errors.Annotatef(err, "unable to save collections for object")
	}

	if err = p.updateCreateActivityObject(act.Object, act); err != nil {
		return act, errors.Annotatef(err, "unable to create activity's object %s", act.Object.GetLink())
	}

	if act.Object, err = p.s.Save(vocab.FlattenProperties(act.Object)); err != nil {
		return act, errors.Annotatef(err, "unable to save object to storage %s", act.Object.GetLink())
	}

	return act, disseminateActivityObjectToLocalReplyToCollections(p, act)
}

// UndoCreateActivity
//
// Removes the side effects of an existing Create activity
// Currently this means only removal of the Create object
func (p P) UndoCreateActivity(create *vocab.Activity) (*vocab.Activity, error) {
	if create == nil {
		return create, InvalidActivity("nil Create activity")
	}
	errs := make([]error, 0)
	toRemove := create.GetLink()

	allRec := create.Recipients()
	removeCollectionOperations := make(map[vocab.IRI]vocab.ItemCollection)
	if p.IsLocal(create.Actor) {
		removeCollectionOperations[vocab.Outbox.IRI(create.Actor)] = vocab.ItemCollection{toRemove}
	}
	for _, rec := range allRec {
		recIRI := rec.GetLink()
		if recIRI == vocab.PublicNS || !p.IsLocalIRI(recIRI) {
			continue
		}
		if !vocab.ValidCollectionIRI(recIRI) {
			// if not a valid collection, then the current recIRI represents an actor, and we need their inbox
			if recIRI = vocab.Inbox.IRI(recIRI); !p.IsLocalIRI(recIRI) {
				continue
			}
		}
		removeCollectionOperations[recIRI] = vocab.ItemCollection{toRemove}
	}
	created := create.Object
	_ = vocab.OnObject(created, func(ob *vocab.Object) error {
		if ob.InReplyTo != nil {
			return vocab.OnItem(ob.InReplyTo, func(replyTo vocab.Item) error {
				if !p.IsLocal(replyTo) {
					return nil
				}
				removeCollectionOperations[vocab.Replies.IRI(replyTo)] = vocab.ItemCollection{created}
				return nil
			})
		}
		return nil
	})

	for colIRI, iris := range removeCollectionOperations {
		if err := p.s.RemoveFrom(colIRI, iris...); err != nil && !errors.IsNotFound(err) {
			errs = append(errs, errors.Annotatef(err, "unable to remove from collection %s", colIRI))
		}
	}
	if len(errs) > 0 {
		return create, errors.Annotatef(errors.Join(errs...), "failed to Undo Create activity")
	}

	err := vocab.OnItem(created, func(ob vocab.Item) error {
		if !p.IsLocal(ob) {
			return nil
		}
		return p.s.Delete(ob.GetLink())
	})
	return create, err
}

func (p P) saveCollectionObjectForParent(parent, colIt vocab.Item) error {
	if vocab.IsNil(colIt) {
		// NOTE(marius): We respect the originating's object creator intention regarding which collections of an object to
		// create, so it's their responsibility to populate them with IRIs or full Collection Objects.
		return nil
	}
	if vocab.IsIRI(colIt) {
		// NOTE(marius): if the collection passed from the parent object is a Collection type we respect that,
		// otherwise we replace it with an OrderedCollection.
		colIt = blankOrderedCollection(colIt.GetLink())
	}
	if _, err := p.s.Load(colIt.GetLink()); err == nil {
		return nil
	}

	var to, cc, bto, bcc, audience vocab.ItemCollection
	published := time.Now().Truncate(time.Second).UTC()
	_ = vocab.OnObject(parent, func(p *vocab.Object) error {
		to = p.To
		bto = p.Bto
		cc = p.CC
		bcc = p.BCC
		audience = p.Audience
		if !p.Published.IsZero() {
			published = p.Published
		}
		return nil
	})

	if _, maybePrivateCol := filters.HiddenCollections.Split(colIt.GetLink()); maybePrivateCol != vocab.Unknown {
		// NOTE(marius): for blocked and ignored collections we forcibly remove the public collection
		to = nil
		bto = vocab.ItemCollection{parent.GetID()}
		cc = nil
		bcc = nil
		audience = nil
	}

	_ = vocab.OnObject(colIt, func(c *vocab.Object) error {
		c.To = to
		c.CC = cc
		c.Bto = bto
		c.BCC = bcc
		c.Audience = audience
		c.Published = published
		if authorIRI := parent.GetLink(); authorIRI != "" {
			c.AttributedTo = authorIRI
		}
		return nil
	})
	_, err := p.s.Save(colIt)
	return err
}

func blankOrderedCollection(iri vocab.IRI) *vocab.OrderedCollection {
	return &vocab.OrderedCollection{ID: iri, Type: vocab.OrderedCollectionType}
}

// CreateCollectionsForObject creates the objects corresponding to each collection that an Actor has set.
func (p *P) CreateCollectionsForObject(it vocab.Item) error {
	if vocab.IsNil(it) || !vocab.IsObject(it) {
		return nil
	}

	if vocab.ActorTypes.Match(it.GetType()) {
		_ = vocab.OnActor(it, func(a *vocab.Actor) error {
			_ = p.saveCollectionObjectForParent(a, a.Inbox)
			_ = p.saveCollectionObjectForParent(a, a.Outbox)
			_ = p.saveCollectionObjectForParent(a, a.Followers)
			_ = p.saveCollectionObjectForParent(a, a.Following)
			_ = p.saveCollectionObjectForParent(a, a.Liked)
			// NOTE(marius): shadow creating hidden collections for Blocked and Ignored items
			// They do not exist on the actor, so we force their creation
			_ = p.saveCollectionObjectForParent(a, blankOrderedCollection(filters.BlockedType.IRI(a)))
			_ = p.saveCollectionObjectForParent(a, blankOrderedCollection(filters.IgnoredType.IRI(a)))
			return nil
		})
	}
	return vocab.OnObject(it, func(o *vocab.Object) error {
		_ = p.saveCollectionObjectForParent(o, o.Replies)
		_ = p.saveCollectionObjectForParent(o, o.Likes)
		_ = p.saveCollectionObjectForParent(o, o.Shares)
		return nil
	})
}

func deref(ctx context.Context, c client.Basic, it vocab.Item) (vocab.Item, error) {
	if vocab.IsNil(it) {
		return nil, nil
	}
	if vocab.IsIRI(it) {
		der, err := c.CtxLoadIRI(ctx, it.GetLink())
		if err != nil {
			return it, err
		}
		it = der
	}
	return it, nil
}

func (p P) dereferenceIntransitiveActivityProperties(receivedIn vocab.IRI) func(act *vocab.IntransitiveActivity) error {
	return func(act *vocab.IntransitiveActivity) error {
		ctx := context.TODO()
		var err error
		if act.Actor, err = deref(ctx, p.c, act.Actor); err != nil {
			return err
		}
		if act.Target, err = deref(ctx, p.c, act.Target); err != nil {
			return err
		}
		return nil
	}
}

func (p P) dereferenceActivityProperties(receivedIn vocab.IRI) func(act *vocab.Activity) error {
	return func(act *vocab.Activity) error {
		ctx := context.TODO()
		var err error
		if act.Object, err = deref(ctx, p.c, act.Object); err != nil {
			return err
		}
		return vocab.OnIntransitiveActivity(act, p.dereferenceIntransitiveActivityProperties(receivedIn))
	}
}

func (p P) dereferenceIRIBasedOnInbox(ob vocab.Item, receivedIn vocab.IRI) (vocab.Item, error) {
	return p.c.CtxLoadIRI(context.TODO(), ob.GetLink())
}

func CreateActivityFromServer(p *P, act *vocab.Activity) (*vocab.Activity, error) {
	return act, disseminateActivityObjectToLocalReplyToCollections(p, act)
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
func (p *P) UpdateActivity(upd *vocab.Activity) (*vocab.Activity, error) {
	var err error
	ob := upd.Object

	if vocab.IsItemCollection(ob) {
		err = vocab.OnItemCollection(ob, func(col *vocab.ItemCollection) error {
			for i, it := range *col {
				old, err := p.loadAndUpdateSingleItem(it)
				if err != nil {
					return err
				}
				(*col)[i] = old
			}
			upd.Object = *col
			return nil
		})
		if err != nil {
			return upd, err
		}
	} else {
		old, err := p.loadAndUpdateSingleItem(ob)
		if err != nil {
			return upd, err
		}
		upd.Object = old
	}
	return upd, disseminateActivityObjectToLocalReplyToCollections(p, upd)
}

func (p *P) loadAndUpdateSingleItem(it vocab.Item) (vocab.Item, error) {
	old, err := p.s.Load(it.GetLink())
	if err != nil {
		return it, err
	}
	if old, err = p.updateSingleItem(firstOrItem(old), it); err != nil {
		return it, err
	}
	return old, nil
}

func CleanOrderedCollectionDynamicProperties(col *vocab.OrderedCollection) error {
	col.First = nil
	col.OrderedItems = nil
	col.TotalItems = 0
	return nil
}

func CleanOrderedCollectionPageDynamicProperties(col *vocab.OrderedCollectionPage) error {
	col.First = nil
	col.OrderedItems = nil
	col.TotalItems = 0
	col.Prev = nil
	col.Next = nil
	return nil
}

func CleanCollectionDynamicProperties(col *vocab.Collection) error {
	col.First = nil
	col.Items = nil
	col.TotalItems = 0
	return nil
}

func CleanCollectionPageDynamicProperties(col *vocab.CollectionPage) error {
	col.First = nil
	col.Items = nil
	col.TotalItems = 0
	col.Prev = nil
	col.Next = nil
	return nil
}

func CleanItemCollectionDynamicProperties(it vocab.Item) error {
	if vocab.IsNil(it) || vocab.IsItemCollection(it) {
		return nil
	}
	typ := it.GetType()
	switch {
	case vocab.OrderedCollectionPageType.Match(typ):
		return vocab.OnOrderedCollectionPage(it, CleanOrderedCollectionPageDynamicProperties)
	case vocab.OrderedCollectionType.Match(typ):
		return vocab.OnOrderedCollection(it, CleanOrderedCollectionDynamicProperties)
	case vocab.CollectionPageType.Match(typ):
		return vocab.OnCollectionPage(it, CleanCollectionPageDynamicProperties)
	case vocab.CollectionType.Match(typ):
		return vocab.OnCollection(it, CleanCollectionDynamicProperties)
	}
	return nil
}

func (p *P) updateSingleItem(found vocab.Item, with vocab.Item) (vocab.Item, error) {
	var err error
	if vocab.IsNil(found) {
		return found, errors.NotFoundf("Unable to find %s %s", with.GetType(), with.GetLink())
	}
	if vocab.IsItemCollection(found) {
		return found, errors.Conflictf("IRI %s does not point to a single object", with.GetLink())
	}

	if vocab.CollectionTypes.Match(with.GetType()) {
		_ = CleanItemCollectionDynamicProperties(with)
	}
	found, err = vocab.CopyItemProperties(found, with)
	if err != nil {
		return found, errors.NewConflict(err, "unable to copy item")
	}
	// TODO(marius): @PreHook@ we can replace this functionality with a function that creates the collections
	if err = p.CreateCollectionsForObject(found); err != nil {
		return found, errors.Annotatef(err, "unable to save collections for object")
	}

	if err = p.updateUpdateActivityObject(found); err != nil {
		return with, errors.Annotatef(err, "unable to update activity's object %s", found.GetLink())
	}
	return p.s.Save(found)
}

func (p *P) updateObjectForUpdate(o *vocab.Object) error {
	if o == nil {
		return nil
	}
	if o.Updated.IsZero() {
		o.Updated = time.Now().UTC()
	}
	// NOTE(marius): We're trying to automatically save tags as separate objects instead
	// of storing them inline in the current Object.
	return p.createNewTags(o.Tag, o)
}

func (p *P) updateUpdateActivityObject(o vocab.Item) error {
	if vocab.IsLink(o) {
		return nil
	}
	return vocab.OnObject(o, p.updateObjectForUpdate)
}

func (p *P) updateObjectForCreate(o *vocab.Object, act *vocab.Activity) error {
	if o == nil {
		return nil
	}
	// See https://www.w3.org/TR/ActivityPub/#create-activity-outbox
	// Copying the actor's IRI to the object's "AttributedTo"
	if vocab.IsNil(o.AttributedTo) && !vocab.IsNil(act.Actor) {
		o.AttributedTo = act.Actor.GetLink()
	}

	// Merging the activity's and the object's "Audience"
	if aud := vocab.ItemCollectionDeduplication(&act.Audience, &o.Audience); aud != nil {
		o.Audience = vocab.FlattenItemCollection(aud)
		act.Audience = vocab.FlattenItemCollection(aud)
	}
	// Merging the activity's and the object's "To" addressing
	if to := vocab.ItemCollectionDeduplication(&act.To, &o.To); to != nil {
		o.To = vocab.FlattenItemCollection(to)
		act.To = vocab.FlattenItemCollection(to)
	}
	// Merging the activity's and the object's "Bto" addressing
	if bto := vocab.ItemCollectionDeduplication(&act.Bto, &o.Bto); bto != nil {
		o.Bto = vocab.FlattenItemCollection(bto)
		act.Bto = vocab.FlattenItemCollection(bto)
	}
	// Merging the activity's and the object's "Cc" addressing
	if cc := vocab.ItemCollectionDeduplication(&act.CC, &o.CC); cc != nil {
		o.CC = vocab.FlattenItemCollection(cc)
		act.CC = vocab.FlattenItemCollection(cc)
	}
	// Merging the activity's and the object's "Bcc" addressing
	if bcc := vocab.ItemCollectionDeduplication(&act.BCC, &o.BCC); bcc != nil {
		o.BCC = vocab.FlattenItemCollection(bcc)
		act.BCC = vocab.FlattenItemCollection(bcc)
	}

	// TODO(marius): Move these to a ProcessObject function
	// Set the published date
	if o.Published.IsZero() {
		o.Published = time.Now().UTC()
	}

	// NOTE(marius): now that we've set the object's attributedTo, we
	// can try to set its ID.
	if err := SetIDIfMissing(o, act, p.createIDFn); err != nil {
		return err
	}
	return p.updateObjectForUpdate(o)
}

func (p *P) updateCreateActivityObject(o vocab.Item, act *vocab.Activity) error {
	if vocab.IsLink(o) {
		return nil
	}
	return vocab.OnObject(o, func(o *vocab.Object) error {
		return p.updateObjectForCreate(o, act)
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
func DeleteActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	// NOTE(marius): I think this is only relevant to @c2s processing
	var err error
	ob := act.Object

	var toRemove vocab.ItemCollection
	if err = replaceItemWithTombstone(l, ob, &toRemove); err != nil {
		return act, errors.Annotatef(err, "unable to create tombstone for object %s", ob)
	}

	if len(toRemove) == 0 {
		return act, nil
	}
	result := make(vocab.ItemCollection, 0)
	for _, r := range toRemove {
		r, err = l.Save(r)
		if err != nil {
			return act, errors.Annotatef(err, "unable to save tombstone for object %s", r)
		}
		result = append(result, r)
	}
	act.Object = result.Normalize()
	return act, nil
}

func replaceItemWithTombstone(l WriteStore, it vocab.Item, toRemove *vocab.ItemCollection) error {
	return vocab.OnItem(it, loadTombstoneForDelete(l, toRemove))
}

func removeItemCollections(st WriteStore, it vocab.Item) error {
	removeCollectionObject := func(colIt vocab.Item) (err error) {
		if !vocab.IsNil(colIt) {
			err = st.Delete(colIt.GetLink())
		}
		return err
	}

	if vocab.ActorTypes.Match(it.GetType()) {
		_ = vocab.OnActor(it, func(a *vocab.Actor) error {
			_ = removeCollectionObject(a.Inbox)
			_ = removeCollectionObject(a.Outbox)
			_ = removeCollectionObject(a.Followers)
			_ = removeCollectionObject(a.Following)
			_ = removeCollectionObject(a.Liked)
			_ = removeCollectionObject(filters.BlockedType.IRI(a))
			_ = removeCollectionObject(filters.IgnoredType.IRI(a))
			return nil
		})
	}
	return vocab.OnObject(it, func(o *vocab.Object) error {
		_ = removeCollectionObject(o.Replies)
		_ = removeCollectionObject(o.Likes)
		_ = removeCollectionObject(o.Shares)
		return nil
	})
}

func loadTombstoneForDelete(l WriteStore, toRemove *vocab.ItemCollection) func(vocab.Item) error {
	loader, ok := l.(ReadStore)
	return func(it vocab.Item) error {
		if !ok {
			return errors.NotFoundf("unable to load %s %s", it.GetType(), it.GetLink())
		}

		found, err := loader.Load(it.GetLink())
		if err != nil {
			return err
		}
		if vocab.IsNil(found) {
			return errors.NotFoundf("unable to find %s %s", it.GetType(), it.GetLink())
		}

		// NOTE(marius): we don't want the object's collections to still be accessible
		_ = removeItemCollections(l, it)

		t := vocab.Tombstone{
			ID:         found.GetLink(),
			Type:       vocab.TombstoneType,
			To:         vocab.ItemCollection{vocab.PublicNS},
			Deleted:    time.Now().UTC(),
			FormerType: it.GetType(),
		}
		*toRemove = append(*toRemove, t)
		return nil
	}
}
