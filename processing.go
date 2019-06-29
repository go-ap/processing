package processing

import (
	"fmt"
	ap "github.com/go-ap/activitypub"
	as "github.com/go-ap/activitystreams"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
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
		if p, err := ap.ToPerson(it); err == nil {
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
func ProcessActivity(r s.Saver, it as.Item) (as.Item, error) {
	var err error

	// TODO(marius): Since we're not failing on the first error, so we can try to process the same type of
	// activity in multiple contexts, we should propagate all the errors to the end, by probably using some
	// errors.Annotatef...

	// First we process the activity to effect whatever changes we need to on the activity properties.
	act, err := as.ToActivity(it)
	if as.ContentManagementActivityTypes.Contains(it.GetType()) && act.Object.GetType() != as.RelationshipType {
		act, err = ContentManagementActivity(r, act)
		if err != nil {
			return it, err
		}
	}
	if as.CollectionManagementActivityTypes.Contains(it.GetType()) {
		act, err = CollectionManagementActivity(r, act)
		if err != nil {
			return it, err
		}
	}
	if as.ReactionsActivityTypes.Contains(it.GetType()) {
		act, err = ReactionsActivity(r, act)
		if err != nil {
			return it, err
		}
	}
	if as.EventRSVPActivityTypes.Contains(it.GetType()) {
		act, err = EventRSVPActivity(r, act)
		if err != nil {
			return it, err
		}
	}
	if as.GroupManagementActivityTypes.Contains(it.GetType()) {
		act, err = GroupManagementActivity(r, act)
		if err != nil {
			return it, err
		}
	}
	if as.ContentExperienceActivityTypes.Contains(it.GetType()) {
		act, err = ContentExperienceActivity(r, act)
		if err != nil {
			return it, err
		}
	}
	if as.GeoSocialEventsActivityTypes.Contains(it.GetType()) {
		act, err = GeoSocialEventsActivity(r, act)
		if err != nil {
			return it, err
		}
	}
	if as.NotificationActivityTypes.Contains(it.GetType()) {
		act, err = NotificationActivity(r, act)
		if err != nil {
			return it, err
		}
	}
	if as.QuestionActivityTypes.Contains(it.GetType()) {
		act, err = QuestionActivity(r, act)
		if err != nil {
			return it, err
		}
	}
	if as.RelationshipManagementActivityTypes.Contains(it.GetType()) && act.Object.GetType() == as.RelationshipType {
		act, err = RelationshipManagementActivity(r, act)
		if err == nil {
			return act, errors.Annotatef(err, "%s activity processing failed", act.Type)
		}
	}
	if as.NegatingActivityTypes.Contains(it.GetType()) {
		act, err = NegatingActivity(r, act)
		if err != nil {
			return it, err
		}
	}
	if as.OffersActivityTypes.Contains(it.GetType()) {
		act, err = OffersActivity(r, act)
		if err != nil {
			return it, err
		}
	}

	iri := it.GetLink()
	if len(iri) == 0 {
		r.GenerateID(it, nil)
	}

	it = FlattenProperties(it)
	return r.SaveActivity(it)
}

// ContentManagementActivity processes matching activities
// The Content Management use case primarily deals with activities that involve the creation,
// modification or deletion of content.
// This includes, for instance, activities such as "John created a new note",
// "Sally updated an article", and "Joe deleted the photo".
func ContentManagementActivity(l s.Saver, act *as.Activity) (*as.Activity, error) {
	var err error
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for Activity")
	}
	now := time.Now().UTC()
	switch act.Type {
	case as.CreateType:
		iri := act.Object.GetLink()
		if len(iri) == 0 {
			l.GenerateID(act.Object, act)
		}
		// TODO(marius) Add function as.AttributedTo(it as.Item, auth as.Item)
		if a, err := as.ToActivity(act.Object); err == nil {
			// See https://www.w3.org/TR/ActivityPub/#create-activity-outbox
			// Copying the actor's IRI to the object's AttributedTo
			a.AttributedTo = act.Actor.GetLink()

			// Setting the Generator to the current service if not specified explicitly
			//if a.Generator == nil && len(ServiceIRI) > 0 {
			//	a.Generator = ServiceIRI
			//}

			aRec := act.Recipients()
			// Copying the activity's recipients to the object's
			a.Audience = aRec
			// Copying the object's recipients to the activity's audience
			act.Audience = a.Recipients()

			// TODO(marius): Move these to a ProcessObject function
			// Set the published date
			a.Published = now

			act.Object = a
		} else if p, err := ap.ToPerson(act.Object); err == nil {
			// See https://www.w3.org/TR/ActivityPub/#create-activity-outbox
			// Copying the actor's IRI to the object's AttributedTo
			p.AttributedTo = act.Actor.GetLink()

			// Setting the Generator to the current service if not specified explicitly
			//if p.Generator == nil && len(ServiceIRI) > 0 {
			//	p.Generator = ServiceIRI
			//}

			aRec := act.Recipients()
			// Copying the activity's recipients to the object's
			p.Audience = aRec
			// Copying the object's recipients to the activity's audience
			act.Audience = p.Recipients()

			// TODO(marius): Move these to a ProcessObject function
			// Set the published date
			p.Published = now

			act.Object = p
		} else if o, err := as.ToObject(act.Object); err == nil {
			// See https://www.w3.org/TR/ActivityPub/#create-activity-outbox
			// Copying the actor's IRI to the object's AttributedTo
			o.AttributedTo = act.Actor.GetLink()

			// Setting the Generator to the current service if not specified explicitly
			//if o.Generator == nil && len(ServiceIRI) > 0 {
			//	o.Generator = ServiceIRI
			//}

			aRec := act.Recipients()
			// Copying the activity's recipients to the object's
			o.Audience = aRec
			// Copying the object's recipients to the activity's audience
			act.Audience = o.Recipients()

			// TODO(marius): Move these to a ProcessObject function
			// Set the published date
			o.Published = now

			act.Object = o
		}

		if colSaver, ok := l.(s.CollectionSaver); ok {
			act.Object, err = AddNewObjectCollections(colSaver, act.Object)
			if err != nil {
				return act, errors.Annotatef(err, "unable to add object collections to object %s", act.Object.GetLink())
			}
		}

		act.Object, err = l.SaveObject(act.Object)
	case as.UpdateType:
		// TODO(marius): Move this piece of logic to the validation mechanism
		if len(act.Object.GetLink()) == 0 {
			return act, errors.Newf("unable to update object without a valid object id")
		}
		act.Object, err = l.UpdateObject(act.Object)
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
		case as.DislikeType:
		case as.FlagType:
		case as.IgnoreType:
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
