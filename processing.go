package processing

import (
	"fmt"
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
	"path"
	"time"
)

func getCollection(it pub.Item, c handlers.CollectionType) pub.CollectionInterface {
	return &pub.OrderedCollection{
		ID:   pub.ID(fmt.Sprintf("%s/%s", it.GetLink(), c)),
		Type: pub.OrderedCollectionType,
	}
}

func AddNewObjectCollections(r s.CollectionSaver, it pub.Item) (pub.Item, error) {
	if pub.ActorTypes.Contains(it.GetType()) {
		if p, err := pub.ToActor(it); err == nil {
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
	} else if pub.ObjectTypes.Contains(it.GetType()) {
		if o, err := pub.ToObject(it); err == nil {
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
func ProcessActivity(r s.Saver, act *pub.Activity, col handlers.CollectionType) (*pub.Activity, error) {
	var err error

	iri := act.GetLink()
	if len(iri) == 0 {
		r.GenerateID(act, nil)
	}
	// TODO(marius): Since we're not failing on the first error, so we can try to process the same type of
	// activity in multiple contexts, we should propagate all the errors to the end, by probably using some
	// errors.Annotatef...
	// First we process the activity to effect whatever changes we need to on the activity properties.
	if pub.ContentManagementActivityTypes.Contains(act.GetType()) && act.Object.GetType() != pub.RelationshipType {
		act, err = ContentManagementActivity(r, act, col)
		if err != nil {
			return act, err
		}
	}
	if pub.CollectionManagementActivityTypes.Contains(act.GetType()) {
		act, err = CollectionManagementActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if pub.ReactionsActivityTypes.Contains(act.GetType()) {
		act, err = ReactionsActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if pub.EventRSVPActivityTypes.Contains(act.GetType()) {
		act, err = EventRSVPActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if pub.GroupManagementActivityTypes.Contains(act.GetType()) {
		act, err = GroupManagementActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if pub.ContentExperienceActivityTypes.Contains(act.GetType()) {
		act, err = ContentExperienceActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if pub.GeoSocialEventsActivityTypes.Contains(act.GetType()) {
		act, err = GeoSocialEventsActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if pub.NotificationActivityTypes.Contains(act.GetType()) {
		act, err = NotificationActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if pub.QuestionActivityTypes.Contains(act.GetType()) {
		act, err = QuestionActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if pub.RelationshipManagementActivityTypes.Contains(act.GetType()) && act.Object.GetType() == pub.RelationshipType {
		act, err = RelationshipManagementActivity(r, act)
		if err == nil {
			return act, errors.Annotatef(err, "%s activity processing failed", act.Type)
		}
	}
	if pub.NegatingActivityTypes.Contains(act.GetType()) {
		act, err = NegatingActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	if pub.OffersActivityTypes.Contains(act.GetType()) {
		act, err = OffersActivity(r, act)
		if err != nil {
			return act, err
		}
	}
	act = FlattenActivityProperties(act)
	if act.Published.IsZero() {
		act.Published = time.Now()
	}
	it, err := r.SaveActivity(act)
	act, _ = it.(*pub.Activity)

	if colSaver, ok := r.(s.CollectionSaver); ok {
		authorOutbox := pub.IRI(fmt.Sprintf("%s/%s", act.Actor.GetLink(), handlers.Outbox))
		if err := colSaver.AddToCollection(authorOutbox, act.GetLink()); err != nil {
			return act, err
		}
		recipients := act.Recipients()
		for _, fw := range recipients {
			colIRI := fw.GetLink()
			if colIRI == pub.PublicNS {
				continue
			}
			// TODO(marius): This needs to check and do the following things only for local Collections and IRIs
			if !handlers.ValidCollection(path.Base(colIRI.String())) {
				// TODO(marius): add check if IRI represents an actor
				colIRI = pub.IRI(fmt.Sprintf("%s/%s", colIRI, handlers.Inbox))
			} else {
				// TODO(marius): the recipient consists of a collection, we need to load it's elements if it's local
				//     and save it in each of them. :(
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

func updateActivity(act *pub.Activity, now time.Time) error {
	// Merging the activity's and the object's Audience

	// Set the published date
	act.Published = now
	return nil
}

// CollectionManagementActivity processes matching activities
// The Collection Management use case primarily deals with activities involving the management of content within collections.
// Examples of collections include things like folders, albums, friend lists, etc.
// This includes, for instance, activities such as "Sally added a file to Folder A",
// "John moved the file from Folder A to Folder B", etc.
func CollectionManagementActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for Activity")
	}
	if act.Target == nil {
		return act, errors.NotValidf("Missing target collection for Activity")
	}
	switch act.Type {
	case pub.AddType:
	case pub.MoveType:
	case pub.RemoveType:
	default:
		return nil, errors.NotValidf("Invalid type %s", act.GetType())
	}
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// EventRSVPActivity processes matching activities
// The Event RSVP use case primarily deals with invitations to events and RSVP type responses.
func EventRSVPActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for Activity")
	}
	switch act.Type {
	case pub.AcceptType:
	case pub.IgnoreType:
	case pub.InviteType:
	case pub.RejectType:
	case pub.TentativeAcceptType:
	case pub.TentativeRejectType:
	default:
		return nil, errors.NotValidf("Invalid type %s", act.GetType())
	}
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// GroupManagementActivity processes matching activities
// The Group Management use case primarily deals with management of groups.
// It can include, for instance, activities such as "John added Sally to Group A", "Sally joined Group A",
// "Joe left Group A", etc.
func GroupManagementActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// ContentExperienceActivity processes matching activities
// The Content Experience use case primarily deals with describing activities involving listening to,
// reading, or viewing content. For instance, "Sally read the article", "Joe listened to the song".
func ContentExperienceActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// GeoSocialEventsActivity processes matching activities
// The Geo-Social Events use case primarily deals with activities involving geo-tagging type activities. For instance,
// it can include activities such as "Joe arrived at work", "Sally left work", and "John is travel from home to work".
func GeoSocialEventsActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// NotificationActivity processes matching activities
// The Notification use case primarily deals with calling attention to particular objects or notifications.
func NotificationActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// QuestionActivity processes matching activities
// The Questions use case primarily deals with representing inquiries of any type. See 5.4
// Representing Questions for more information.
func QuestionActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// RelationshipManagementActivity processes matching activities
// The Relationship Management use case primarily deals with representing activities involving the management
// of interpersonal and social relationships (e.g. friend requests, management of social network, etc).
// See 5.2 Representing Relationships Between Entities for more information:
// https://www.w3.org/TR/activitystreams-vocabulary/#connections
func RelationshipManagementActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// OffersActivity processes matching activities
// The Offers use case deals with activities involving offering one object to another. It can include, for instance,
// activities such as "Company A is offering a discount on purchase of Product Z to Sally",
// "Sally is offering to add a File to Folder A", etc.
func OffersActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	// TODO(marius):
	return nil, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}
