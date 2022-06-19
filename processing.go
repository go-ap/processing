package processing

import (
	"crypto"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/netip"

	vocab "github.com/go-ap/activitypub"
	c "github.com/go-ap/client"
	"github.com/go-ap/errors"
	"github.com/go-ap/httpsig"
	"golang.org/x/crypto/ed25519"
)

type Validator interface {
	ValidateClientActivity(vocab.Item, vocab.IRI) error
}

type _p struct {
	p *defaultProcessor
	v *defaultValidator
}

var emptyLogFn c.LogFn = func(s string, el ...interface{}) {}

type defaultProcessor struct {
	baseIRI vocab.IRIs
	v       *defaultValidator
	c       c.Basic
	s       WriteStore
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
			addr:   ipCache{addr: make(map[string][]netip.Addr)},
			infoFn: emptyLogFn,
			errFn:  emptyLogFn,
		},
	}
	for _, fn := range o {
		if err := fn(v); err != nil {
			return v.p, v.v, err
		}
	}
	v.p.v = v.v
	return v.p, v.v, nil
}

type optionFn func(s *_p) error

func SetIDGenerator(genFn IDGenerator) optionFn {
	createID = genFn
	return func(v *_p) error { return nil }
}

func SetActorKeyGenerator(genFn vocab.WithActorFn) optionFn {
	createKey = genFn
	return func(_ *_p) error { return nil }
}

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

func SetClient(c c.Basic) optionFn {
	return func(v *_p) error {
		v.v.c = c
		v.p.c = c
		return nil
	}
}

func SetStorage(s Store) optionFn {
	return func(v *_p) error {
		v.v.s = s
		v.p.s = s
		return nil
	}
}

func SetIRI(i ...vocab.IRI) optionFn {
	return func(v *_p) error {
		v.v.baseIRI = i
		v.p.baseIRI = i
		return nil
	}
}

func createNewTags(l WriteStore, tags vocab.ItemCollection) error {
	if len(tags) == 0 {
		return nil
	}
	// According to the example in the Implementation Notes on the Activity Streams Vocabulary spec,
	// tag objects are ActivityStreams Objects without a type, that's why we use an empty string valid type:
	// https://www.w3.org/TR/activitystreams-vocabulary/#microsyntaxes
	validTypes := vocab.ActivityVocabularyTypes{vocab.MentionType, vocab.ObjectType, vocab.ActivityVocabularyType("")}
	for _, tag := range tags {
		if typ := tag.GetType(); !validTypes.Contains(typ) {
			continue
		}
		if id := tag.GetID(); len(id) > 0 {
			continue
		}
		if err := SetID(tag, nil, nil); err == nil {
			l.Save(tag)
		}
	}
	return nil
}

func isBlocked(loader ReadStore, rec, act vocab.Item) bool {
	// Check if any of the local recipients are blocking the actor, we assume rec is local
	blockedIRI := BlockedCollection.IRI(rec)
	blockedAct, err := loader.Load(blockedIRI)
	if err != nil {
		return false
	}
	blocked := false
	if blockedAct.IsCollection() {
		vocab.OnCollectionIntf(blockedAct, func(c vocab.CollectionInterface) error {
			blocked = c.Contains(act)
			return nil
		})
	}
	return blocked
}

type KeyLoader interface {
	LoadKey(vocab.IRI) (crypto.PrivateKey, error)
}

type KeySaver interface {
	GenKey(vocab.IRI) error
}

var defaultSignFn c.RequestSignFn = func(*http.Request) error { return nil }

func s2sSignFn(keyLoader KeyLoader, actor vocab.Item) func(r *http.Request) error {
	return func(r *http.Request) error {
		key, err := keyLoader.LoadKey(actor.GetLink())
		if err != nil {
			return err
		}
		typ, err := keyType(key)
		if err != nil {
			return err
		}

		signHdrs := []string{"(request-target)", "host", "date"}
		keyId := fmt.Sprintf("%s#main-key", actor.GetID())
		return httpsig.NewSigner(keyId, key, typ, signHdrs).Sign(r)
	}
}

func keyType(key crypto.PrivateKey) (httpsig.Algorithm, error) {
	switch key.(type) {
	case *rsa.PrivateKey:
		return httpsig.RSASHA256, nil
	case ed25519.PrivateKey:
		return httpsig.Ed25519, nil
	}
	return nil, errors.Errorf("Unknown private key type[%T] %v", key, key)
}

