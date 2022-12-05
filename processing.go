package processing

import (
	"bytes"
	"crypto"
	"io"
	"net/http"
	"net/netip"
	"path"
	"sync"
	"time"

	"git.sr.ht/~mariusor/lw"
	vocab "github.com/go-ap/activitypub"
	c "github.com/go-ap/client"
	"github.com/go-ap/errors"
	"github.com/go-fed/httpsig"
	"github.com/openshift/osin"
)

var (
	emptyLogFn c.LogFn = func(s string, el ...interface{}) {}
	infoFn     c.LogFn = emptyLogFn
	errFn      c.LogFn = emptyLogFn
)

type P struct {
	baseIRI vocab.IRIs
	auth    *vocab.Actor
	c       c.Basic
	s       Store
}

func New(o ...optionFn) (*P, error) {
	p := new(P)
	for _, fn := range o {
		fn(p)
	}
	localAddressCache = ipCache{addr: make(map[string][]netip.Addr)}
	return p, nil
}

type optionFn func(s *P)

func WithIDGenerator(genFn IDGenerator) optionFn {
	new(sync.Once).Do(func() {
		createID = genFn
	})
	return func(_ *P) {}
}

func WithActorKeyGenerator(genFn vocab.WithActorFn) optionFn {
	new(sync.Once).Do(func() {
		createKey = genFn
	})
	return func(_ *P) {}
}

func WithInfoLogger(logFn c.LogFn) optionFn {
	new(sync.Once).Do(func() {
		infoFn = logFn
	})
	return func(_ *P) {}
}

func WithErrorLogger(logFn c.LogFn) optionFn {
	new(sync.Once).Do(func() {
		errFn = logFn
	})
	return func(_ *P) {}
}

func WithClient(c c.Basic) optionFn {
	return func(p *P) {
		p.c = c
	}
}

func WithStorage(s Store) optionFn {
	return func(p *P) {
		p.s = s
	}
}

func WithIRI(i ...vocab.IRI) optionFn {
	return func(p *P) {
		p.baseIRI = i
	}
}

func WithLocalIRIChecker(isLocalFn IRIValidator) optionFn {
	new(sync.Once).Do(func() {
		isLocalIRI = isLocalFn
	})
	return func(_ *P) {}
}

