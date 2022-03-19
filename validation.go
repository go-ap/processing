package processing

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"path"
	"strings"
	"sync"

	pub "github.com/go-ap/activitypub"
	c "github.com/go-ap/client"
	"github.com/go-ap/errors"
	"github.com/go-ap/handlers"
	s "github.com/go-ap/storage"
)

type ClientActivityValidator interface {
	ValidateClientActivity(pub.Item, pub.IRI) error
	//ValidateClientObject(pub.Item) error
	ValidateClientActor(pub.Item) error
	//ValidateClientTarget(pub.Item) error
	//ValidateClientAudience(...pub.ItemCollection) error
}

type ServerActivityValidator interface {
	ValidateServerActivity(pub.Item, pub.IRI) error
	//ValidateServerObject(pub.Item) error
	ValidateServerActor(pub.Item) error
	//ValidateServerTarget(pub.Item) error
	//ValidateServerAudience(...pub.ItemCollection) error
}

// ActivityValidator is an interface used for validating activity objects.
type ActivityValidator interface {
	ClientActivityValidator
	ServerActivityValidator
}

//type AudienceValidator interface {
//	ValidateAudience(...pub.ItemCollection) error
//}

// ObjectValidator is an interface used for validating generic objects
type ObjectValidator interface {
	ValidateObject(pub.Item) error
}

// ActorValidator is an interface used for validating actor objects
type ActorValidator interface {
	ValidActor(pub.Item) error
}

// TargetValidator is an interface used for validating an object that is an activity's target
// TODO(marius): this seems to have a different semantic than the previous ones.
//  Ie, any object can be a target, but in the previous cases, the main validation mechanism is based on the Type.
//type TargetValidator interface {
//	ValidTarget(pub.Item) error
//}

type invalidActivity struct {
	errors.Err
}

type ipCache struct {
	addr map[string][]netip.Addr
	m    sync.RWMutex
}

type defaultValidator struct {
	baseIRI pub.IRIs
	addr    ipCache
	auth    *pub.Actor
	c       c.Basic
	s       s.ReadStore
	infoFn  c.LogFn
	errFn   c.LogFn
}

type ActivityPubError struct {
	errors.Err
}

type MissingActorError struct {
	errors.Err
}

var InvalidActivity = func(s string, p ...interface{}) ActivityPubError {
	return ActivityPubError{wrapErr(nil, fmt.Sprintf("Activity is not valid: %s", s), p...)}
}
var MissingActivityActor = func(s string, p ...interface{}) MissingActorError {
	return MissingActorError{wrapErr(nil, fmt.Sprintf("Missing actor %s", s), p...)}
}
var InvalidActivityActor = func(s string, p ...interface{}) ActivityPubError {
	return ActivityPubError{wrapErr(nil, fmt.Sprintf("Actor is not valid: %s", s), p...)}
}
var InvalidActivityObject = func(s string, p ...interface{}) errors.Err {
	return wrapErr(nil, fmt.Sprintf("Object is not valid: %s", s), p...)
}
var InvalidIRI = func(s string, p ...interface{}) errors.Err {
	return wrapErr(nil, fmt.Sprintf("IRI is not valid: %s", s), p...)
}
var InvalidTarget = func(s string, p ...interface{}) ActivityPubError {
	return ActivityPubError{wrapErr(nil, fmt.Sprintf("Target is not valid: %s", s), p...)}
}

func (m *MissingActorError) Is(e error) bool {
	_, okp := e.(*MissingActorError)
	_, oks := e.(MissingActorError)
	return okp || oks
}

