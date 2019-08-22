package processing

import (
	"fmt"
	"github.com/go-ap/activitypub"
	as "github.com/go-ap/activitystreams"
	"github.com/go-ap/auth"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
	"path"
	"time"
)

func getCollection(it as.Item, c handlers.CollectionType) as.CollectionInterface {
	return &as.OrderedCollection{
		Parent: as.Parent{
			ID:   as.ObjectID(fmt.Sprintf("%s/%s", it.GetLink(), c)),
			Type: as.OrderedCollectionType,
		},
	}
}

func AddNewObjectCollections(r s.CollectionSaver, it as.Item) (as.Item, error) {
	if as.ActorTypes.Contains(it.GetType()) {
		if p, err := auth.ToPerson(it); err == nil {
			if in, err := r.CreateCollection(getCollection(p, handlers.Inbox)); err != nil {
				return it, errors.Errorf("could not create bucket for collection %s", err)
			} else {
				p.Inbox = in.GetLink()
			}
			if out, err := r.CreateCollection(getCollection(p, handlers.Outbox)); err != nil {
				return it, errors.Errorf("could not create bucket for collection %s", err)
			} else {
				p.Outbox = out.GetLink()
			}
			if fers, err := r.CreateCollection(getCollection(p, handlers.Followers)); err != nil {
				return it, errors.Errorf("could not create bucket for collection %s", err)
			} else {
				p.Followers = fers.GetLink()
			}
			if fing, err := r.CreateCollection(getCollection(p, handlers.Following)); err != nil {
				return it, errors.Errorf("could not create bucket for collection %s", err)
			} else {
				p.Following = fing.GetLink()
			}
			if ld, err := r.CreateCollection(getCollection(p, handlers.Liked)); err != nil {
				return it, errors.Errorf("could not create bucket for collection %s", err)
			} else {
				p.Liked = ld.GetLink()
			}
			if ls, err := r.CreateCollection(getCollection(p, handlers.Likes)); err != nil {
				return it, errors.Errorf("could not create bucket for collection %s", err)
			} else {
				p.Likes = ls.GetLink()
			}
			if sh, err := r.CreateCollection(getCollection(p, handlers.Shares)); err != nil {
				return it, errors.Errorf("could not create bucket for collection %s", err)
			} else {
				p.Shares = sh.GetLink()
			}
			it = p
		}
	} else if as.ObjectTypes.Contains(it.GetType()) {
		if o, err := as.ToObject(it); err == nil {
			if repl, err := r.CreateCollection(getCollection(o, handlers.Replies)); err != nil {
				return it, errors.Errorf("could not create bucket for collection %s", err)
			} else {
				o.Replies = repl.GetLink()
			}
			it = o
		}
	}
	return it, nil
}

