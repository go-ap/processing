package processing

import (
	"crypto"
	"sync"
	"time"

	"git.sr.ht/~mariusor/lw"
	vocab "github.com/go-ap/activitypub"
	c "github.com/go-ap/client"
	"github.com/go-ap/errors"
)

type P struct {
	baseIRI vocab.IRIs
	async   bool
	c       c.Basic
	s       Store
	l       lw.Logger
}

var (
	nilLogger = lw.Nil()
)

func New(o ...OptionFn) P {
	p := P{l: nilLogger}
	for _, fn := range o {
		fn(&p)
	}
	localAddressCache = ipCache{addr: sync.Map{}}
	return p
}

type OptionFn func(s *P)

func Async(p *P) {
	p.async = true
}

func WithIDGenerator(genFn IDGenerator) OptionFn {
	new(sync.Once).Do(func() {
		createID = genFn
	})
	return func(_ *P) {}
}

func WithActorKeyGenerator(genFn vocab.WithActorFn) OptionFn {
	new(sync.Once).Do(func() {
		createKey = genFn
	})
	return func(_ *P) {}
}

func WithLogger(l lw.Logger) OptionFn {
	return func(p *P) {
		p.l = l
	}
}

func WithClient(c c.Basic) OptionFn {
	return func(p *P) {
		p.c = c
	}
}

func WithStorage(s Store) OptionFn {
	return func(p *P) {
		p.s = s
	}
}

func WithIRI(i ...vocab.IRI) OptionFn {
	return func(p *P) {
		p.baseIRI = i
	}
}

func WithLocalIRIChecker(isLocalFn IRIValidator) OptionFn {
	new(sync.Once).Do(func() {
		isLocalIRI = isLocalFn
	})
	return func(_ *P) {}
}

// ProcessActivity processes an Activity received
func (p P) ProcessActivity(it vocab.Item, author vocab.Actor, receivedIn vocab.IRI) (vocab.Item, error) {
	if vocab.IsNil(it) {
		return nil, errors.BadRequestf("nil activity received")
	}
	p.l = p.l.WithContext(lw.Ctx{"in": receivedIn, "type": it.GetType()})
	p.l.Debugf("Processing started")
	defer func(start time.Time) {
		p.l.WithContext(lw.Ctx{"duration": time.Now().Sub(start)}).Debugf("Processing ended")
	}(time.Now())

	if IsOutbox(receivedIn) {
		return p.ProcessClientActivity(it, author, receivedIn)
	}
	if IsInbox(receivedIn) {
		return p.ProcessServerActivity(it, author, receivedIn)
	}

	return nil, errors.MethodNotAllowedf("unable to process activities at current IRI: %s", receivedIn)
}

func createNewTags(l WriteStore, tags vocab.ItemCollection, parent vocab.Item) error {
	if len(tags) == 0 {
		return nil
	}
	// According to the example in the Implementation Notes on the Activity Streams Vocabulary spec,
	// tag objects are ActivityStreams Objects without a type, that's why we use an empty string valid type:
	// https://www.w3.org/TR/activitystreams-vocabulary/#microsyntaxes
	validTagTypes := vocab.ActivityVocabularyTypes{vocab.MentionType, vocab.ObjectType, vocab.ActivityVocabularyType("")}
	for _, tag := range tags {
		if typ := tag.GetType(); !validTagTypes.Contains(typ) {
			continue
		}
		if id := tag.GetID(); len(id) > 0 {
			continue
		}
		if err := SetIDIfMissing(tag, nil, parent); err == nil {
			l.Save(tag)
		}
	}
	return nil
}

func isBlocked(loader ReadStore, rec, act vocab.Item) bool {
	// Check if any of the local recipients are blocking the actor, we assume rec is local
	blockedIRI := BlockedCollection.IRI(rec)
	blockedAct, err := loader.Load(blockedIRI)
	if err != nil || vocab.IsNil(blockedAct) {
		return false
	}
	blocked := false
	_ = vocab.OnCollectionIntf(blockedAct, func(c vocab.CollectionInterface) error {
		blocked = c.Contains(act)
		return nil
	})
	return blocked
}

type KeyLoader interface {
	LoadKey(vocab.IRI) (crypto.PrivateKey, error)
}

const OAuthOOBRedirectURN = "urn:ietf:wg:oauth:2.0:oob:auto"

// BuildReplyToCollections builds the list of objects that it is inReplyTo
func (p P) BuildReplyToCollections(it vocab.Item) vocab.ItemCollection {
	ob, err := vocab.ToObject(it)
	if err != nil {
		return nil
	}
	collections := make(vocab.ItemCollection, 0)

	if ob.InReplyTo == nil {
		return nil
	}
	if vocab.IsIRI(ob.InReplyTo) {
		collections = append(collections, vocab.Replies.IRI(ob.InReplyTo.GetLink()))
	}
	if vocab.IsObject(ob.InReplyTo) {
		err = vocab.OnObject(ob.InReplyTo, func(replyTo *vocab.Object) error {
			collections = append(collections, vocab.Replies.IRI(replyTo.GetLink()))
			return nil
		})
	}
	if vocab.IsItemCollection(ob.InReplyTo) {
		_ = vocab.OnItemCollection(ob.InReplyTo, func(replyTos *vocab.ItemCollection) error {
			for _, replyTo := range replyTos.Collection() {
				collections = append(collections, vocab.Replies.IRI(replyTo.GetLink()))
			}
			return nil
		})
	}
	return collections
}