func (v defaultValidator) ValidateServerActivity(a pub.Item, inbox pub.IRI) error {
	if !IsInbox(inbox) {
		return errors.NotValidf("Trying to validate a non inbox IRI %s", inbox)
	}
	//if v.auth.GetLink() == pub.PublicNS {
	//	return errors.Unauthorizedf("%s actor is not allowed posting to current inbox", v.auth.Name)
	//}
	if a == nil {
		return InvalidActivityActor("received nil activity")
	}
	if a.IsLink() {
		return v.ValidateLink(a.GetLink())
	}
	if !pub.ActivityTypes.Contains(a.GetType()) {
		return InvalidActivity("invalid type %s", a.GetType())
	}
	return pub.OnActivity(a, func(act *pub.Activity) error {
		if len(act.ID) == 0 {
			return InvalidActivity("invalid activity id %s", act.ID)
		}

		inboxBelongsTo, err := handlers.Inbox.OfActor(inbox)
		if err != nil {
			return err
		}
		if isBlocked(v.s, inboxBelongsTo, act.Actor) {
			return errors.NotFoundf("")
		}
		if act.Actor, err = v.ValidateServerActor(act.Actor); err != nil {
			if missingActor.Is(err) && v.auth != nil {
				act.Actor = v.auth
			} else {
				return err
			}
		}
		if act.Object, err = v.ValidateServerObject(act.Object); err != nil {
			return err
		}
		if act.Target != nil {
			if act.Target, err = v.ValidateServerObject(act.Target); err != nil {
				return err
			}
		}
		return nil
	})
}

func IsOutbox(i pub.IRI) bool {
	return strings.ToLower(path.Base(i.String())) == strings.ToLower(string(handlers.Outbox))
}

func IsInbox(i pub.IRI) bool {
	return strings.ToLower(path.Base(i.String())) == strings.ToLower(string(handlers.Inbox))
}

// IRIBelongsToActor checks if the search iri represents any of the collections associated with the actor.
func IRIBelongsToActor(iri pub.IRI, actor *pub.Actor) bool {
	if actor == nil {
		return false
	}
	if handlers.Inbox.IRI(actor).Equals(iri, false) {
		return true
	}
	if handlers.Outbox.IRI(actor).Equals(iri, false) {
		return true
	}
	// The following should not really come into question at any point.
	// This function should be used for checking inbox/outbox/sharedInbox IRIS
	if handlers.Following.IRI(actor).Equals(iri, false) {
		return true
	}
	if handlers.Followers.IRI(actor).Equals(iri, false) {
		return true
	}
	if handlers.Replies.IRI(actor).Equals(iri, false) {
		return true
	}
	if handlers.Liked.IRI(actor).Equals(iri, false) {
		return true
	}
	if handlers.Shares.IRI(actor).Equals(iri, false) {
		return true
	}
	if handlers.Likes.IRI(actor).Equals(iri, false) {
		return true
	}
	if actor.Endpoints != nil && actor.Endpoints.SharedInbox.GetLink().Equals(iri, false) {
		return true
	}
	return false
}

var missingActor = new(MissingActorError)

func name(a *pub.Actor) pub.LangRefValue {
	if len(a.Name) > 0 {
		return a.Name.First()
	}
	if len(a.PreferredUsername) > 0 {
		return a.PreferredUsername.First()
	}
	return pub.LangRefValue{Value: pub.Content(path.Base(string(a.ID)))}
}