// ProcessActivity
func ProcessActivity(r s.Saver, act *as.Activity, col handlers.CollectionType) (*as.Activity, error) {
	var err error

	// TODO(marius): Since we're not failing on the first error, so we can try to process the same type of
	// activity in multiple contexts, we should propagate all the errors to the end, by probably using some
	// errors.Annotatef...

	// First we process the activity to effect whatever changes we need to on the activity properties.
	if as.ContentManagementActivityTypes.Contains(act.GetType()) && act.Object.GetType() != as.RelationshipType {
		act, err = ContentManagementActivity(r, act, col)
		if err != nil {
			return act, err
		}
	}
	if as.CollectionManagementActivityTypes.Contains(act.GetType()) {
		act, err = CollectionManagementActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if as.ReactionsActivityTypes.Contains(act.GetType()) {
		act, err = ReactionsActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if as.EventRSVPActivityTypes.Contains(act.GetType()) {
		act, err = EventRSVPActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if as.GroupManagementActivityTypes.Contains(act.GetType()) {
		act, err = GroupManagementActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if as.ContentExperienceActivityTypes.Contains(act.GetType()) {
		act, err = ContentExperienceActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if as.GeoSocialEventsActivityTypes.Contains(act.GetType()) {
		act, err = GeoSocialEventsActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if as.NotificationActivityTypes.Contains(act.GetType()) {
		act, err = NotificationActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if as.QuestionActivityTypes.Contains(act.GetType()) {
		act, err = QuestionActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if as.RelationshipManagementActivityTypes.Contains(act.GetType()) && act.Object.GetType() == as.RelationshipType {
		act, err = RelationshipManagementActivity(r, act)
		if err == nil {
			return act, errors.Annotatef(err, "%s activity processing failed", act.Type)
		}
	}
	if as.NegatingActivityTypes.Contains(act.GetType()) {
		act, err = NegatingActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if as.OffersActivityTypes.Contains(act.GetType()) {
		act, err = OffersActivity(r, act)
		if err != nil {
			return act, err
		}
	}

	iri := act.GetLink()
	if len(iri) == 0 {
		r.GenerateID(act, nil)
	}

	act = FlattenActivityProperties(act)
	it, err := r.SaveActivity(act)
	act, _ = it.(*as.Activity)

	if colSaver, ok := r.(s.CollectionSaver); ok {
		recipients := act.Recipients()
		for _, fw := range recipients {
			colIRI := fw.GetLink()
			if colIRI == as.PublicNS {
				continue
			}
			authorOutbox :=  as.IRI(fmt.Sprintf("%s/%s", act.Actor.GetLink(), handlers.Outbox))
			if colIRI == act.Actor.GetLink() {
				// the recipient is just the author IRI
				colIRI = authorOutbox
			} else {
				if !handlers.ValidCollection(path.Base(colIRI.String())) {
					// TODO(marius): add check if IRI represents an actor
					colIRI = as.IRI(fmt.Sprintf("%s/%s", colIRI, handlers.Inbox))
				} else {
					// TODO(marius): the recipient consists of a collection, we need to load it's elements if it's local
					//     and save it in each of them. :(
				}
			}
			// TODO(marius): the processing module needs a method to see if an IRI is local or not
			//    For each recipient we need to save the incoming activity to the actor's Inbox if the actor is local
			//    Or disseminate it using S2S if the actor is not local
			colSaver.AddToCollection(colIRI, act.GetLink())
		}
	}

	return act, err
}

func updateActivityObject(l s.Saver, o *as.Object, act *as.Activity, now time.Time) {
	// See https://www.w3.org/TR/ActivityPub/#create-activity-outbox
	// Copying the actor's IRI to the object's AttributedTo
	o.AttributedTo = act.Actor.GetLink()

	// Copying the activity's recipients to the object's
	o.Audience = FlattenItemCollection(act.Recipients())

	// Copying the object's recipients to the activity's audience
	act.Audience = FlattenItemCollection(o.Recipients())

	if o.InReplyTo != nil {
		if colSaver, ok := l.(s.CollectionSaver); ok {
			replies := as.IRI(fmt.Sprintf("%s/%s", o.InReplyTo.GetLink(), handlers.Replies))
			colSaver.AddToCollection(replies, o.GetLink())
		}
	}

	// TODO(marius): Move these to a ProcessObject function
	// Set the published date
	o.Published = now
}

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
		_, err = CreateActivity(l, act)
	case as.UpdateType:
		// TODO(marius): Move this piece of logic to the validation mechanism
		if len(act.Object.GetLink()) == 0 {
			return act, errors.Newf("unable to update object without a valid object id")
		}

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
			updateActivityObject(l, &a.Parent, act, now)
			act.Object = a
			return nil
		})
	} else if as.ActorTypes.Contains(obType) {
		activitypub.OnPerson(act.Object, func(p *activitypub.Person) error {
			updateActivityObject(l, &p.Parent, act, now)
			act.Object = p
			return nil
		})
	} else {
		activitypub.OnObject(act.Object, func(o *as.Object) error {
			updateActivityObject(l, o, act, now)
			act.Object = o
			return nil
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

// ReactionsActivity processes matching activities
// The Reactions use case primarily deals with reactions to content.
// This can include activities such as liking or disliking content, ignoring updates,
// flagging content as being inappropriate, accepting or rejecting objects, etc.
func ReactionsActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	var err error
	if act.Object != nil {
		switch act.Type {
		case as.BlockType:
		case as.AcceptType:
			// TODO(marius): either the actor or the object needs to be local for this action to be valid
			// in the case of C2S... the actor needs to be local
			// in the case of S2S... the object is
		case as.FlagType:
		case as.IgnoreType:
		case as.DislikeType:
			fallthrough
		case as.LikeType:
		case as.RejectType:
		case as.TentativeAcceptType:
		case as.TentativeRejectType:
		}
	}
	return act, err
}

// CollectionManagementActivity processes matching activities
// The Collection Management use case primarily deals with activities involving the management of content within collections.
// Examples of collections include things like folders, albums, friend lists, etc.
// This includes, for instance, activities such as "Sally added a file to Folder A",
// "John moved the file from Folder A to Folder B", etc.
func CollectionManagementActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.Errorf("Not implemented")
}

// EventRSVPActivity processes matching activities
// The Event RSVP use case primarily deals with invitations to events and RSVP type responses.
func EventRSVPActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.Errorf("Not implemented")
}

// GroupManagementActivity processes matching activities
// The Group Management use case primarily deals with management of groups.
// It can include, for instance, activities such as "John added Sally to Group A", "Sally joined Group A",
// "Joe left Group A", etc.
func GroupManagementActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.Errorf("Not implemented")
}

// ContentExperienceActivity processes matching activities
// The Content Experience use case primarily deals with describing activities involving listening to,
// reading, or viewing content. For instance, "Sally read the article", "Joe listened to the song".
func ContentExperienceActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.Errorf("Not implemented")
}

// GeoSocialEventsActivity processes matching activities
// The Geo-Social Events use case primarily deals with activities involving geo-tagging type activities. For instance,
// it can include activities such as "Joe arrived at work", "Sally left work", and "John is travel from home to work".
func GeoSocialEventsActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.Errorf("Not implemented")
}

// NotificationActivity processes matching activities
// The Notification use case primarily deals with calling attention to particular objects or notifications.
func NotificationActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.Errorf("Not implemented")
}

// QuestionActivity processes matching activities
// The Questions use case primarily deals with representing inquiries of any type. See 5.4
// Representing Questions for more information.
func QuestionActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.Errorf("Not implemented")
}

// RelationshipManagementActivity processes matching activities
// The Relationship Management use case primarily deals with representing activities involving the management
// of interpersonal and social relationships (e.g. friend requests, management of social network, etc).
// See 5.2 Representing Relationships Between Entities for more information:
// https://www.w3.org/TR/activitystreams-vocabulary/#connections
func RelationshipManagementActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.Errorf("Not implemented")
}

