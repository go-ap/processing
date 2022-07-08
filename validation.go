package processing

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"path"
	"strings"
	"sync"

	vocab "github.com/go-ap/activitypub"
	c "github.com/go-ap/client"
	"github.com/go-ap/errors"
)

type Validator interface {
	ValidateClientActivity(vocab.Item, vocab.IRI) error
}

type ClientActivityValidator interface {
	ValidateClientActivity(vocab.Item, vocab.IRI) error
	//ValidateClientObject(vocab.Item) error
	ValidateClientActor(vocab.Item) error
	//ValidateClientTarget(vocab.Item) error
	//ValidateClientAudience(...vocab.ItemCollection) error
}

type ServerActivityValidator interface {
	ValidateServerActivity(vocab.Item, vocab.IRI) error
	//ValidateServerObject(vocab.Item) error
	ValidateServerActor(vocab.Item) error
	//ValidateServerTarget(vocab.Item) error
	//ValidateServerAudience(...vocab.ItemCollection) error
}

// ActivityValidator is an interface used for validating activity objects.
type ActivityValidator interface {
	ClientActivityValidator
	ServerActivityValidator
}

//type AudienceValidator interface {
//	ValidateAudience(...vocab.ItemCollection) error
//}

// ObjectValidator is an interface used for validating generic objects
type ObjectValidator interface {
	ValidateObject(vocab.Item) error
}

// ActorValidator is an interface used for validating actor objects
type ActorValidator interface {
	ValidActor(vocab.Item) error
}

// TargetValidator is an interface used for validating an object that is an activity's target
// TODO(marius): this seems to have a different semantic than the previous ones.
//  Ie, any object can be a target, but in the previous cases, the main validation mechanism is based on the Type.
//type TargetValidator interface {
//	ValidTarget(vocab.Item) error
//}

type invalidActivity struct {
	errors.Err
}

type ipCache struct {
	addr map[string][]netip.Addr
	m    sync.RWMutex
}