func (v defaultValidator) ValidateClientActivity(a pub.Item, outbox pub.IRI) error {
	if !IsOutbox(outbox) {
		return errors.NotValidf("Trying to validate a non outbox IRI %s", outbox)
	}
	if v.auth == nil || v.auth.GetLink() == pub.PublicNS {
		return errors.Unauthorizedf("%s actor is not allowed posting to current outbox", name(v.auth))
	}
	if !IRIBelongsToActor(outbox, v.auth) {
		return errors.Unauthorizedf("%s actor does not own the current outbox", name(v.auth))
	}
	if a == nil {
		return InvalidActivityActor("received nil activity")
	}
	if a.IsLink() {
		return v.ValidateLink(a.GetLink())
	}
	validActivityTypes := append(pub.ActivityTypes, pub.IntransitiveActivityTypes...)
	if !validActivityTypes.Contains(a.GetType()) {
		return InvalidActivity("invalid type %s", a.GetType())
	}
	return pub.OnActivity(a, func(act *pub.Activity) error {
		var err error
		if act.Actor, err = v.ValidateClientActor(act.Actor); err != nil {
			if missingActor.Is(err) && v.auth != nil {
				act.Actor = v.auth
			} else {
				return err
			}
		}
		if !pub.IntransitiveActivityTypes.Contains(a.GetType()) {
			// @TODO(marius): this needs to be extended by a ValidateActivityClientObject
			//   because the first step would be to test the object in the context of the activity
			//   The ValidateActivityClientObject could then validate just the object itself.
			if act.Object, err = v.ValidateClientObject(act.Object); err != nil {
				return err
			}
		}
		if act.Target != nil {
			if act.Target, err = v.ValidateClientObject(act.Target); err != nil {
				return err
			}
		}
		if pub.ContentManagementActivityTypes.Contains(act.GetType()) && act.Object.GetType() != pub.RelationshipType {
			err = ValidateClientContentManagementActivity(v.s, act)
		} else if pub.CollectionManagementActivityTypes.Contains(act.GetType()) {
			err = ValidateClientCollectionManagementActivity(v.s, act)
		} else if pub.ReactionsActivityTypes.Contains(act.GetType()) {
			err = ValidateClientReactionsActivity(v.s, act)
		} else if pub.EventRSVPActivityTypes.Contains(act.GetType()) {
			err = ValidateClientEventRSVPActivity(v.s, act)
		} else if pub.GroupManagementActivityTypes.Contains(act.GetType()) {
			err = ValidateClientGroupManagementActivity(v.s, act)
		} else if pub.ContentExperienceActivityTypes.Contains(act.GetType()) {
			err = ValidateClientContentExperienceActivity(v.s, act)
		} else if pub.GeoSocialEventsActivityTypes.Contains(act.GetType()) {
			err = ValidateClientGeoSocialEventsActivity(v.s, act)
		} else if pub.NotificationActivityTypes.Contains(act.GetType()) {
			err = ValidateClientNotificationActivity(v.s, act)
		} else if pub.QuestionActivityTypes.Contains(act.GetType()) {
			err = ValidateClientQuestionActivity(v.s, act)
		} else if pub.RelationshipManagementActivityTypes.Contains(act.GetType()) {
			err = ValidateClientRelationshipManagementActivity(v.s, act)
		} else if pub.NegatingActivityTypes.Contains(act.GetType()) {
			err = ValidateClientNegatingActivity(v.s, act)
		} else if pub.OffersActivityTypes.Contains(act.GetType()) {
			err = ValidateClientOffersActivity(v.s, act)
		}
		return err
	})
}

// ValidateClientContentManagementActivity
func ValidateClientContentManagementActivity(l s.ReadStore, act *pub.Activity) error {
	if act.Object == nil {
		return errors.NotValidf("nil object for %s activity", act.Type)
	}
	ob := act.Object
	switch act.Type {
	case pub.UpdateType:
		if pub.ActivityTypes.Contains(ob.GetType()) {
			return errors.Newf("trying to update an immutable activity")
		}
		fallthrough
	case pub.DeleteType:
		if len(ob.GetLink()) == 0 {
			return errors.Newf("invalid object id for %s activity", act.Type)
		}
		if ob.IsLink() {
			return nil
		}
		var (
			found pub.Item
			err   error
		)

		found, err = l.Load(ob.GetLink())
		if err != nil {
			return errors.Annotatef(err, "failed to load object from storage")
		}
		if found == nil {
			return errors.NotFoundf("found nil object in storage")
		}
	case pub.CreateType:
	default:
	}

	return nil
}

// ValidateClientCollectionManagementActivity
func ValidateClientCollectionManagementActivity(l s.ReadStore, act *pub.Activity) error {
	return nil
}

// ValidateClientReactionsActivity
func ValidateClientReactionsActivity(l s.ReadStore, act *pub.Activity) error {
	return nil
}

// ValidateClientEventRSVPActivity
func ValidateClientEventRSVPActivity(l s.ReadStore, act *pub.Activity) error {
	return nil
}

// ValidateClientGroupManagementActivity
func ValidateClientGroupManagementActivity(l s.ReadStore, act *pub.Activity) error {
	return nil
}

// ValidateClientContentExperienceActivity
func ValidateClientContentExperienceActivity(l s.ReadStore, act *pub.Activity) error {
	return nil
}

// ValidateClientGeoSocialEventsActivity
func ValidateClientGeoSocialEventsActivity(l s.ReadStore, act *pub.Activity) error {
	return nil
}

// ValidateClientNotificationActivity
func ValidateClientNotificationActivity(l s.ReadStore, act *pub.Activity) error {
	return nil
}