// NegatingActivity processes matching activities
// The Negating Activity use case primarily deals with the ability to redact previously completed activities.
// See 5.5 Inverse Activities and "Undo" for more information:
// https://www.w3.org/TR/activitystreams-vocabulary/#inverse
func NegatingActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.Errorf("Not implemented")
}

// OffersActivity processes matching activities
// The Offers use case deals with activities involving offering one object to another. It can include, for instance,
// activities such as "Company A is offering a discount on purchase of Product Z to Sally",
// "Sally is offering to add a File to Folder A", etc.
func OffersActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.Errorf("Not implemented")
}

// UpdateObjectProperties updates the "old" object properties with "new's"
func UpdateObjectProperties(old, new *as.Object) (*as.Object, error) {
	old.Name = replaceIfNaturalLanguageValues(old.Name, new.Name)
	old.Attachment = replaceIfItem(old.Attachment, new.Attachment)
	old.AttributedTo = replaceIfItem(old.AttributedTo, new.AttributedTo)
	old.Audience = replaceIfItemCollection(old.Audience, new.Audience)
	old.Content = replaceIfNaturalLanguageValues(old.Content, new.Content)
	old.Context = replaceIfItem(old.Context, new.Context)
	if len(new.MediaType) > 0 {
		old.MediaType = new.MediaType
	}
	if !new.EndTime.IsZero() {
		old.EndTime = new.EndTime
	}
	old.Generator = replaceIfItem(old.Generator, new.Generator)
	old.Icon = replaceIfItem(old.Icon, new.Icon)
	old.Image = replaceIfItem(old.Image, new.Image)
	old.InReplyTo = replaceIfItem(old.InReplyTo, new.InReplyTo)
	old.Location = replaceIfItem(old.Location, new.Location)
	old.Preview = replaceIfItem(old.Preview, new.Preview)
	if !new.Published.IsZero() {
		old.Published = new.Published
	}
	old.Replies = replaceIfItem(old.Replies, new.Replies)
	if !new.StartTime.IsZero() {
		old.StartTime = new.StartTime
	}
	old.Summary = replaceIfNaturalLanguageValues(old.Summary, new.Summary)
	old.Tag = replaceIfItemCollection(old.Tag, new.Tag)
	if !new.Updated.IsZero() {
		old.Updated = new.Updated
	}
	if new.URL != nil {
		old.URL = new.URL
	}
	old.To = replaceIfItemCollection(old.To, new.To)
	old.Bto = replaceIfItemCollection(old.Bto, new.Bto)
	old.CC = replaceIfItemCollection(old.CC, new.CC)
	old.BCC = replaceIfItemCollection(old.BCC, new.BCC)
	if new.Duration == 0 {
		old.Duration = new.Duration
	}
	return old, nil
}

