package processing

import (
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"strings"
	"sync"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/filters"
)

type Validator interface {
	ValidateClientActivity(vocab.Item, vocab.IRI) error
}

type ClientActivityValidator interface {
	ValidateClientActivity(vocab.Item, vocab.IRI) error
	ValidateClientObject(vocab.Item) error
	ValidateClientActor(vocab.Item) error
	//ValidateClientTarget(vocab.Item) error
	//ValidateClientAudience(...vocab.ItemCollection) error
}

type ServerActivityValidator interface {
	ValidateServerActivity(vocab.Item, vocab.IRI) error
	ValidateServerObject(vocab.Item) error
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

type ipCache struct {
	addr sync.Map
}

var localAddressCache ipCache

var ValidationError = errors.BadRequestf

var InvalidActivity = func(s string, p ...interface{}) error {
	return ValidationError(fmt.Sprintf("Activity is not valid: %s", s), p...)
}
var MissingActivityActor = func(s string, p ...interface{}) error {
	return ValidationError(fmt.Sprintf("Missing actor %s", s), p...)
}
var InvalidActivityActor = func(s string, p ...interface{}) error {
	return ValidationError(fmt.Sprintf("Actor is not valid: %s", s), p...)
}
var InvalidActivityObject = func(s string, p ...interface{}) error {
	return ValidationError(fmt.Sprintf("Object is not valid: %s", s), p...)
}
var InvalidIRI = func(s string, p ...interface{}) error {
	return ValidationError(fmt.Sprintf("IRI is not valid: %s", s), p...)
}
var InvalidTarget = func(s string, p ...interface{}) error {
	return ValidationError(fmt.Sprintf("Target is not valid: %s", s), p...)
}

func (p P) ValidateServerActivity(a vocab.Item, author vocab.Actor, inbox vocab.IRI) error {
	if !IsInbox(inbox) {
		return errors.NotValidf("Trying to validate a non inbox IRI %s", inbox)
	}
	if author.GetLink().Equals(vocab.PublicNS, true) {
		// NOTE(marius): Should we use 403 Forbidden here?
		return errors.Unauthorizedf("%s actor is not allowed posting to current inbox: %s", name(&author), inbox)
	}
	if vocab.IsNil(a) {
		return InvalidActivity("received nil")
	}
	if a.IsLink() {
		return p.ValidateIRI(a.GetLink())
	}
	if !vocab.ActivityTypes.Contains(a.GetType()) {
		return InvalidActivity("invalid type %s", a.GetType())
	}

	return vocab.OnActivity(a, func(act *vocab.Activity) error {
		if len(act.ID) == 0 {
			return InvalidActivity("invalid activity id %s", act.ID)
		}

		var err error
		if isBlocked(p.s, inbox, act.Actor) {
			return errors.NotFoundf("")
		}
		if act.Actor, err = p.ValidateServerActor(act.Actor, author); err != nil {
			if errors.IsBadRequest(err) {
				act.Actor = &author
			} else {
				return err
			}
		}
		if act.Object, err = p.ValidateServerObject(act.Object); err != nil {
			return err
		}
		if act.Target != nil {
			if act.Target, err = p.ValidateServerObject(act.Target); err != nil {
				return err
			}
		}
		return nil
	})
}

func IsOutbox(i vocab.IRI) bool {
	return strings.ToLower(filepath.Base(i.String())) == strings.ToLower(string(vocab.Outbox))
}

func IsInbox(i vocab.IRI) bool {
	return strings.ToLower(filepath.Base(i.String())) == strings.ToLower(string(vocab.Inbox))
}

// IRIBelongsToActor checks if the search iri represents any of the collections associated with the actor.
func IRIBelongsToActor(iri vocab.IRI, actor vocab.Actor) bool {
	if vocab.Inbox.IRI(actor).Equals(iri, false) {
		return true
	}
	if vocab.Outbox.IRI(actor).Equals(iri, false) {
		return true
	}
	// If it exists the sharedInbox IRI is a valid collection associated with the actor.
	if actor.Endpoints != nil && actor.Endpoints.SharedInbox != nil {
		return actor.Endpoints.SharedInbox.GetLink().Equals(iri, false)
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
	return false
}

func name(a *vocab.Actor) vocab.LangRefValue {
	if a == nil {
		return vocab.LangRefValue{}
	}
	if len(a.Name) > 0 {
		return a.Name.First()
	}
	if len(a.PreferredUsername) > 0 {
		return a.PreferredUsername.First()
	}
	return vocab.LangRefValue{Value: vocab.Content(filepath.Base(string(a.ID)))}
}

func (p P) ValidateActivity(a vocab.Item, author vocab.Actor, receivedIn vocab.IRI) error {
	if vocab.IsNil(a) {
		return InvalidActivityActor("received nil activity")
	}
	if IsOutbox(receivedIn) {
		return p.ValidateClientActivity(a, author, receivedIn)
	}
	if IsInbox(receivedIn) {
		return p.ValidateServerActivity(a, author, receivedIn)
	}

	return errors.MethodNotAllowedf("unable to process activities at current IRI: %s", receivedIn)
}

func (p P) ValidateClientActivity(a vocab.Item, author vocab.Actor, outbox vocab.IRI) error {
	if !IsOutbox(outbox) {
		return errors.NotValidf("trying to validate a non outbox IRI %s", outbox)
	}
	if author.ID == vocab.PublicNS {
		// NOTE(marius): Should we use 403 Forbidden here?
		return errors.Unauthorizedf("missing actor: not allowed to post to outbox %s", outbox)
	}
	if !IRIBelongsToActor(outbox, author) {
		// NOTE(marius): Should we use 403 Forbidden here?
		return errors.Unauthorizedf("actor %q does not own the current outbox %s", name(&author), outbox)
	}
	if vocab.IsNil(a) {
		return InvalidActivityActor("received nil activity")
	}
	if a.IsLink() {
		return p.ValidateIRI(a.GetLink())
	}

	validActivityTypes := append(vocab.ActivityTypes, vocab.IntransitiveActivityTypes...)
	if !validActivityTypes.Contains(a.GetType()) {
		return InvalidActivity("invalid type %s", a.GetType())
	}

	err := vocab.OnIntransitiveActivity(a, func(act *vocab.IntransitiveActivity) error {
		var err error

		act.Actor, _ = p.DereferenceItem(act.Actor)
		if act.Actor, err = p.ValidateClientActor(act.Actor, author); err != nil {
			if errors.IsBadRequest(err) {
				act.Actor = &author
			} else {
				return err
			}
		}
		if vocab.IsNil(act.AttributedTo) {
			act.AttributedTo = &author
		}
		if !vocab.IsNil(act.Target) {
			act.Target, _ = p.DereferenceItem(act.Target)
			if act.Target, err = p.ValidateClientObject(act.Target); err != nil {
				return err
			}
		}
		return err
	})
	if err != nil {
		return err
	}

	if vocab.QuestionActivityTypes.Contains(a.GetType()) {
		err = vocab.OnQuestion(a, func(q *vocab.Question) error {
			return ValidateClientQuestionActivity(p.s, q)
		})
		if err != nil {
			return err
		}
	}

	if vocab.ActivityTypes.Contains(a.GetType()) {
		err = vocab.OnActivity(a, func(act *vocab.Activity) error {
			// @TODO(marius): this needs to be extended by a ValidateActivityClientObject
			//   because the first step would be to test the object in the context of the activity
			//   The ValidateActivityClientObject could then validate just the object itself.
			act.Object, _ = p.DereferenceItem(act.Object)
			if act.Object, err = p.ValidateClientObject(act.Object); err != nil {
				return err
			}

			if vocab.ContentManagementActivityTypes.Contains(act.GetType()) && act.Object.GetType() != vocab.RelationshipType {
				err = ValidateClientContentManagementActivity(p.s, act)
			} else if vocab.CollectionManagementActivityTypes.Contains(act.GetType()) {
				err = ValidateClientCollectionManagementActivity(p.s, act)
			} else if vocab.ReactionsActivityTypes.Contains(act.GetType()) {
				err = ValidateClientReactionsActivity(p.s, act)
			} else if vocab.EventRSVPActivityTypes.Contains(act.GetType()) {
				err = ValidateClientEventRSVPActivity(p.s, act)
			} else if vocab.GroupManagementActivityTypes.Contains(act.GetType()) {
				err = ValidateClientGroupManagementActivity(p.s, act)
			} else if vocab.ContentExperienceActivityTypes.Contains(act.GetType()) {
				err = ValidateClientContentExperienceActivity(p.s, act)
			} else if vocab.GeoSocialEventsActivityTypes.Contains(act.GetType()) {
				err = ValidateClientGeoSocialEventsActivity(p.s, act)
			} else if vocab.NotificationActivityTypes.Contains(act.GetType()) {
				err = ValidateClientNotificationActivity(p.s, act)
			} else if vocab.RelationshipManagementActivityTypes.Contains(act.GetType()) {
				err = ValidateClientRelationshipManagementActivity(p.s, act)
			} else if vocab.NegatingActivityTypes.Contains(act.GetType()) {
				err = ValidateClientNegatingActivity(p.s, act)
			} else if vocab.OffersActivityTypes.Contains(act.GetType()) {
				err = ValidateClientOffersActivity(p.s, act)
			}
			return err
		})
	}
	return err
}

// ValidateClientContentManagementActivity
func ValidateClientContentManagementActivity(l ReadStore, act *vocab.Activity) error {
	if vocab.IsNil(act.Object) {
		return errors.NotValidf("nil object for %s activity", act.Type)
	}

	return vocab.OnItem(act.Object, func(ob vocab.Item) error {
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
	})
}

// ValidateClientCollectionManagementActivity
func ValidateClientCollectionManagementActivity(l ReadStore, act *vocab.Activity) error {
	return nil
}

// ValidateClientReactionsActivity
func ValidateClientReactionsActivity(l ReadStore, act *vocab.Activity) error {
	if act.Object != nil {
		switch act.Type {
		case vocab.DislikeType:
			fallthrough
		case vocab.LikeType:
			//return ValidateAppreciationActivity(l, act)
		case vocab.RejectType, vocab.TentativeRejectType:
			return ValidateClientRejectActivity(l, act)
		case vocab.TentativeAcceptType, vocab.AcceptType:
			return ValidateClientAcceptActivity(l, act)
		case vocab.BlockType:
			//return ValidateBlockActivity(l, act)
		case vocab.FlagType:
			//return ValidateFlagActivity(l, act)
		case vocab.IgnoreType:
			//return ValidateIgnoreActivity(l, act)
		}
	}

	return nil
}

// ValidateClientAcceptActivity
func ValidateClientAcceptActivity(l ReadStore, act *vocab.Activity) error {
	if err := ValidateAcceptActivity(l, act); err != nil {
		return err
	}
	return vocab.OnActivity(act.Object, func(follow *vocab.Activity) error {
		if follow.GetType() != vocab.FollowType {
			return errors.NotValidf("object Activity type %s is incorrect, expected %s", follow.Type, vocab.FollowType)
		}
		if !act.Actor.GetLink().Equals(follow.Object.GetLink(), false) {
			return errors.NotValidf(
				"The %s activity has a different actor than the received %s's object: %s, expected %s",
				act.Type, follow.Type,
				act.Actor.GetLink(),
				follow.Object.GetLink(),
			)
		}
		return nil
	})
}

// ValidateAcceptActivity
func ValidateAcceptActivity(l ReadStore, act *vocab.Activity) error {
	good := vocab.ActivityVocabularyTypes{vocab.AcceptType, vocab.TentativeAcceptType}
	if !good.Contains(act.Type) {
		return errors.NotValidf("Activity has wrong type %s, expected %v", act.Type, good)
	}
	return nil
}

// ValidateClientRejectActivity
func ValidateClientRejectActivity(l ReadStore, act *vocab.Activity) error {
	if err := ValidateRejectActivity(l, act); err != nil {
		return err
	}

	return vocab.OnActivity(act.Object, func(follow *vocab.Activity) error {
		if follow.GetType() != vocab.FollowType {
			return errors.NotValidf("object Activity type %s is incorrect, expected %s", follow.Type, vocab.FollowType)
		}
		if !act.Actor.GetLink().Equals(follow.Object.GetLink(), false) {
			return errors.NotValidf(
				"The %s activity has a different actor than the received %s's object: %s, expected %s",
				act.Type, follow.Type,
				act.Actor.GetLink(),
				follow.Object.GetLink(),
			)
		}
		return nil
	})
}

// ValidateRejectActivity
func ValidateRejectActivity(l ReadStore, act *vocab.Activity) error {
	good := vocab.ActivityVocabularyTypes{vocab.RejectType, vocab.TentativeRejectType}
	if !good.Contains(act.Type) {
		return errors.NotValidf("Activity has wrong type %s, expected %v", act.Type, good)
	}
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
func ValidateClientQuestionActivity(l ReadStore, act *vocab.Question) error {
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
	case vocab.AddType, vocab.BlockType, vocab.CreateType, vocab.DeleteType,
		vocab.IgnoreType, vocab.InviteType, vocab.AcceptType, vocab.RejectType:
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

// IsLocal shows if the received IRI belongs to the current instance
func (p P) IsLocal(i vocab.Item) bool {
	return isLocalIRI(i.GetLink()) || p.validateLocalIRI(i.GetLink()) == nil
}

// IsLocalIRI shows if the received IRI belongs to the current instance
func (p P) IsLocalIRI(i vocab.IRI) bool {
	return isLocalIRI(i) || p.validateLocalIRI(i) == nil
}

func (p P) ValidateClientActor(a vocab.Item, expected vocab.Actor) (vocab.Item, error) {
	if vocab.IsNil(a) {
		return a, InvalidActivityActor("is nil")
	}

	err := vocab.OnItem(a, func(item vocab.Item) error {
		return p.validateLocalIRI(a.GetLink())
	})
	if err != nil {
		return a, InvalidActivityActor("%s is not local", a.GetLink())
	}
	return p.ValidateActor(a, expected)
}

func (p P) ValidateIRI(i vocab.IRI) error {
	if i.Equals(vocab.PublicNS, false) {
		return InvalidIRI("Public namespace is not a valid IRI")
	}
	if _, err := i.URL(); err != nil {
		return errors.Annotatef(err, "underlying URL could not be parsed: %s", i)
	}
	return nil
}

func (p P) ValidateServerActor(a vocab.Item, expected vocab.Actor) (vocab.Item, error) {
	return p.ValidateActor(a, expected)
}

func (p P) ValidateActor(a vocab.Item, expected vocab.Actor) (vocab.Item, error) {
	if vocab.IsNil(a) {
		return a, InvalidActivityActor("is nil")
	}
	if a.IsLink() {
		iri := a.GetLink()
		err := p.ValidateIRI(iri)
		if err != nil {
			return a, err
		}
		var loadFn = p.s.Load
		if !p.IsLocalIRI(iri) {
			loadFn = func(iri vocab.IRI, _ ...filters.Check) (vocab.Item, error) {
				return p.c.LoadIRI(iri)
			}
		}
		if a, err = loadFn(iri); err != nil {
			return a, err
		}
	} else {
		if vocab.IsNil(a) {
			return a, errors.NotFoundf("Invalid actor")
		}
	}
	err := vocab.OnActor(a, func(act *vocab.Actor) error {
		a = act
		if !vocab.ActorTypes.Contains(act.GetType()) {
			return InvalidActivityActor("invalid type %s", act.GetType())
		}
		if !expected.GetLink().Equals(act.GetLink(), false) {
			return InvalidActivityActor("the actor doesn't match the authenticated one")
		}
		return nil
	})
	return a, err
}

func (p P) ValidateClientObject(o vocab.Item) (vocab.Item, error) {
	if vocab.IsNil(o) {
		return nil, InvalidActivityObject("is nil")
	}
	fetchIRI := func(o vocab.Item) (vocab.Item, error) {
		if vocab.IsNil(o) {
			return o, errors.NotFoundf("Invalid activity object")
		}

		iri := o.GetLink()
		err := p.ValidateIRI(iri)
		if err != nil {
			return o, err
		}
		if !p.IsLocalIRI(iri) {
			return o, nil
		}
		if o, err = p.s.Load(iri); err != nil {
			return o, err
		}
		if vocab.IsNil(o) {
			return o, errors.NotFoundf("Invalid activity object")
		}
		return o, nil
	}
	if o.IsLink() {
		return fetchIRI(o)
	}
	if vocab.IsItemCollection(o) {
		derefObj := make(vocab.ItemCollection, 0)
		err := vocab.OnItemCollection(o, func(col *vocab.ItemCollection) error {
			for _, maybeIRI := range col.Collection() {
				ob, err := fetchIRI(maybeIRI)
				if err != nil {
					continue
				}
				_ = derefObj.Append(ob)
			}
			if derefObj.Count() == 0 {
				return errors.NotFoundf("Invalid activity object, unable to dereference item collection")
			}
			return nil
		})
		if err != nil {
			return o, nil
		}
		o = derefObj
	}
	return o, nil
}

func (p P) ValidateServerObject(o vocab.Item) (vocab.Item, error) {
	if vocab.IsNil(o) {
		return o, InvalidActivityObject("is nil")
	}
	if err := p.ValidateIRI(o.GetLink()); err != nil {
		return o, err
	}
	return o, nil
}

func (p P) ValidateTarget(t vocab.Item) error {
	if vocab.IsNil(t) {
		return InvalidActivityObject("is nil")
	}
	if t.IsLink() {
		return p.ValidateIRI(t.GetLink())
	}
	if !(vocab.ObjectTypes.Contains(t.GetType()) || vocab.ActorTypes.Contains(t.GetType()) || vocab.ActivityTypes.Contains(t.GetType())) {
		return InvalidActivityObject("invalid type %s", t.GetType())
	}
	return nil
}

func (p P) ValidateAudience(audience ...vocab.ItemCollection) error {
	for _, elem := range audience {
		for _, iri := range elem {
			if err := p.validateLocalIRI(iri.GetLink()); err == nil {
				return nil
			}
			if iri.GetLink() == vocab.PublicNS {
				return nil
			}
		}
	}
	return errors.Newf("None of the audience elements is local")
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

func (p P) validateLocalIRI(i vocab.IRI) error {
	if len(p.baseIRI) > 0 {
		for _, base := range p.baseIRI {
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
	if _, ok := localAddressCache.addr.Load(u.Host); !ok {
		h, _ := hostSplit(u.Host)

		if ip, err := netip.ParseAddr(h); err == nil && !ip.IsUnspecified() {
			localAddressCache.addr.Store(u.Host, []netip.Addr{ip})
		} else {
			addrs, err := net.LookupHost(u.Host)
			if err != nil {
				return errors.Annotatef(err, "%s is not a local IRI", i)
			}
			hosts := make([]netip.Addr, len(addrs))
			for i, a := range addrs {
				if ip, err = netip.ParseAddr(a); err == nil && !ip.IsUnspecified() {
					hosts[i] = ip
				}
			}
			localAddressCache.addr.Store(u.Host, hosts)
		}
	}

	if v, found := localAddressCache.addr.Load(u.Host); found {
		if ips, ok := v.([]netip.Addr); ok {
			for _, ip := range ips {
				if ip.IsLoopback() {
					return nil
				}
			}
		}
	}
	return InvalidIRI("%s is not a local IRI", i)
}
