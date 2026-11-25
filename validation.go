package processing

import (
	"fmt"
	"path/filepath"

	"git.sr.ht/~mariusor/lw"
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
}

type ServerActivityValidator interface {
	ValidateServerActivity(vocab.Item, vocab.IRI) error
	ValidateServerObject(vocab.Item) error
	ValidateServerActor(vocab.Item) error
}

// ActivityValidator is an interface used for validating activity objects.
type ActivityValidator interface {
	ClientActivityValidator
	ServerActivityValidator
}

//type AudienceValidator interface {
//	ValidateAudienceForRemoteActivity(...vocab.ItemCollection) error
//}

// ObjectValidator is an interface used for validating generic objects
type ObjectValidator interface {
	ValidateObject(vocab.Item) error
}

// ActorValidator is an interface used for validating actor objects
type ActorValidator interface {
	ValidActor(vocab.Item) error
}

var ValidationError = errors.BadRequestf

var InvalidActivity = func(s string, p ...any) error {
	return ValidationError(fmt.Sprintf("Activity is not valid: %s", s), p...)
}
var InvalidActivityActor = func(s string, p ...any) error {
	return ValidationError(fmt.Sprintf("Actor is not valid: %s", s), p...)
}
var InvalidActivityObject = func(s string, p ...any) error {
	return ValidationError(fmt.Sprintf("Object is not valid: %s", s), p...)
}
var InvalidIRI = func(s string, p ...any) error {
	return ValidationError(fmt.Sprintf("IRI is not valid: %s", s), p...)
}

var errMissingObject = func(typ vocab.Typer) error {
	return ValidationError("missing object for %v Activity", typ)
}
var errMissingActor = func(typ vocab.Typer) error {
	return ValidationError("missing object for %v Activity", typ)
}

var validActivityTypes = append(vocab.ActivityTypes, vocab.IntransitiveActivityTypes...)