// ProcessActivity processes an Activity received
func (p P) ProcessActivity(it vocab.Item, receivedIn vocab.IRI) (vocab.Item, error) {
	if IsOutbox(receivedIn) {
		return p.ProcessClientActivity(it, receivedIn)
	}
	if IsInbox(receivedIn) {
		return p.ProcessServerActivity(it, receivedIn)
	}

	return nil, errors.MethodNotAllowedf("unable to process activities at current IRI: %s", receivedIn)
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

const OAuthOOBRedirectURN = "urn:ietf:wg:oauth:2.0:oob:auto"

var defaultSignFn c.RequestSignFn = func(*http.Request) error { return nil }

func genOAuth2Token(c osin.Storage, actor *vocab.Actor, cl vocab.Item) (string, error) {
	if actor == nil {
		return "", errors.Newf("invalid actor")
	}

	var client osin.Client
	if !vocab.IsNil(cl) {
		client, _ = c.GetClient(path.Base(cl.GetLink().String()))
	}
	if client == nil {
		client = &osin.DefaultClient{Id: "temp-client"}
	}
	now := time.Now().UTC()
	expiration := time.Hour * 24 * 14
	ad := &osin.AccessData{
		Client:      client,
		ExpiresIn:   int32(expiration.Seconds()),
		Scope:       "scope",
		RedirectUri: OAuthOOBRedirectURN,
		CreatedAt:   now,
		UserData:    actor.GetLink(),
	}

	// save access token
	if err := c.SaveAccess(ad); err != nil {
		return "", err
	}

	return ad.AccessToken, nil
}

func c2sSignFn(storage osin.Storage, act vocab.Item) func(r *http.Request) error {
	return func(req *http.Request) error {
		return vocab.OnActor(act, func(actor *vocab.Actor) error {
			tok, err := genOAuth2Token(storage, actor, nil)
			if len(tok) > 0 {
				req.Header.Set("Authorization", "Bearer "+tok)
			}
			return err
		})
	}
}

func s2sSignFn(keyLoader KeyLoader, actor vocab.Item) func(r *http.Request) error {
	key, err := keyLoader.LoadKey(actor.GetLink())
	if err != nil {
		return func(r *http.Request) error {
			return errors.Annotatef(err, "unable to load the actor's private key")
		}
	}
	if key == nil {
		return func(r *http.Request) error {
			return errors.Newf("invalid private key for actor")
		}
	}
	prefs := []httpsig.Algorithm{httpsig.ED25519, httpsig.RSA_SHA512, httpsig.RSA_SHA256}
	digestAlgorithm := httpsig.DigestSha256
	headersToSign := []string{httpsig.RequestTarget, "host", "date"}
	signer, _, err := httpsig.NewSigner(prefs, digestAlgorithm, headersToSign, httpsig.Signature, int64(time.Hour.Seconds()))
	if err != nil {
		return func(r *http.Request) error {
			return err
		}
	}
	keyId := actor.GetID() + "#main-key"
	return func(r *http.Request) error {
		bodyBuf := bytes.Buffer{}
		if r.Body != nil {
			if _, err := io.Copy(&bodyBuf, r.Body); err == nil {
				r.Body = io.NopCloser(&bodyBuf)
			}
		}
		return signer.SignRequest(key, keyId.String(), r, bodyBuf.Bytes())
	}
}

// BuildReplyToCollections builds the list of objects that it is inReplyTo
func (p P) BuildReplyToCollections(it vocab.Item) (vocab.ItemCollection, error) {
	ob, err := vocab.ToObject(it)
	if err != nil {
		return nil, err
	}
	collections := make(vocab.ItemCollection, 0)

	if ob.InReplyTo == nil {
		return nil, nil
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
		err = vocab.OnItemCollection(ob.InReplyTo, func(replyTos *vocab.ItemCollection) error {
			for _, replyTo := range replyTos.Collection() {
				collections = append(collections, vocab.Replies.IRI(replyTo.GetLink()))
			}
			return nil
		})
	}
	return collections, err
}

func loadSharedInboxRecipients(p P, sharedInbox vocab.IRI) vocab.ItemCollection {
	if len(p.baseIRI) == 0 {
		return nil
	}

	next := func(it vocab.Item) vocab.IRI {
		var next vocab.IRI
		switch it.GetType() {
		case vocab.CollectionPageType, vocab.OrderedCollectionPageType:
			vocab.OnCollectionPage(it, func(p *vocab.CollectionPage) error {
				next = p.Next.GetLink()
				return nil
			})
		case vocab.CollectionType, vocab.OrderedCollectionType:
			vocab.OnCollection(it, func(p *vocab.Collection) error {
				next = p.First.GetLink()
				return nil
			})
		}
		return next
	}
	// NOTE(marius): all of this is terrible, as it relies on FedBOX discoverability of actors
	//  It also doesn't iterate through the whole collection but only through the first page of results
	iri := p.baseIRI[0].AddPath("actors?maxItems=200")

	actors := make(vocab.ItemCollection, 0)
	for {
		col, err := p.s.Load(iri)
		if err != nil {
			errFn("unable to load actors for sharedInbox check: %s", err)
			break
		}
		vocab.OnCollectionIntf(col, func(col vocab.CollectionInterface) error {
			for _, act := range col.Collection() {
				vocab.OnActor(act, func(act *vocab.Actor) error {
					if act.Endpoints != nil {
						if sharedInbox.Equals(act.Endpoints.SharedInbox.GetLink(), false) && !actors.Contains(act.GetLink()) {
							actors = append(actors, act)
						}
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
	return actors
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