// UpdateItemProperties delegates to the correct per type functions for copying
// properties between matching Activity Objects
func UpdateItemProperties(to, from as.Item) (as.Item, error) {
	if to == nil {
		return to, errors.Newf("Nil object to update")
	}
	if from == nil {
		return to, errors.Newf("Nil object for update")
	}
	if *to.GetID() != *from.GetID() {
		return to, errors.Newf("Object IDs don't match")
	}
	if to.GetType() != from.GetType() {
		return to, errors.Newf("Invalid object types for update")
	}
	if as.ActorTypes.Contains(to.GetType()) {
		o, err := auth.ToPerson(to)
		if err != nil {
			return o, err
		}
		n, err := auth.ToPerson(from)
		if err != nil {
			return o, err
		}
		return UpdatePersonProperties(o, n)
	}
	if as.ObjectTypes.Contains(to.GetType()) {
		o, err := as.ToObject(to)
		if err != nil {
			return o, err
		}
		n, err := as.ToObject(from)
		if err != nil {
			return o, err
		}
		return UpdateObjectProperties(o, n)
	}
	return to, errors.Newf("could not process objects with type %s", to.GetType())
}

// UpdatePersonProperties
func UpdatePersonProperties(old, new *auth.Person) (*auth.Person, error) {
	o, err := UpdateObjectProperties(&old.Parent, &new.Parent)
	old.Parent = *o
	old.Inbox = replaceIfItem(old.Inbox, new.Inbox)
	old.Outbox = replaceIfItem(old.Outbox, new.Outbox)
	old.Following = replaceIfItem(old.Following, new.Following)
	old.Followers = replaceIfItem(old.Followers, new.Followers)
	old.Liked = replaceIfItem(old.Liked, new.Liked)
	old.PreferredUsername = replaceIfNaturalLanguageValues(old.PreferredUsername, new.PreferredUsername)
	return old, err
}

func replaceIfItem(old, new as.Item) as.Item {
	if new == nil {
		return old
	}
	return new
}

func replaceIfItemCollection(old, new as.ItemCollection) as.ItemCollection {
	if new == nil {
		return old
	}
	return new
}

func replaceIfNaturalLanguageValues(old, new as.NaturalLanguageValues) as.NaturalLanguageValues {
	if new == nil {
		return old
	}
	return new
}