type defaultValidator struct {
	baseIRI vocab.IRIs
	addr    ipCache
	auth    *vocab.Actor
	c       c.Basic
	s       ReadStore
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

func (v defaultValidator) ValidateServerActivity(a vocab.Item, inbox vocab.IRI) error {
	if !IsInbox(inbox) {
		return errors.NotValidf("Trying to validate a non inbox IRI %s", inbox)
	}
	//if v.auth.GetLink() == vocab.PublicNS {
	//	return errors.Unauthorizedf("%s actor is not allowed posting to current inbox", v.auth.Name)
	//}
	if a == nil {
		return InvalidActivityActor("received nil activity")
	}
	if a.IsLink() {
		return v.ValidateLink(a.GetLink())
	}
	if !vocab.ActivityTypes.Contains(a.GetType()) {
		return InvalidActivity("invalid type %s", a.GetType())
	}
	return vocab.OnActivity(a, func(act *vocab.Activity) error {
		if len(act.ID) == 0 {
			return InvalidActivity("invalid activity id %s", act.ID)
		}

		inboxBelongsTo, err := vocab.Inbox.OfActor(inbox)
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

func IsOutbox(i vocab.IRI) bool {
	return strings.ToLower(path.Base(i.String())) == strings.ToLower(string(vocab.Outbox))
}

func IsInbox(i vocab.IRI) bool {
	return strings.ToLower(path.Base(i.String())) == strings.ToLower(string(vocab.Inbox))
}

// IRIBelongsToActor checks if the search iri represents any of the collections associated with the actor.
func IRIBelongsToActor(iri vocab.IRI, actor *vocab.Actor) bool {
	if actor == nil {
		return false
	}
	if vocab.Inbox.IRI(actor).Equals(iri, false) {
		return true
	}
	if vocab.Outbox.IRI(actor).Equals(iri, false) {
		return true
	}
	// The following should not really come into question at any point.
	// This function should be used for checking inbox/outbox/sharedInbox IRIS
	if vocab.Following.IRI(actor).Equals(iri, false) {
		return true
	}
	if vocab.Followers.IRI(actor).Equals(iri, false) {
		return true
	}
	if vocab.Replies.IRI(actor).Equals(iri, false) {
		return true
	}
	if vocab.Liked.IRI(actor).Equals(iri, false) {
		return true
	}
	if vocab.Shares.IRI(actor).Equals(iri, false) {
		return true
	}
	if vocab.Likes.IRI(actor).Equals(iri, false) {
		return true
	}
	if actor.Endpoints != nil && actor.Endpoints.SharedInbox.GetLink().Equals(iri, false) {
		return true
	}
	return false
}

var missingActor = new(MissingActorError)

func name(a *vocab.Actor) vocab.LangRefValue {
	if len(a.Name) > 0 {
		return a.Name.First()
	}
	if len(a.PreferredUsername) > 0 {
		return a.PreferredUsername.First()
	}
	return vocab.LangRefValue{Value: vocab.Content(path.Base(string(a.ID)))}
}

func (v defaultValidator) ValidateActivity(a vocab.Item, receivedIn vocab.IRI) error {
	if IsOutbox(receivedIn) {
		return v.ValidateClientActivity(a, receivedIn)
	}
	if IsInbox(receivedIn) {
		return v.ValidateServerActivity(a, receivedIn)
	}

	return errors.MethodNotAllowedf("unable to process activities at current IRI: %s", receivedIn)
}

func (v defaultValidator) ValidateClientActivity(a vocab.Item, outbox vocab.IRI) error {
	if !IsOutbox(outbox) {
		return errors.NotValidf("Trying to validate a non outbox IRI %s", outbox)
	}
	if v.auth == nil || v.auth.GetLink() == vocab.PublicNS {
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
	validActivityTypes := append(vocab.ActivityTypes, vocab.IntransitiveActivityTypes...)
	if !validActivityTypes.Contains(a.GetType()) {
		return InvalidActivity("invalid type %s", a.GetType())
	}
	return vocab.OnActivity(a, func(act *vocab.Activity) error {
		var err error
		if act.Actor, err = v.ValidateClientActor(act.Actor); err != nil {
			if missingActor.Is(err) && v.auth != nil {
				act.Actor = v.auth
			} else {
				return err
			}
		}
		if !vocab.IntransitiveActivityTypes.Contains(a.GetType()) {
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
		if vocab.ContentManagementActivityTypes.Contains(act.GetType()) && act.Object.GetType() != vocab.RelationshipType {
			err = ValidateClientContentManagementActivity(v.s, act)
		} else if vocab.CollectionManagementActivityTypes.Contains(act.GetType()) {
			err = ValidateClientCollectionManagementActivity(v.s, act)
		} else if vocab.ReactionsActivityTypes.Contains(act.GetType()) {
			err = ValidateClientReactionsActivity(v.s, act)
		} else if vocab.EventRSVPActivityTypes.Contains(act.GetType()) {
			err = ValidateClientEventRSVPActivity(v.s, act)
		} else if vocab.GroupManagementActivityTypes.Contains(act.GetType()) {
			err = ValidateClientGroupManagementActivity(v.s, act)
		} else if vocab.ContentExperienceActivityTypes.Contains(act.GetType()) {
			err = ValidateClientContentExperienceActivity(v.s, act)
		} else if vocab.GeoSocialEventsActivityTypes.Contains(act.GetType()) {
			err = ValidateClientGeoSocialEventsActivity(v.s, act)
		} else if vocab.NotificationActivityTypes.Contains(act.GetType()) {
			err = ValidateClientNotificationActivity(v.s, act)
		} else if vocab.QuestionActivityTypes.Contains(act.GetType()) {
			err = ValidateClientQuestionActivity(v.s, act)
		} else if vocab.RelationshipManagementActivityTypes.Contains(act.GetType()) {
			err = ValidateClientRelationshipManagementActivity(v.s, act)
		} else if vocab.NegatingActivityTypes.Contains(act.GetType()) {
			err = ValidateClientNegatingActivity(v.s, act)
		} else if vocab.OffersActivityTypes.Contains(act.GetType()) {
			err = ValidateClientOffersActivity(v.s, act)
		}
		return err
	})
}

// ValidateClientContentManagementActivity
func ValidateClientContentManagementActivity(l ReadStore, act *vocab.Activity) error {
	if act.Object == nil {
		return errors.NotValidf("nil object for %s activity", act.Type)
	}
	ob := act.Object
	switch act.Type {
	case vocab.UpdateType:
		if vocab.ActivityTypes.Contains(ob.GetType()) {
			return errors.Newf("trying to update an immutable activity")
		}
		fallthrough
	case vocab.DeleteType:
		if len(ob.GetLink()) == 0 {
			return errors.Newf("invalid object id for %s activity", act.Type)
		}
		if ob.IsLink() {
			return nil
		}
		var (
			found vocab.Item
			err   error
		)

		found, err = l.Load(ob.GetLink())
		if err != nil {
			return errors.Annotatef(err, "failed to load object from storage")
		}
		if found == nil {
			return errors.NotFoundf("found nil object in storage")
		}
	case vocab.CreateType:
	default:
	}

	return nil
}

// ValidateClientCollectionManagementActivity
func ValidateClientCollectionManagementActivity(l ReadStore, act *vocab.Activity) error {
	return nil
}

// ValidateClientReactionsActivity
func ValidateClientReactionsActivity(l ReadStore, act *vocab.Activity) error {
	return nil
}

// ValidateClientEventRSVPActivity
func ValidateClientEventRSVPActivity(l ReadStore, act *vocab.Activity) error {
	return nil
}

// ValidateClientGroupManagementActivity
func ValidateClientGroupManagementActivity(l ReadStore, act *vocab.Activity) error {
	return nil
}

// ValidateClientContentExperienceActivity
func ValidateClientContentExperienceActivity(l ReadStore, act *vocab.Activity) error {
	return nil
}

// ValidateClientGeoSocialEventsActivity
func ValidateClientGeoSocialEventsActivity(l ReadStore, act *vocab.Activity) error {
	return nil
}

// ValidateClientNotificationActivity
func ValidateClientNotificationActivity(l ReadStore, act *vocab.Activity) error {
	return nil
}

// ValidateClientQuestionActivity
func ValidateClientQuestionActivity(l ReadStore, act *vocab.Activity) error {
	return nil
}

// ValidateClientRelationshipManagementActivity
func ValidateClientRelationshipManagementActivity(l ReadStore, act *vocab.Activity) error {
	switch act.Type {
	case vocab.FollowType:
		if iri := act.GetLink(); len(iri) > 0 {
			if a, _ := l.Load(iri); !vocab.IsNil(firstOrItem(a)) {
				return errors.Conflictf("%s already exists for this actor/object pair", act.Type)
			}
		}
	case vocab.AddType:
	case vocab.BlockType:
	case vocab.CreateType:
	case vocab.DeleteType:
	case vocab.IgnoreType:
	case vocab.InviteType:
	case vocab.AcceptType:
		fallthrough
	case vocab.RejectType:
		// TODO(marius): either the actor or the object needs to be local for this action to be valid
		//   in the case of C2S... the actor needs to be local
		//   in the case of S2S... the object needs to be local
		// TODO(marius): Object needs to be a valid Follow activity
	default:
	}
	return nil
}

// ValidateClientNegatingActivity
func ValidateClientNegatingActivity(l ReadStore, act *vocab.Activity) error {
	return nil
}

// ValidateClientOffersActivity
func ValidateClientOffersActivity(l ReadStore, act *vocab.Activity) error {
	return nil
}

// IsLocalIRI shows if the received IRI belongs to the current instance
func (v defaultValidator) IsLocalIRI(i vocab.IRI) bool {
	return v.validateLocalIRI(i) == nil
}

func (v defaultValidator) ValidateLink(i vocab.IRI) error {
	if i.Equals(vocab.PublicNS, false) {
		return InvalidIRI("Public namespace is not a local IRI")
	}
	var loadFn func(vocab.IRI) (vocab.Item, error) = v.s.Load
	if !v.IsLocalIRI(i) {
		loadFn = v.c.LoadIRI
	}
	it, err := loadFn(i)
	if err != nil {
		return err
	}
	if vocab.IsNil(it) {
		return InvalidIRI("%s could not be found locally", i)
	}
	return nil
}

func (v defaultValidator) ValidateClientActor(a vocab.Item) (vocab.Item, error) {
	if a == nil {
		return a, MissingActivityActor("")
	}
	if err := v.validateLocalIRI(a.GetLink()); err != nil {
		return a, InvalidActivityActor("%s is not local", a.GetLink())
	}
	return v.ValidateActor(a)
}

func (v defaultValidator) ValidateServerActor(a vocab.Item) (vocab.Item, error) {
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
	err = vocab.OnActor(a, func(act *vocab.Actor) error {
		if !vocab.ActorTypes.Contains(act.GetType()) {
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

func (v defaultValidator) ValidateActor(a vocab.Item) (vocab.Item, error) {
	if a == nil {
		return a, InvalidActivityActor("is nil")
	}
	if a.IsLink() {
		iri := a.GetLink()
		err := v.ValidateLink(iri)
		if err != nil {
			return a, err
		}
		var loadFn func(vocab.IRI) (vocab.Item, error) = v.s.Load
		if !v.IsLocalIRI(iri) {
			loadFn = v.c.LoadIRI
		}
		if a, err = loadFn(iri); err != nil {
			return a, err
		}
	} else {
		if vocab.IsNil(a) {
			return a, errors.NotFoundf("Invalid activity actor")
		}
	}
	return a, vocab.OnActor(a, func(act *vocab.Actor) error {
		if !vocab.ActorTypes.Contains(act.GetType()) {
			return InvalidActivityActor("invalid type %s", act.GetType())
		}
		if v.auth != nil && v.auth.GetLink().Equals(act.GetLink(), false) {
			return nil
		}
		return InvalidActivityActor("current activity's actor doesn't match the authenticated one")
	})
}

func (v defaultValidator) ValidateClientObject(o vocab.Item) (vocab.Item, error) {
	return v.ValidateObject(o)
}

func (v defaultValidator) ValidateServerObject(o vocab.Item) (vocab.Item, error) {
	var err error
	if o, err = v.ValidateObject(o); err != nil {
		return o, err
	}
	if err = v.ValidateLink(o.GetLink()); err != nil {
		return o, err
	}
	return o, nil
}

func (v defaultValidator) ValidateObject(o vocab.Item) (vocab.Item, error) {
	if o == nil {
		return o, InvalidActivityObject("is nil")
	}
	if o.IsLink() {
		iri := o.GetLink()
		err := v.ValidateLink(iri)
		if err != nil {
			return o, err
		}
		var loadFn func(vocab.IRI) (vocab.Item, error)
		if !v.IsLocalIRI(iri) {
			loadFn = v.c.LoadIRI
		} else {
			// FIXME(marius): this does not work for the case where IRI is not a Public item
			// We need to invent a way to pass the currently authorized actor to the ReadStore.Load
			// The way we're doing it now is not great as it makes assumption that the underlying storage
			// receives the authenticated actor as a basic auth user in the IRI. Maybe that's a safe
			// assumption to make, but I'm not thrilled about it.
			if v.auth != nil {
				u, _ := iri.URL()
				u.User = url.User(v.auth.ID.String())
				iri = vocab.IRI(u.String())
			}
			loadFn = v.s.Load
		}
		if o, err = loadFn(iri); err != nil {
			return o, err
		}
		if vocab.IsNil(o) {
			return o, errors.NotFoundf("Invalid activity object")
		}
	}
	return o, nil
}

func (v defaultValidator) ValidateTarget(t vocab.Item) error {
	if t == nil {
		return InvalidActivityObject("is nil")
	}
	if t.IsLink() {
		return v.ValidateLink(t.GetLink())
	}
	if !(vocab.ObjectTypes.Contains(t.GetType()) || vocab.ActorTypes.Contains(t.GetType()) || vocab.ActivityTypes.Contains(t.GetType())) {
		return InvalidActivityObject("invalid type %s", t.GetType())
	}
	return nil
}

func (v defaultValidator) ValidateAudience(audience ...vocab.ItemCollection) error {
	for _, elem := range audience {
		for _, iri := range elem {
			if err := v.validateLocalIRI(iri.GetLink()); err == nil {
				return nil
			}
			if iri.GetLink() == vocab.PublicNS {
				return nil
			}
		}
	}
	return errors.Newf("None of the audience elements is local")
}

var ValidatorKey = CtxtKey("__validator")

func ValidatorFromContext(ctx context.Context) (*defaultValidator, bool) {
	ctxVal := ctx.Value(ValidatorKey)
	s, ok := ctxVal.(*defaultValidator)
	return s, ok
}

func (v *defaultValidator) SetActor(p *vocab.Actor) {
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

func (v defaultValidator) validateLocalIRI(i vocab.IRI) error {
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