func loadSharedInboxRecipients(p P, sharedInbox vocab.IRI) vocab.ItemCollection {
	if len(p.baseIRI) == 0 {
		return nil
	}

	next := func(it vocab.Item) vocab.IRI {
		var next vocab.IRI
		switch it.GetType() {
		case vocab.CollectionPageType, vocab.OrderedCollectionPageType:
			_ = vocab.OnCollectionPage(it, func(p *vocab.CollectionPage) error {
				if p.Next != nil {
					next = p.Next.GetLink()
				}
				return nil
			})
		case vocab.CollectionType, vocab.OrderedCollectionType:
			_ = vocab.OnCollection(it, func(p *vocab.Collection) error {
				if p.First != nil {
					next = p.First.GetLink()
				}
				return nil
			})
		}
		return next
	}

	actors := make(vocab.ItemCollection, 0)
	for _, us := range p.baseIRI {
		if !sharedInbox.Contains(us, true) {
			continue
		}
		// NOTE(marius): all of this is terrible, as it relies on FedBOX discoverability of actors
		//  It also doesn't iterate through the whole collection but only through the first page of results
		iri := vocab.CollectionPath("actors").Of(us).GetLink()
		for {
			col, err := p.s.Load(iri)
			if err != nil {
				p.l.Warnf("unable to load actors for sharedInbox check: %+s", err)
				break
			}
			_ = vocab.OnCollectionIntf(col, func(col vocab.CollectionInterface) error {
				for _, act := range col.Collection() {
					_ = vocab.OnActor(act, func(act *vocab.Actor) error {
						if act.Endpoints == nil || act.Endpoints.SharedInbox == nil {
							return nil
						}
						if sharedInbox.Equals(act.Endpoints.SharedInbox.GetLink(), false) && !actors.Contains(act.GetLink()) {
							_ = actors.Append(actors)
						}
						return nil
					})
				}
				return nil
			})
			if iri = next(col); iri == "" {
				break
			}
		}
	}
	return actors
}

// CollectionManagementActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-collections
//
// The Collection Management use case primarily deals with activities involving the management of content within collections.
// Examples of collections include things like folders, albums, friend lists, etc.
// This includes, for instance, activities such as "Sally added a file to Folder A",
// "John moved the file from Folder A to Folder B", etc.
func CollectionManagementActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(act.Object) {
		return act, errors.NotValidf("Missing object for Activity")
	}
	switch act.Type {
	case vocab.AddType:
	case vocab.MoveType:
	case vocab.RemoveType:
	default:
		return nil, errors.NotValidf("Invalid type %s", act.GetType())
	}
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// EventRSVPActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-rsvp
//
// The Event RSVP use case primarily deals with invitations to events and RSVP type responses.
func EventRSVPActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	if vocab.IsNil(act.Object) {
		return act, errors.NotValidf("Missing object for Activity")
	}
	switch act.Type {
	case vocab.AcceptType:
	case vocab.IgnoreType:
	case vocab.InviteType:
	case vocab.RejectType:
	case vocab.TentativeAcceptType:
	case vocab.TentativeRejectType:
	default:
		return nil, errors.NotValidf("Invalid type %s", act.GetType())
	}
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// GroupManagementActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-group
//
// The Group Management use case primarily deals with management of groups.
// It can include, for instance, activities such as "John added Sally to Group A", "Sally joined Group A",
// "Joe left Group A", etc.
func GroupManagementActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// ContentExperienceActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-experience
//
// The Content Experience use case primarily deals with describing activities involving listening to,
// reading, or viewing content. For instance, "Sally read the article", "Joe listened to the song".
func ContentExperienceActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// GeoSocialEventsActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-geo
//
// The Geo-Social Events use case primarily deals with activities involving geo-tagging type activities. For instance,
// it can include activities such as "Joe arrived at work", "Sally left work", and "John is travel from home to work".
func GeoSocialEventsActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// GeoSocialEventsIntransitiveActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-geo
//
// The Geo-Social Events use case primarily deals with activities involving geo-tagging type activities. For instance,
// it can include activities such as "Joe arrived at work", "Sally left work", and "John is travel from home to work".
func GeoSocialEventsIntransitiveActivity(l WriteStore, act *vocab.IntransitiveActivity) (*vocab.IntransitiveActivity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// NotificationActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-notification
//
// The Notification use case primarily deals with calling attention to particular objects or notifications.
func NotificationActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// OffersActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-offer
//
// The Offers use case deals with activities involving offering one object to another. It can include, for instance,
// activities such as "Company A is offering a discount on purchase of Product Z to Sally",
// "Sally is offering to add a File to Folder A", etc.
func OffersActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}
