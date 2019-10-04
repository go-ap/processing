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
			it = p
		}
	} else if as.ObjectTypes.Contains(it.GetType()) {
		if o, err := activitypub.ToObject(it); err == nil {
			if repl, err := r.CreateCollection(getCollection(o, handlers.Replies)); err != nil {
				return it, errors.Errorf("could not create bucket for collection %s", err)
			} else {
				o.Replies = repl.GetLink()
			}
			if ls, err := r.CreateCollection(getCollection(o, handlers.Likes)); err != nil {
				return it, errors.Errorf("could not create bucket for collection %s", err)
			} else {
				o.Likes = ls.GetLink()
			}
			if sh, err := r.CreateCollection(getCollection(o, handlers.Shares)); err != nil {
				return it, errors.Errorf("could not create bucket for collection %s", err)
			} else {
				o.Shares = sh.GetLink()
			}
			it = o
		}
	}
	return it, nil
}

// ProcessActivity
func ProcessActivity(r s.Saver, act *as.Activity, col handlers.CollectionType) (*as.Activity, error) {
	var err error

	iri := act.GetLink()
	if len(iri) == 0 {
		r.GenerateID(act, nil)
	}
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
			authorOutbox := as.IRI(fmt.Sprintf("%s/%s", act.Actor.GetLink(), handlers.Outbox))
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
			err := colSaver.AddToCollection(colIRI, act.GetLink())
			if err != nil {
				return act, err
			}
		}
	}

	return act, err
}

func updateActivityObject(l s.Saver, o *as.Object, act *as.Activity, now time.Time) error {
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
			for _, repl := range o.InReplyTo {
				iri := as.IRI(fmt.Sprintf("%s/%s", repl.GetLink(), handlers.Replies))
				colSaver.AddToCollection(iri, o.GetLink())
			}
		}
	}

	// TODO(marius): Move these to a ProcessObject function
	// Set the published date
	o.Published = now

	return nil
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
		_, err = UpdateActivity(l, act)
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

// UpdateActivity
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

// AppreciationActivity
func AppreciationActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	good := as.ActivityVocabularyTypes{as.LikeType, as.DislikeType}
	if !good.Contains(act.Type) {
		return act, errors.NotValidf("Activity has wrong type %s, expected %v", act.Type, good)
	}
	var err error

	liked := as.IRI(fmt.Sprintf("%s/%s", act.Actor.GetLink(), handlers.Liked))
	likes := as.IRI(fmt.Sprintf("%s/%s", act.Object.GetLink(), handlers.Likes))
	if loader, ok := l.(s.ActivityLoader); ok {
		basePath := path.Dir(string(act.GetLink()))
		f := as.Activity{
			Parent: as.Object{
				ID:   as.ObjectID(basePath),
				Type: act.Type,
			},
			Actor: act.Actor.GetLink(),
			Object: act.Object.GetLink(),
		}
		filter := s.FilterItem(f)

		// TODO(marius): we need to check if an activity matching the Type and Actor of the current
		//     one already exists, and error out if it does.
		existing, cnt, err := loader.LoadActivities(filter)
		if cnt > 0 && err == nil {
			return act, errors.Newf("A %s activity already exists for current actor/object: %s", act.GetType(), existing.GetLink())
		}
	}

	if colSaver, ok := l.(s.CollectionSaver); ok {
		err = colSaver.AddToCollection(liked, act.Object.GetLink())
		err = colSaver.AddToCollection(likes, act.GetLink())
	}

	return act, err
}