// ValidateClientQuestionActivity
func ValidateClientQuestionActivity(l s.ReadStore, act *pub.Activity) error {
	return nil
}

// ValidateClientRelationshipManagementActivity
func ValidateClientRelationshipManagementActivity(l s.ReadStore, act *pub.Activity) error {
	switch act.Type {
	case pub.FollowType:
		a, _ := l.Load(act.GetLink())
		if !pub.IsNil(firstOrItem(a)) {
			return errors.Newf("%s already exists for this actor/object pair", act.Type)
		}
	case pub.AddType:
	case pub.BlockType:
	case pub.CreateType:
	case pub.DeleteType:
	case pub.IgnoreType:
	case pub.InviteType:
	case pub.AcceptType:
		fallthrough
	case pub.RejectType:
		// TODO(marius): either the actor or the object needs to be local for this action to be valid
		//   in the case of C2S... the actor needs to be local
		//   in the case of S2S... the object needs to be local
		// TODO(marius): Object needs to be a valid Follow activity
	default:
	}
	return nil
}

// ValidateClientNegatingActivity
func ValidateClientNegatingActivity(l s.ReadStore, act *pub.Activity) error {
	return nil
}

// ValidateClientOffersActivity
func ValidateClientOffersActivity(l s.ReadStore, act *pub.Activity) error {
	return nil
}

// IsLocalIRI shows if the received IRI belongs to the current instance
func (v defaultValidator) IsLocalIRI(i pub.IRI) bool {
	return v.validateLocalIRI(i) == nil
}

func (v defaultValidator) ValidateLink(i pub.IRI) error {
	if i.Equals(pub.PublicNS, false) {
		return InvalidIRI("Public namespace is not a local IRI")
	}
	var loadFn func(pub.IRI) (pub.Item, error) = v.s.Load
	if !v.IsLocalIRI(i) {
		loadFn = v.c.LoadIRI
	}
	it, err := loadFn(i)
	if err != nil {
		return err
	}
	if pub.IsNil(it) {
		return InvalidIRI("%s could not be found locally", i)
	}
	return nil
}

func (v defaultValidator) ValidateClientActor(a pub.Item) (pub.Item, error) {
	if a == nil {
		return a, MissingActivityActor("")
	}
	if err := v.validateLocalIRI(a.GetLink()); err != nil {
		return a, InvalidActivityActor("%s is not local", a.GetLink())
	}
	return v.ValidateActor(a)
}

func (v defaultValidator) ValidateServerActor(a pub.Item) (pub.Item, error) {
	if a == nil {
		return a, InvalidActivityActor("is nil")
	}
	var err error
	if a.IsLink() {
		a, err = v.c.LoadIRI(a.GetLink())
		if err != nil {
			return a, err
		}
		if a == nil {
			return a, errors.NotFoundf("Invalid activity actor")
		}
	}
	err = pub.OnActor(a, func(act *pub.Actor) error {
		if !pub.ActorTypes.Contains(act.GetType()) {
			return InvalidActivityActor("invalid type %s", act.GetType())
		}
		if v.auth != nil {
			if !v.auth.GetLink().Equals(act.GetLink(), false) {
				return InvalidActivityActor("current activity's actor doesn't match the authenticated one")
			}
		}
		a = act
		return nil
	})
	return a, err
}

func (v defaultValidator) ValidateActor(a pub.Item) (pub.Item, error) {
	if a == nil {
		return a, InvalidActivityActor("is nil")
	}
	if a.IsLink() {
		iri := a.GetLink()
		err := v.ValidateLink(iri)
		if err != nil {
			return a, err
		}
		var loadFn func(pub.IRI) (pub.Item, error) = v.s.Load
		if !v.IsLocalIRI(iri) {
			loadFn = v.c.LoadIRI
		}
		if a, err = loadFn(iri); err != nil {
			return a, err
		}
	} else {
		if pub.IsNil(a) {
			return a, errors.NotFoundf("Invalid activity actor")
		}
	}
	return a, pub.OnActor(a, func(act *pub.Actor) error {
		if !pub.ActorTypes.Contains(act.GetType()) {
			return InvalidActivityActor("invalid type %s", act.GetType())
		}
		if v.auth != nil && v.auth.GetLink().Equals(act.GetLink(), false) {
			return nil
		}
		return InvalidActivityActor("current activity's actor doesn't match the authenticated one")
	})
}

