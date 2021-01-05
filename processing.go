package processing

import (
	pub "github.com/go-ap/activitypub"
	c "github.com/go-ap/client"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
	"net"
	"time"
)

type _p struct {
	p *defaultProcessor
	v *defaultValidator
}

var emptyLogFn c.LogFn = func(s string, el ...interface{}) {}

type defaultProcessor struct {
	baseIRI pub.IRIs
	c       c.ActivityPub
	s       s.Saver
	infoFn  c.LogFn
	errFn   c.LogFn
}

func New(o ...optionFn) (*defaultProcessor, *defaultValidator, error) {
	v := &_p{
		p: &defaultProcessor{
			infoFn: emptyLogFn,
			errFn:  emptyLogFn,
		},
		v: &defaultValidator{
			addr:   ipCache{
				addr: make(map[string][]net.IP),
			},
			infoFn: emptyLogFn,
			errFn:  emptyLogFn,
		},
	}
	for _, fn := range o {
		if err := fn(v); err != nil {
			return v.p, v.v, err
		}
	}
	return v.p, v.v, nil
}

type optionFn func(s *_p) error

func SetInfoLogger(logFn c.LogFn) optionFn {
	return func(v *_p) error {
		v.v.infoFn = logFn
		v.p.infoFn = logFn
		return nil
	}
}

func SetErrorLogger(logFn c.LogFn) optionFn {
	return func(v *_p) error {
		v.v.errFn = logFn
		v.p.errFn = logFn
		return nil
	}
}

func SetClient(c c.ActivityPub) optionFn {
	return func(v *_p) error {
		v.v.c = c
		v.p.c = c
		return nil
	}
}

func SetStorage(s s.Repository) optionFn {
	return func(v *_p) error {
		v.v.s = s
		v.p.s = s
		return nil
	}
}

func SetIRI(i ...pub.IRI) optionFn {
	return func(v *_p) error {
		v.v.baseIRI = i
		v.p.baseIRI = i
		return nil
	}
}

// ProcessActivity
// TODO(marius): we need to create an Activity specific interface that we use as the type of the parameter, so we can
//   receive both Transitive and Intransitive activities. In the current form we can only process transitive ones.
func (p defaultProcessor) ProcessClientActivity(it pub.Item) (pub.Item, error) {
	if it == nil {
		return nil, errors.Newf("Unable to process nil activity")
	}
	if pub.IntransitiveActivityTypes.Contains(it.GetType()) {
		act, err := pub.ToIntransitiveActivity(it)
		if err != nil {
			return nil, err
		}
		if act == nil {
			return nil, errors.Newf("Unable to process nil intransitive activity")
		}

		return processIntransitiveActivity(p, act)
	}
	act, err := pub.ToActivity(it)
	if err != nil {
		return nil, err
	}
	if act == nil {
		return nil, errors.Newf("Unable to process nil intransitive activity")
	}

	return processActivity(p, act)
}

func processIntransitiveActivity(p defaultProcessor, act *pub.IntransitiveActivity) (*pub.IntransitiveActivity, error) {
	iri := act.GetLink()
	if len(iri) == 0 {
		p.s.GenerateID(act, nil)
	}
	var err error
	if pub.QuestionActivityTypes.Contains(act.Type) {
		act, err = QuestionActivity(p.s, act)
	} else if pub.GeoSocialEventsActivityTypes.Contains(act.Type) {
		act, err = GeoSocialEventsIntransitiveActivity(p.s, act)
	}
	if err != nil {
		return act, err
	}
	act = FlattenIntransitiveActivityProperties(act)
	if act.Published.IsZero() {
		act.Published = time.Now().UTC()
	}

	var it pub.Item
	it, err = p.s.SaveActivity(act)
	if err != nil {
		return act, err
	}
	if colSaver, ok := p.s.(s.CollectionSaver); ok {
		it, err = AddToCollections(colSaver, it)
	}
	return act, nil
}

func processActivity(p defaultProcessor, act *pub.Activity) (*pub.Activity, error) {
	iri := act.GetLink()
	if len(iri) == 0 {
		p.s.GenerateID(act, nil)
	}
	var err error

	if act.Object == nil {
		return act, errors.BadRequestf("Invalid %s: object is nil", act.Type)
	}

	obType := act.Object.GetType()
	// First we process the activity to effect whatever changes we need to on the activity properties.
	if pub.ContentManagementActivityTypes.Contains(act.Type) && obType != pub.RelationshipType {
		act, err = ContentManagementActivity(p.s, act, handlers.Outbox)
	} else if pub.CollectionManagementActivityTypes.Contains(act.Type) {
		act, err = CollectionManagementActivity(p.s, act)
	} else if pub.ReactionsActivityTypes.Contains(act.Type) {
		act, err = ReactionsActivity(p.s, act)
	} else if pub.EventRSVPActivityTypes.Contains(act.Type) {
		act, err = EventRSVPActivity(p.s, act)
	} else if pub.GroupManagementActivityTypes.Contains(act.Type) {
		act, err = GroupManagementActivity(p.s, act)
	} else if pub.ContentExperienceActivityTypes.Contains(act.Type) {
		act, err = ContentExperienceActivity(p.s, act)
	} else if pub.GeoSocialEventsActivityTypes.Contains(act.Type) {
		act, err = GeoSocialEventsActivity(p.s, act)
	} else if pub.NotificationActivityTypes.Contains(act.Type) {
		act, err = NotificationActivity(p.s, act)
	} else if pub.RelationshipManagementActivityTypes.Contains(act.Type) {
		act, err = RelationshipManagementActivity(p.s, act)
	} else if pub.NegatingActivityTypes.Contains(act.Type) {
		act, err = NegatingActivity(p.s, act)
	} else if pub.OffersActivityTypes.Contains(act.Type) {
		act, err = OffersActivity(p.s, act)
	}
	if err != nil {
		return act, err
	}
	act = FlattenActivityProperties(act)
	if act.Published.IsZero() {
		act.Published = time.Now().UTC()
	}

	var it pub.Item
	if act.Content != nil {
		// For activities that have a content value, we create the collections that allow actors to interact
		// with them as they are a regular object.
		pub.OnObject(act, addNewObjectCollections)
	}
	it, err = p.s.SaveActivity(act)
	if err != nil {
		return act, err
	}

	if colSaver, ok := p.s.(s.CollectionSaver); ok {
		it, err = AddToCollections(colSaver, it)
	}
	return act, nil
}