// ReactionsActivity processes matching activities
// The Reactions use case primarily deals with reactions to content.
// This can include activities such as liking or disliking content, ignoring updates,
// flagging content as being inappropriate, accepting or rejecting objects, etc.
func ReactionsActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	var err error
	if act.Object != nil {
		switch act.Type {
		case as.DislikeType:
			fallthrough
		case as.LikeType:
			AppreciationActivity(l, act)
		case as.BlockType:
			fallthrough
		case as.AcceptType:
			// TODO(marius): either the actor or the object needs to be local for this action to be valid
			// in the case of C2S... the actor needs to be local
			// in the case of S2S... the object is
			fallthrough
		case as.FlagType:
			fallthrough
		case as.IgnoreType:
			fallthrough
		case as.RejectType:
			fallthrough
		case as.TentativeAcceptType:
			fallthrough
		case as.TentativeRejectType:
			return act, errors.NotImplementedf("Processing reaction activity of type %s is not implemented", act.GetType())
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
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// EventRSVPActivity processes matching activities
// The Event RSVP use case primarily deals with invitations to events and RSVP type responses.
func EventRSVPActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// GroupManagementActivity processes matching activities
// The Group Management use case primarily deals with management of groups.
// It can include, for instance, activities such as "John added Sally to Group A", "Sally joined Group A",
// "Joe left Group A", etc.
func GroupManagementActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// ContentExperienceActivity processes matching activities
// The Content Experience use case primarily deals with describing activities involving listening to,
// reading, or viewing content. For instance, "Sally read the article", "Joe listened to the song".
func ContentExperienceActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// GeoSocialEventsActivity processes matching activities
// The Geo-Social Events use case primarily deals with activities involving geo-tagging type activities. For instance,
// it can include activities such as "Joe arrived at work", "Sally left work", and "John is travel from home to work".
func GeoSocialEventsActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// NotificationActivity processes matching activities
// The Notification use case primarily deals with calling attention to particular objects or notifications.
func NotificationActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// QuestionActivity processes matching activities
// The Questions use case primarily deals with representing inquiries of any type. See 5.4
// Representing Questions for more information.
func QuestionActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// RelationshipManagementActivity processes matching activities
// The Relationship Management use case primarily deals with representing activities involving the management
// of interpersonal and social relationships (e.g. friend requests, management of social network, etc).
// See 5.2 Representing Relationships Between Entities for more information:
// https://www.w3.org/TR/activitystreams-vocabulary/#connections
func RelationshipManagementActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// NegatingActivity processes matching activities
// The Negating Activity use case primarily deals with the ability to redact previously completed activities.
// See 5.5 Inverse Activities and "Undo" for more information:
// https://www.w3.org/TR/activitystreams-vocabulary/#inverse
func NegatingActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for %s Activity", act.Type)
	}
	if act.Actor == nil {
		return act, errors.NotValidf("Missing actor for %s Activity", act.Type)
	}
	if act.Type != as.UndoType {
		return act, errors.NotValidf("Activity has wrong type %s, expected %s", act.Type, as.UndoType)
	}
	// the object of the activity needs to be an activity
	if !as.ActivityTypes.Contains(act.Object.GetType()) {
		return act, errors.NotValidf("Activity object has wrong type %s, expected one of %v", act.Type, as.ActivityTypes)
	}
	// dereference object activity
	if act.Object.IsLink() {
		if actLoader, ok := l.(s.ActivityLoader); ok {
			obj, _, err := actLoader.LoadActivities(act.Object.GetLink())
			if err != nil {
				return act, errors.NotValidf("Unable to dereference object: %s", act.Object.GetLink())
			}
			act.Object = obj
		}
	}
	err := activitypub.OnActivity(act.Object, func(a *as.Activity) error {
		if act.Actor.GetLink() != a.Actor.GetLink() {
			return errors.NotValidf("The Undo activity has a different actor than its object: %s, expected %s", act.Actor.GetLink(), a.Actor.GetLink())
		}
		// TODO(marius): add more valid types
		good := as.ActivityVocabularyTypes{as.LikeType, as.DislikeType, as.BlockType, as.FollowType}
		if !good.Contains(act.Type) {
			return errors.NotValidf("Object Activity has wrong type %s, expected %v", act.Type, good)
		}
		return nil
	})
	if err != nil {
		return act, err
	}
	return UndoActivity(l, act)
}

// OffersActivity processes matching activities
// The Offers use case deals with activities involving offering one object to another. It can include, for instance,
// activities such as "Company A is offering a discount on purchase of Product Z to Sally",
// "Sally is offering to add a File to Folder A", etc.
func OffersActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// UndoAppreciationActivity
func UndoAppreciationActivity(r s.Saver, act *as.Activity) (*as.Activity, error) {
	var err error
	if colSaver, ok := r.(s.CollectionSaver); ok {
		liked := as.IRI(fmt.Sprintf("%s/%s", act.Actor.GetLink(), handlers.Liked))
		err = colSaver.RemoveFromCollection(liked, act.Object.GetLink())

		likes := as.IRI(fmt.Sprintf("%s/%s", act.Object.GetLink(), handlers.Likes))
		err = colSaver.RemoveFromCollection(likes, act.GetLink())
	}

	return act, err
}

// UndoActivity
func UndoActivity(r s.Saver, act *as.Activity) (*as.Activity, error) {
	var err error

	iri := act.GetLink()
	if len(iri) == 0 {
		r.GenerateID(act, nil)
	}
	err = activitypub.OnActivity(act.Object, func(toUndo *as.Activity) error {
		switch toUndo.GetType() {
		case as.DislikeType:
			fallthrough
		case as.LikeType:
			UndoAppreciationActivity(r, toUndo)
		case as.BlockType:
			fallthrough
		case as.FlagType:
			fallthrough
		case as.IgnoreType:
			return errors.NotImplementedf("Undoing %s is not implemented", toUndo.GetType())
		}
		return nil
	})
	return act, err
}

// UpdateObjectProperties updates the "old" object properties with "new's"
func UpdateObjectProperties(old, new *activitypub.Object) (*activitypub.Object, error) {
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
	old.InReplyTo = replaceIfItemCollection(old.InReplyTo, new.InReplyTo)
	old.Location = replaceIfItem(old.Location, new.Location)
	old.Preview = replaceIfItem(old.Preview, new.Preview)
	if old.Published.IsZero() && !new.Published.IsZero() {
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
	old.Source = replaceIfSource(old.Source, new.Source)
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
		return to, errors.Newf("Invalid object types for update %s(old) and %s(new)", from.GetType(), to.GetType())
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
		o, err := activitypub.ToObject(to)
		if err != nil {
			return o, err
		}
		n, err := activitypub.ToObject(from)
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

func replaceIfSource(old, new activitypub.Source) activitypub.Source {
	if new.MediaType != old.MediaType {
		return new
	}
	old.Content = replaceIfNaturalLanguageValues(old.Content, new.Content)
	return old
}