func (v defaultValidator) ValidateClientObject(o pub.Item) (pub.Item, error) {
	return v.ValidateObject(o)
}

func (v defaultValidator) ValidateServerObject(o pub.Item) (pub.Item, error) {
	var err error
	if o, err = v.ValidateObject(o); err != nil {
		return o, err
	}
	if err = v.ValidateLink(o.GetLink()); err != nil {
		return o, err
	}
	return o, nil
}

func (v defaultValidator) ValidateObject(o pub.Item) (pub.Item, error) {
	if o == nil {
		return o, InvalidActivityObject("is nil")
	}
	if o.IsLink() {
		iri := o.GetLink()
		err := v.ValidateLink(iri)
		if err != nil {
			return o, err
		}
		var loadFn func(pub.IRI) (pub.Item, error) = v.s.Load
		if !v.IsLocalIRI(iri) {
			loadFn = v.c.LoadIRI
		}
		if o, err = loadFn(iri); err != nil {
			return o, err
		}
		if pub.IsNil(o) {
			return o, errors.NotFoundf("Invalid activity object")
		}
	}
	return o, nil
}

func (v defaultValidator) ValidateTarget(t pub.Item) error {
	if t == nil {
		return InvalidActivityObject("is nil")
	}
	if t.IsLink() {
		return v.ValidateLink(t.GetLink())
	}
	if !(pub.ObjectTypes.Contains(t.GetType()) || pub.ActorTypes.Contains(t.GetType()) || pub.ActivityTypes.Contains(t.GetType())) {
		return InvalidActivityObject("invalid type %s", t.GetType())
	}
	return nil
}

func (v defaultValidator) ValidateAudience(audience ...pub.ItemCollection) error {
	for _, elem := range audience {
		for _, iri := range elem {
			if err := v.validateLocalIRI(iri.GetLink()); err == nil {
				return nil
			}
			if iri.GetLink() == pub.PublicNS {
				return nil
			}
		}
	}
	return errors.Newf("None of the audience elements is local")
}

type CtxtKey string

var ValidatorKey = CtxtKey("__validator")

func ValidatorFromContext(ctx context.Context) (*defaultValidator, bool) {
	ctxVal := ctx.Value(ValidatorKey)
	s, ok := ctxVal.(*defaultValidator)
	return s, ok
}

func (v *defaultValidator) SetActor(p *pub.Actor) {
	v.auth = p
}

func hostSplit(h string) (string, string) {
	pieces := strings.Split(h, ":")
	if len(pieces) == 0 {
		return "", ""
	}
	if len(pieces) == 1 {
		return pieces[0], ""
	}
	return pieces[0], pieces[1]
}

func (v defaultValidator) validateLocalIRI(i pub.IRI) error {
	if len(v.baseIRI) > 0 {
		for _, base := range v.baseIRI {
			if i.Contains(base, false) {
				return nil
			}
		}
		return errors.Newf("%s is not a local IRI", i)
	}
	u, err := i.URL()
	if err != nil {
		return errors.Annotatef(err, "%s is not a local IRI", i)
	}
	v.addr.m.Lock()
	defer v.addr.m.Unlock()
	if _, ok := v.addr.addr[u.Host]; !ok {
		h, _ := hostSplit(u.Host)

		if ip, err := netip.ParseAddr(h); err == nil && !ip.IsUnspecified() {
			v.addr.addr[u.Host] = []netip.Addr{ip}
		} else {
			addrs, err := net.LookupHost(u.Host)
			if err != nil {
				return errors.Annotatef(err, "%s is not a local IRI", i)
			}
			v.addr.addr[u.Host] = make([]netip.Addr, len(addrs))
			for i, addr := range addrs {
				if ip, err = netip.ParseAddr(addr); err == nil && !ip.IsUnspecified() {
					v.addr.addr[u.Host][i] = ip
				}
			}
		}
	}
	for _, ip := range v.addr.addr[u.Host] {
		if ip.IsLoopback() {
			return nil
		}
	}
	return InvalidIRI("%s is not a local IRI", i)
}