const blockedCollection = handlers.CollectionType("blocked")

func isBlocked(loader s.Loader, rec, act pub.Item) bool {
	// Check if any of the local recipients are blocking the actor
	blockedIRI := blockedCollection.IRI(rec)
	blockedAct, err := loader.LoadCollection(blockedIRI)
	return err == nil && blockedAct.Contains(act)
}

// AddToCollections handles the dissemination of the received it Activity to the local collections,
// it is addressed to:
//  - the author's Outbox
//  - the recipients' Inboxes
func AddToCollections(colSaver s.CollectionSaver, it pub.Item) (pub.Item, error) {
	act, err := pub.ToActivity(it)
	if err != nil {
		return nil, err
	}
	if act == nil {
		return nil, errors.Newf("Unable to process nil activity")
	}

	if act.Actor.GetLink() != pub.PublicNS && !act.GetLink().Contains(handlers.Outbox.IRI(act.Actor), false) {
		err = colSaver.AddToCollection(handlers.Outbox.IRI(act.Actor), act.GetLink())
		if err != nil {
			return act, err
		}
	}
	allRecipients := make(pub.ItemCollection, 0)
	for _, rec := range act.Recipients() {
		recIRI := rec.GetLink()
		if recIRI == pub.PublicNS {
			continue
		}
		if handlers.ValidCollectionIRI(recIRI) {
			// TODO(marius): this step should happen at validation time
			if loader, ok := colSaver.(s.Loader); ok {
				// Load all members if colIRI is a valid actor collection
				members, cnt, err := loader.LoadActors(recIRI)
				if err != nil || cnt == 0 {
					continue
				}
				for _, m := range members {
					if !pub.ActorTypes.Contains(m.GetType()) || isBlocked(loader, m, act.Actor) {
						continue
					}
					allRecipients = append(allRecipients, handlers.Inbox.IRI(m))
				}
			}
		} else {
			if loader, ok := colSaver.(s.Loader); ok {
				if isBlocked(loader, recIRI, act.Actor) {
					continue
				}
			}
			// TODO(marius): add check if IRI represents an actor (or rely on the collection saver to break if not)
			allRecipients = append(allRecipients, handlers.Inbox.IRI(recIRI))
		}
	}
	for _, recInb := range pub.ItemCollectionDeduplication(&allRecipients) {
		// TODO(marius): the processing module needs a method to see if an IRI is local or not
		//    For each recipient we need to save the incoming activity to the actor's Inbox if the actor is local
		//    Or disseminate it using S2S if the actor is not local
		colSaver.AddToCollection(recInb.GetLink(), act.GetLink())
	}
	return act, nil
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
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
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
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// GroupManagementActivity processes matching activities
// The Group Management use case primarily deals with management of groups.
// It can include, for instance, activities such as "John added Sally to Group A", "Sally joined Group A",
// "Joe left Group A", etc.
func GroupManagementActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// ContentExperienceActivity processes matching activities
// The Content Experience use case primarily deals with describing activities involving listening to,
// reading, or viewing content. For instance, "Sally read the article", "Joe listened to the song".
func ContentExperienceActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// GeoSocialEventsActivity processes matching activities
// The Geo-Social Events use case primarily deals with activities involving geo-tagging type activities. For instance,
// it can include activities such as "Joe arrived at work", "Sally left work", and "John is travel from home to work".
func GeoSocialEventsActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// GeoSocialEventsIntransitiveActivity processes matching activities
// The Geo-Social Events use case primarily deals with activities involving geo-tagging type activities. For instance,
// it can include activities such as "Joe arrived at work", "Sally left work", and "John is travel from home to work".
func GeoSocialEventsIntransitiveActivity(l s.Saver, act *pub.IntransitiveActivity) (*pub.IntransitiveActivity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// NotificationActivity processes matching activities
// The Notification use case primarily deals with calling attention to particular objects or notifications.
func NotificationActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// QuestionActivity processes matching activities
// The Questions use case primarily deals with representing inquiries of any type. See 5.4
// Representing Questions for more information.
func QuestionActivity(l s.Saver, act *pub.IntransitiveActivity) (*pub.IntransitiveActivity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// OffersActivity processes matching activities
// The Offers use case deals with activities involving offering one object to another. It can include, for instance,
// activities such as "Company A is offering a discount on purchase of Product Z to Sally",
// "Sally is offering to add a File to Folder A", etc.
func OffersActivity(l s.Saver, act *pub.Activity) (*pub.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}