func (p *P) ValidateServerActivity(a vocab.Item, author vocab.Actor, inbox vocab.IRI) error {
	if !IsInbox(inbox) {
		return errors.BadRequestf("Trying to validate a non inbox IRI %s", inbox)
	}

	if vocab.PublicNS.Equal(author.ID) {
		// NOTE(marius): Should we use 403 Forbidden here?
		return errors.Unauthorizedf("%s actor is not allowed posting to current inbox: %s", name(&author), inbox)
	}
	if vocab.IsNil(a) {
		return InvalidActivity("received nil")
	}
	if vocab.IsIRI(a) {
		return p.ValidateIRI(a.GetLink())
	}
	if !validActivityTypes.Match(a.GetType()) {
		return InvalidActivity("invalid type %v", a.GetType())
	}

	var err error
	if !p.skipValidationOnInboundCollections {
		// NOTE(marius): check if the inbox collection actually exists
		if _, err = p.s.Load(inbox, filters.WithMaxCount(1)); err != nil {
			return errors.NewMethodNotAllowed(err, "invalid collection %s", inbox)
		}
	}

	maybeOwner, _ := vocab.Split(inbox)
	inboxOwnerHasBlocked := p.actorHasBlockedFn(maybeOwner)
	if inboxOwnerHasBlocked(author) {
		// NOTE(marius): if the inbox owner has blocked the authorized actor, we error
		p.l.WithContext(lw.Ctx{"actor": maybeOwner.GetID(), "rec": author.ID}).Tracef("Skipping blocked authorized actor")
		return errors.NotFoundf("")
	}

	err = vocab.OnIntransitiveActivity(a, func(act *vocab.IntransitiveActivity) error {
		if len(act.ID) == 0 {
			return InvalidActivity("empty activity id")
		}
		if inboxOwnerHasBlocked(act.Actor) {
			return errors.NotFoundf("")
		}

		var err error
		if act.Actor, err = p.ValidateServerActor(act.Actor, author); err != nil {
			if errors.IsBadRequest(err) {
				act.Actor = &author
			} else {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	typ := a.GetType()
	if vocab.ActivityTypes.Match(typ) {
		err = vocab.OnActivity(a, func(act *vocab.Activity) error {
			if act.Object, err = p.ValidateServerObject(act.Object); err != nil {
				return err
			}
			switch {
			case vocab.ContentManagementActivityTypes.Match(typ) && !vocab.RelationshipType.Match(act.Object.GetType()):
				return p.ValidateServerContentManagementActivity(act)
			case vocab.CollectionManagementActivityTypes.Match(typ):
				return p.ValidateServerCollectionManagementActivity(act)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func collectionMatchesIRI(i vocab.IRI, col vocab.CollectionPath) bool {
	_, maybePath := vocab.Split(i)
	return maybePath == col
}

func IsOutbox(i vocab.IRI) bool {
	return collectionMatchesIRI(i, vocab.Outbox)
}

func IsInbox(i vocab.IRI) bool {
	return collectionMatchesIRI(i, vocab.Inbox)
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

func name(a *vocab.Actor) vocab.Content {
	if a == nil {
		return nil
	}
	if len(a.Name) > 0 {
		return a.Name.First()
	}
	if len(a.PreferredUsername) > 0 {
		return a.PreferredUsername.First()
	}
	return vocab.Content(filepath.Base(string(a.ID)))
}

func (p *P) ValidateActivity(a vocab.Item, author vocab.Actor, receivedIn vocab.IRI) error {
	if vocab.IsNil(a) {
		return InvalidActivity("received nil activity")
	}
	if IsOutbox(receivedIn) {
		return p.ValidateClientActivity(a, author, receivedIn)
	}
	if IsInbox(receivedIn) {
		return p.ValidateServerActivity(a, author, receivedIn)
	}

	return errors.MethodNotAllowedf("unable to process activities at current IRI: %s", receivedIn)
}

func (p *P) ValidateClientActivity(a vocab.Item, author vocab.Actor, outbox vocab.IRI) error {
	if !IsOutbox(outbox) {
		return errors.BadRequestf("trying to validate a non outbox IRI %s", outbox)
	}
	if author.ID == "" || vocab.PublicNS.Equal(author.ID) {
		// NOTE(marius): Should we use 403 Forbidden here?
		return errors.Unauthorizedf("missing actor: not allowed to post to outbox %s", outbox)
	}
	if !IRIBelongsToActor(outbox, author) {
		// NOTE(marius): Should we use 403 Forbidden here?
		return errors.Unauthorizedf("actor %s does not own the current outbox %s", name(&author), outbox)
	}
	if vocab.IsNil(a) {
		return InvalidActivity("is nil")
	}

	var err error
	if !p.skipValidationOnInboundCollections {
		// NOTE(marius): check if the outbox collection actually exists
		if _, err = p.s.Load(outbox, filters.WithMaxCount(1)); err != nil {
			return errors.NewMethodNotAllowed(err, "invalid collection %s", outbox)
		}
	}

	if vocab.IsIRI(a) {
		if a, err = p.DereferenceItem(a.GetLink()); err != nil {
			return errors.NewBadRequest(err, "unable to dereference activity")
		}
	}

	if !validActivityTypes.Match(a.GetType()) {
		return InvalidActivity("invalid type %v", a.GetType())
	}

	err = vocab.OnIntransitiveActivity(a, func(act *vocab.IntransitiveActivity) error {
		if !p.allowClientActivityID && act.ID != "" {
			act.ID = ""
			p.l.Debugf("activity has an ID, resetting it.")
		}
		// NOTE(marius): validate the local actor
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
			if act.Target, err = p.ValidateClientObject(act.Target); err != nil {
				return err
			}
		}
		return err
	})
	if err != nil {
		return err
	}

	typ := a.GetType()
	switch {
	case vocab.ActivityTypes.Match(typ):
		err = vocab.OnActivity(a, func(act *vocab.Activity) error {
			// TODO(marius): this needs to be extended by a ValidateActivityClientObject
			//   because the first step would be to test the object in the context of the activity
			//   The ValidateActivityClientObject could then validate just the object itself.
			if act.Object, err = p.ValidateClientObject(act.Object); err != nil {
				return err
			}

			switch {
			case vocab.ContentManagementActivityTypes.Match(typ) && !vocab.RelationshipType.Match(act.Object.GetType()):
				err = p.ValidateClientContentManagementActivity(act)
			case vocab.ReactionsActivityTypes.Match(typ):
				err = p.ValidateClientReactionsActivity(act)
			case vocab.NotificationActivityTypes.Match(typ):
				err = p.ValidateClientNotificationActivity(act)
			case vocab.RelationshipManagementActivityTypes.Match(typ):
				err = ValidateClientRelationshipManagementActivity(p.s, act)
			case vocab.NegatingActivityTypes.Match(typ):
				err = p.ValidateClientNegatingActivity(act)
			}
			return err
		})
	}
	return err
}

// ValidateClientContentManagementActivity
func (p *P) ValidateClientContentManagementActivity(act *vocab.Activity) error {
	l := p.s
	if vocab.IsNil(act.Object) {
		return errors.BadRequestf("nil object for %v activity", act.Type)
	}

	validateObjectForChange := func(ob vocab.Item) error {
		if len(ob.GetLink()) == 0 {
			return errors.BadRequestf("empty object id for %v activity", act.Type)
		}
		if !vocab.IsObject(ob) {
			return nil
		}

		found, err := l.Load(ob.GetLink())
		if err != nil {
			return errors.Annotatef(err, "failed to load object from storage")
		}
		if vocab.IsNil(found) {
			return errors.NotFoundf("found nil object in storage")
		}
		// NOTE(marius): check if found object is modifiable for the current activity actor.
		return p.objectAuthCheckFn(act.Actor, found)
	}

	return vocab.OnItem(act.Object, func(ob vocab.Item) error {
		switch {
		case vocab.UpdateType.Match(act.Type):
			if vocab.ActivityTypes.Match(ob.GetType()) {
				return errors.BadRequestf("trying to update an immutable activity")
			}
			if err := validateObjectForChange(ob); err != nil {
				return err
			}
		case vocab.DeleteType.Match(act.Type):
			if err := validateObjectForChange(ob); err != nil {
				return err
			}
		case vocab.CreateType.Match(act.Type):
		default:
		}
		return nil
	})
}

// ValidateClientReactionsActivity
func (p *P) ValidateClientReactionsActivity(act *vocab.Activity) error {
	if act.Object != nil {
		switch {
		case vocab.DislikeType.Match(act.Type):
			fallthrough
		case vocab.LikeType.Match(act.Type):
			//return ValidateAppreciationActivity(l, act)
		case vocab.ActivityVocabularyTypes{vocab.RejectType, vocab.TentativeRejectType}.Match(act.Type):
			return p.ValidateClientRejectActivity(act)
		case vocab.ActivityVocabularyTypes{vocab.TentativeAcceptType, vocab.AcceptType}.Match(act.Type):
			return p.ValidateClientAcceptActivity(act)
		case vocab.BlockType.Match(act.Type):
			//return ValidateBlockActivity(l, act)
		case vocab.FlagType.Match(act.Type):
			//return ValidateFlagActivity(l, act)
		case vocab.IgnoreType.Match(act.Type):
			//return ValidateIgnoreActivity(l, act)
		}
	}

	return nil
}

func validateFollowup(actor vocab.Item, wantedType vocab.ActivityVocabularyType) func(*vocab.Activity) error {
	return func(follow *vocab.Activity) error {
		if !wantedType.Match(follow.GetType()) {
			return errors.BadRequestf("object Activity type %s is incorrect, expected %s", follow.Type, wantedType)
		}
		// NOTE(marius): we could have the original activity actor is actually multiple actors
		// so we use OnItem to iterate through them
		return vocab.OnItem(actor, func(actor vocab.Item) error {
			// NOTE(marius): we could have the original follow activity follow to have multiple objects,
			// so we use OnItem to iterate through them
			return vocab.OnItem(follow.Object, func(toFollow vocab.Item) error {
				if !toFollow.GetLink().Equals(actor.GetLink(), false) {
					return errors.BadRequestf(
						"The %s activity has a different actor than the received %s's object: %s, expected %s",
						wantedType, follow.Type,
						actor.GetLink(),
						toFollow.GetLink(),
					)
				}
				return nil
			})
		})
	}
}

// ValidateClientAcceptActivity
func (p *P) ValidateClientAcceptActivity(act *vocab.Activity) error {
	if err := p.ValidateAcceptActivity(act); err != nil {
		return err
	}
	if vocab.IsIRI(act.Object) {
		return nil
	}
	return vocab.OnActivity(act.Object, validateFollowup(act.Actor, vocab.FollowType))
}

// ValidateAcceptActivity
func (p *P) ValidateAcceptActivity(act *vocab.Activity) error {
	good := vocab.ActivityVocabularyTypes{vocab.AcceptType, vocab.TentativeAcceptType}
	if !good.Match(act.Type) {
		return errors.BadRequestf("Activity has wrong type %s, expected %v", act.Type, good)
	}
	return nil
}

// ValidateClientRejectActivity
func (p *P) ValidateClientRejectActivity(act *vocab.Activity) error {
	if err := p.ValidateRejectActivity(act); err != nil {
		return err
	}

	return vocab.OnActivity(act.Object, validateFollowup(act.Actor, vocab.FollowType))
}

// ValidateRejectActivity
func (p *P) ValidateRejectActivity(act *vocab.Activity) error {
	good := vocab.ActivityVocabularyTypes{vocab.RejectType, vocab.TentativeRejectType}
	if !good.Match(act.Type) {
		return errors.BadRequestf("Activity has wrong type %s, expected %v", act.Type, good)
	}
	return nil
}

// ValidateClientRelationshipManagementActivity
func ValidateClientRelationshipManagementActivity(l ReadStore, act *vocab.Activity) error {
	switch {
	case vocab.FollowType.Match(act.Type):
		if iri := act.GetLink(); len(iri) > 0 {
			if a, _ := l.Load(iri); !vocab.IsNil(firstOrItem(a)) {
				return errors.Conflictf("%s already exists for this actor/object pair", act.Type)
			}
		}
	case vocab.ActivityVocabularyTypes{vocab.AddType, vocab.BlockType, vocab.CreateType, vocab.DeleteType,
		vocab.IgnoreType, vocab.InviteType, vocab.AcceptType, vocab.RejectType}.Match(act.Type):
		// TODO(marius): either the actor or the object needs to be local for this action to be valid
		//   in the case of C2S... the actor needs to be local
		//   in the case of S2S... the object needs to be local
		// TODO(marius): Object needs to be a valid Follow activity
	default:
	}
	return nil
}

// IsLocal shows if the received IRI belongs to the current instance
func (p *P) IsLocal(i vocab.Item) bool {
	result := false
	_ = vocab.OnItem(i, func(i vocab.Item) error {
		result = p.validateLocalIRI(i.GetLink()) == nil || p.localIRICheckFn(i.GetLink())
		return nil
	})
	return result
}

// IsLocalIRI shows if the received IRI belongs to the current instance
func (p *P) IsLocalIRI(i vocab.IRI) bool {
	return p.validateLocalIRI(i) == nil || p.localIRICheckFn(i)
}

func (p *P) ValidateClientActor(a vocab.Item, expected vocab.Actor) (vocab.Item, error) {
	if vocab.IsNil(a) {
		return a, InvalidActivityActor("is nil")
	}

	// NOTE(marius): we use OnItem for the cases where the received
	// actor is actually a slice of actors
	err := vocab.OnItem(a, func(item vocab.Item) error {
		if !p.IsLocal(item.GetLink()) {
			return errors.Newf("%s is not a local IRI", a.GetLink())
		}
		return nil
	})
	if err != nil {
		return a, InvalidActivityActor("%s is not local", a.GetLink())
	}
	return p.ValidateActor(a, expected)
}

func (p *P) ValidateIRI(i vocab.IRI) error {
	if i == "" {
		return InvalidIRI("empty")
	}
	if i.Equals(vocab.PublicNS, false) {
		return InvalidIRI("Public namespace is not a valid IRI")
	}
	if _, err := i.URL(); err != nil {
		return errors.Annotatef(err, "URL could not be parsed: %s", i)
	}
	return nil
}

func (p *P) ValidateServerActor(a vocab.Item, expected vocab.Actor) (vocab.Item, error) {
	return p.ValidateActor(a, expected)
}

func (p *P) ValidateActor(a vocab.Item, expected vocab.Actor) (vocab.Item, error) {
	if vocab.IsNil(a) {
		return a, InvalidActivityActor("is nil")
	}

	var err error
	if a, err = p.DereferenceItem(a); err != nil {
		return a, errors.NewBadRequest(err, "unable to dereference Activity Actor")
	}

	// NOTE(marius): we check that among the actors of the activity we have at least one that
	// matches the one we extracted from the authorization mechanism.
	foundExpected := false
	err = vocab.OnActor(a, func(act *vocab.Actor) error {
		if typ := act.GetType(); !vocab.ActorTypes.Match(typ) {
			return InvalidActivityActor("invalid type %v", typ)
		}
		if !foundExpected {
			foundExpected = act.ID.Equal(expected.GetLink())
		}
		return nil
	})
	if err == nil && !foundExpected {
		err = InvalidActivityActor("the actor doesn't match the authenticated one")
	}
	return a, err
}

func (p *P) ValidateClientObject(o vocab.Item) (vocab.Item, error) {
	objects := make(vocab.ItemCollection, 0, 2)
	err := vocab.OnItem(o, func(it vocab.Item) error {
		if vocab.IsNil(it) {
			return InvalidActivityObject("is nil")
		}
		var err error
		if it, err = p.DereferenceItem(it); err != nil {
			return errors.NewBadRequest(err, "unable to dereference Activity Object")
		}
		if vocab.IsIRI(it) {
			return errors.BadRequestf("unable to dereference Activity Object: %s", it)
		}
		return objects.Append(it)
	})
	return objects.Normalize(), err
}

func (p *P) ValidateServerObject(o vocab.Item) (vocab.Item, error) {
	if vocab.IsNil(o) {
		return o, InvalidActivityObject("is nil")
	}
	err := vocab.OnItem(o, func(it vocab.Item) error {
		if vocab.IsIRI(it) {
			return p.ValidateIRI(it.GetID())
		}
		return nil
	})
	if err != nil {
		return nil, errors.NewBadRequest(err, "invalid Object")
	}
	return firstOrItem(o), nil
}

func (p *P) ValidateAudienceForRemoteActivity(audience ...vocab.ItemCollection) error {
	for _, elem := range audience {
		for _, iri := range elem {
			if p.IsLocal(iri.GetLink()) || iri.GetLink().Equal(vocab.PublicNS) {
				return nil
			}
		}
	}
	return errors.Newf("None of the audience elements is local")
}

func (p *P) validateLocalIRI(i vocab.IRI) error {
	if len(p.baseIRI) > 0 {
		for _, base := range p.baseIRI {
			if i.Contains(base, false) {
				return nil
			}
		}
		return errors.Newf("%s is not a local IRI", i)
	}
	return errors.Newf("%s is not a local IRI", i)
}

func (p *P) ValidateServerContentManagementActivity(act *vocab.Activity) error {
	if vocab.IsNil(act.Object) {
		return errors.BadRequestf("nil object for %v activity", act.Type)
	}

	if !(vocab.UpdateType.Match(act.Type) || vocab.DeleteType.Match(act.Type)) {
		return nil
	}

	return vocab.OnItem(act.Object, func(ob vocab.Item) error {
		if vocab.ActivityTypes.Match(ob.GetType()) {
			return errors.BadRequestf("trying to modify an immutable activity")
		}

		// NOTE(marius): Update activities should always have an embedded replacement copy
		if vocab.IsIRI(ob) && vocab.UpdateType.Match(act.Type) {
			return errors.BadRequestf("unable to process IRI object for %v activity", act.Type)
		}

		if len(ob.GetLink()) == 0 {
			return errors.BadRequestf("empty object id for %v activity", act.Type)
		}

		found, err := p.DereferenceItem(ob.GetLink())
		if err != nil || vocab.IsNil(found) || vocab.IsIRI(found) {
			return errors.Annotatef(err, "failed to dereference object")
		}
		// NOTE(marius): check if found object is modifiable for the current activity actor.
		if err = p.objectAuthCheckFn(act.Actor, found); err != nil {
			return err
		}
		ob = found
		return nil
	})
}

func (p *P) ValidateServerCollectionManagementActivity(act *vocab.Activity) (err error) {
	if !vocab.IsNil(act.Target) {
		// NOTE(marius): the target is either a local object, and we need to
		// make sure the actor is allowed to operate on it, or is a remote object
		// and we don't need to do any local processing.
		target, err := p.ValidateClientObject(act.Target)
		if err != nil {
			if target, err = p.ValidateServerObject(act.Target); err != nil {
				return err
			}
		}
		act.Target = target
	}
	if !vocab.IsNil(act.Origin) {
		if act.Origin, err = p.ValidateServerObject(act.Origin); err != nil {
			return err
		}
	}

	checkForActorPermissions := func(ob vocab.Item) error {
		if len(ob.GetLink()) == 0 {
			return errors.BadRequestf("empty object id for %v activity", act.Type)
		}

		found, err := p.DereferenceItem(ob.GetLink())
		if err != nil || vocab.IsNil(found) || vocab.IsIRI(found) {
			return errors.Annotatef(err, "failed to dereference object")
		}
		// NOTE(marius): check if found object is modifiable for the current activity actor.
		if err = p.objectAuthCheckFn(act.Actor, found); err != nil {
			return err
		}
		ob = found
		return nil
	}

	typ := act.Type
	switch {
	case vocab.AddType.Match(typ):
		return vocab.OnItem(act.Target, checkForActorPermissions)
	case vocab.RemoveType.Match(typ):
		return vocab.OnItem(act.Target, checkForActorPermissions)
	case vocab.MoveType.Match(typ):
		if err = vocab.OnItem(act.Target, checkForActorPermissions); err != nil {
			return err
		}
		if err = vocab.OnItem(act.Origin, checkForActorPermissions); err != nil {
			return err
		}
	}
	return nil
}