// AddToCollections handles the dissemination of the received it Activity to the local collections,
// it is addressed to:
//  - the author's Outbox - if the author is local
//  - the recipients' Inboxes - if they are local
func AddToCollections(p defaultProcessor, colSaver CollectionStore, it vocab.Item) (vocab.Item, error) {
	act, err := vocab.ToActivity(it)
	if err != nil {
		return nil, err
	}
	if act == nil {
		return nil, errors.Newf("Unable to process nil activity")
	}

	allRecipients := make(vocab.ItemCollection, 0)
	if act.Actor != nil {
		actIRI := act.Actor.GetLink()
		outbox := vocab.Outbox.IRI(actIRI)

		if !actIRI.Equals(vocab.PublicNS, true) && !act.GetLink().Contains(outbox, false) && p.v.IsLocalIRI(actIRI) {
			allRecipients = append(allRecipients, outbox)
		}
	}

	for _, rec := range act.Recipients() {
		recIRI := rec.GetLink()
		if recIRI == vocab.PublicNS {
			// NOTE(marius): if the activity is addressed to the Public NS, we store it to the local service's inbox
			if len(p.baseIRI) > 0 {
				allRecipients = append(allRecipients, vocab.Inbox.IRI(p.baseIRI[0]))
			}
			continue
		}
		if vocab.ValidCollectionIRI(recIRI) {
			// TODO(marius): this step should happen at validation time
			if loader, ok := colSaver.(ReadStore); ok {
				// Load all members if colIRI is a valid actor collection
				members, err := loader.Load(recIRI)
				if err != nil || vocab.IsNil(members) {
					continue
				}
				vocab.OnCollectionIntf(members, func(col vocab.CollectionInterface) error {
					for _, m := range col.Collection() {
						if !vocab.ActorTypes.Contains(m.GetType()) || (p.v.IsLocalIRI(m.GetLink()) && isBlocked(loader, m, act.Actor)) {
							continue
						}
						allRecipients = append(allRecipients, vocab.Inbox.IRI(m))
					}
					return nil
				})
			}
		} else {
			if loader, ok := colSaver.(ReadStore); ok {
				if p.v.IsLocalIRI(recIRI) && isBlocked(loader, recIRI, act.Actor) {
					continue
				}
			}
			inb := vocab.Inbox.IRI(recIRI)
			if !allRecipients.Contains(inb) {
				// TODO(marius): add check if IRI represents an actor (or rely on the collection saver to break if not)
				allRecipients = append(allRecipients, inb)
			}
		}
	}
	return disseminateToCollections(p, act, allRecipients)
}

func disseminateToCollections(p defaultProcessor, act *vocab.Activity, allRecipients vocab.ItemCollection) (*vocab.Activity, error) {
	for _, recInb := range vocab.ItemCollectionDeduplication(&allRecipients) {
		if err := disseminateToCollection(p, recInb.GetLink(), act); err != nil {
			p.errFn("Failed: %s", err.Error())
		}
	}
	return act, nil
}

func disseminateToCollection(p defaultProcessor, col vocab.IRI, act *vocab.Activity) error {
	colSaver, ok := p.s.(CollectionStore)
	if !ok {
		// TODO(marius): not returning an error might be the wrong move here
		return nil
	}
	// TODO(marius): the processing module needs a method to see if an IRI is local or not
	//    For each recipient we need to save the incoming activity to the actor's Inbox if the actor is local
	//    Or disseminate it using S2S if the actor is not local
	if p.v.IsLocalIRI(col) {
		p.infoFn("Saving to local actor's collection %s", col)
		if err := colSaver.AddTo(col, act.GetLink()); err != nil {
			return err
		}
	} else if p.v.IsLocalIRI(act.ID) {
		keyLoader, ok := p.s.(KeyLoader)
		if !ok {
			return nil
		}
		// TODO(marius): Move this function to either the go-ap/auth package, or in FedBOX itself.
		//   We should probably change the signature for client.RequestSignFn to accept an Actor/IRI as a param.
		p.c.SignFn(s2sSignFn(keyLoader, act.Actor))
		p.infoFn("Pushing to remote actor's collection %s", col)
		if _, _, err := p.c.ToCollection(col, act); err != nil {
			return err
		}
	}
	return nil
}

// CollectionManagementActivity processes matching activities
// The Collection Management use case primarily deals with activities involving the management of content within collections.
// Examples of collections include things like folders, albums, friend lists, etc.
// This includes, for instance, activities such as "Sally added a file to Folder A",
// "John moved the file from Folder A to Folder B", etc.
func CollectionManagementActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	if act.Object == nil {
		return act, errors.NotValidf("Missing object for Activity")
	}
	if act.Target == nil {
		return act, errors.NotValidf("Missing target collection for Activity")
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
// The Event RSVP use case primarily deals with invitations to events and RSVP type responses.
func EventRSVPActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	if act.Object == nil {
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
// The Group Management use case primarily deals with management of groups.
// It can include, for instance, activities such as "John added Sally to Group A", "Sally joined Group A",
// "Joe left Group A", etc.
func GroupManagementActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// ContentExperienceActivity processes matching activities
// The Content Experience use case primarily deals with describing activities involving listening to,
// reading, or viewing content. For instance, "Sally read the article", "Joe listened to the song".
func ContentExperienceActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// GeoSocialEventsActivity processes matching activities
// The Geo-Social Events use case primarily deals with activities involving geo-tagging type activities. For instance,
// it can include activities such as "Joe arrived at work", "Sally left work", and "John is travel from home to work".
func GeoSocialEventsActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// GeoSocialEventsIntransitiveActivity processes matching activities
// The Geo-Social Events use case primarily deals with activities involving geo-tagging type activities. For instance,
// it can include activities such as "Joe arrived at work", "Sally left work", and "John is travel from home to work".
func GeoSocialEventsIntransitiveActivity(l WriteStore, act *vocab.IntransitiveActivity) (*vocab.IntransitiveActivity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// NotificationActivity processes matching activities
// The Notification use case primarily deals with calling attention to particular objects or notifications.
func NotificationActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}

// OffersActivity processes matching activities
//
// The Offers use case deals with activities involving offering one object to another. It can include, for instance,
// activities such as "Company A is offering a discount on purchase of Product Z to Sally",
// "Sally is offering to add a File to Folder A", etc.
func OffersActivity(l WriteStore, act *vocab.Activity) (*vocab.Activity, error) {
	// TODO(marius):
	return act, errors.NotImplementedf("Processing %s activity is not implemented", act.GetType())
}
