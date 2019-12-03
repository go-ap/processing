package processing

import (
	pub "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// CopyObjectProperties updates the "old" object properties with "new's"
func CopyObjectProperties(old, new *pub.Object) (*pub.Object, error) {
	old.Name = replaceIfNaturalLanguageValues(old.Name, new.Name)
	old.Attachment = replaceIfItem(old.Attachment, new.Attachment)
	old.AttributedTo = replaceIfItem(old.AttributedTo, new.AttributedTo)
	old.Audience = replaceIfItemCollection(old.Audience, new.Audience)
	old.Content = replaceIfNaturalLanguageValues(old.Content, new.Content)
	old.Context = replaceIfItem(old.Context, new.Context)
	if len(new.MediaType) > 0 {
		old.MediaType = new.MediaType
	}
	if !new.EndTime.IsZero() {
		old.EndTime = new.EndTime
	}
	old.Generator = replaceIfItem(old.Generator, new.Generator)
	old.Icon = replaceIfItem(old.Icon, new.Icon)
	old.Image = replaceIfItem(old.Image, new.Image)
	old.InReplyTo = replaceIfItem(old.InReplyTo, new.InReplyTo)
	old.Location = replaceIfItem(old.Location, new.Location)
	old.Preview = replaceIfItem(old.Preview, new.Preview)
	if old.Published.IsZero() && !new.Published.IsZero() {
		old.Published = new.Published
	}
	old.Replies = replaceIfItem(old.Replies, new.Replies)
	if !new.StartTime.IsZero() {
		old.StartTime = new.StartTime
	}
	old.Summary = replaceIfNaturalLanguageValues(old.Summary, new.Summary)
	old.Tag = replaceIfItemCollection(old.Tag, new.Tag)
	if !new.Updated.IsZero() {
		old.Updated = new.Updated
	}
	if new.URL != nil {
		old.URL = new.URL
	}
	old.To = replaceIfItemCollection(old.To, new.To)
	old.Bto = replaceIfItemCollection(old.Bto, new.Bto)
	old.CC = replaceIfItemCollection(old.CC, new.CC)
	old.BCC = replaceIfItemCollection(old.BCC, new.BCC)
	if new.Duration == 0 {
		old.Duration = new.Duration
	}
	old.Source = replaceIfSource(old.Source, new.Source)
	return old, nil
}

// CopyItemProperties delegates to the correct per type functions for copying
// properties between matching Activity Objects
func CopyItemProperties(to, from pub.Item) (pub.Item, error) {
	if to == nil {
		return to, errors.Newf("Nil object to update")
	}
	if from == nil {
		return to, errors.Newf("Nil object for update")
	}
	if to.GetID() != from.GetID() {
		return to, errors.Newf("Object IDs don't match")
	}
	if to.GetType() != from.GetType() {
		return to, errors.Newf("Invalid object types for update %s(old) and %s(new)", from.GetType(), to.GetType())
	}
	if pub.ActorTypes.Contains(to.GetType()) {
		o, err := pub.ToActor(to)
		if err != nil {
			return o, err
		}
		n, err := pub.ToActor(from)
		if err != nil {
			return o, err
		}
		return UpdatePersonProperties(o, n)
	}
	if pub.ObjectTypes.Contains(to.GetType()) {
		o, err := pub.ToObject(to)
		if err != nil {
			return o, err
		}
		n, err := pub.ToObject(from)
		if err != nil {
			return o, err
		}
		return CopyObjectProperties(o, n)
	}
	return to, errors.Newf("could not process objects with type %s", to.GetType())
}

// UpdatePersonProperties
func UpdatePersonProperties(old, new *pub.Actor) (*pub.Actor, error) {
	old.Name = replaceIfNaturalLanguageValues(old.Name, new.Name)
	old.Attachment = replaceIfItem(old.Attachment, new.Attachment)
	old.AttributedTo = replaceIfItem(old.AttributedTo, new.AttributedTo)
	old.Audience = replaceIfItemCollection(old.Audience, new.Audience)
	old.Content = replaceIfNaturalLanguageValues(old.Content, new.Content)
	old.Context = replaceIfItem(old.Context, new.Context)
	if len(new.MediaType) > 0 {
		old.MediaType = new.MediaType
	}
	if !new.EndTime.IsZero() {
		old.EndTime = new.EndTime
	}
	old.Generator = replaceIfItem(old.Generator, new.Generator)
	old.Icon = replaceIfItem(old.Icon, new.Icon)
	old.Image = replaceIfItem(old.Image, new.Image)
	old.InReplyTo = replaceIfItem(old.InReplyTo, new.InReplyTo)
	old.Location = replaceIfItem(old.Location, new.Location)
	old.Preview = replaceIfItem(old.Preview, new.Preview)
	if old.Published.IsZero() && !new.Published.IsZero() {
		old.Published = new.Published
	}
	old.Replies = replaceIfItem(old.Replies, new.Replies)
	if !new.StartTime.IsZero() {
		old.StartTime = new.StartTime
	}
	old.Summary = replaceIfNaturalLanguageValues(old.Summary, new.Summary)
	old.Tag = replaceIfItemCollection(old.Tag, new.Tag)
	if !new.Updated.IsZero() {
		old.Updated = new.Updated
	}
	if new.URL != nil {
		old.URL = new.URL
	}
	old.To = replaceIfItemCollection(old.To, new.To)
	old.Bto = replaceIfItemCollection(old.Bto, new.Bto)
	old.CC = replaceIfItemCollection(old.CC, new.CC)
	old.BCC = replaceIfItemCollection(old.BCC, new.BCC)
	if new.Duration == 0 {
		old.Duration = new.Duration
	}
	old.Source = replaceIfSource(old.Source, new.Source)
	old.Inbox = replaceIfItem(old.Inbox, new.Inbox)
	old.Outbox = replaceIfItem(old.Outbox, new.Outbox)
	old.Following = replaceIfItem(old.Following, new.Following)
	old.Followers = replaceIfItem(old.Followers, new.Followers)
	old.Liked = replaceIfItem(old.Liked, new.Liked)
	old.PreferredUsername = replaceIfNaturalLanguageValues(old.PreferredUsername, new.PreferredUsername)
	return old, nil
}

func replaceIfItem(old, new pub.Item) pub.Item {
	if new == nil {
		return old
	}
	return new
}

func replaceIfItemCollection(old, new pub.ItemCollection) pub.ItemCollection {
	if new == nil {
		return old
	}
	return new
}

func replaceIfNaturalLanguageValues(old, new pub.NaturalLanguageValues) pub.NaturalLanguageValues {
	if new == nil {
		return old
	}
	return new
}

func replaceIfSource(old, new pub.Source) pub.Source {
	if new.MediaType != old.MediaType {
		return new
	}
	old.Content = replaceIfNaturalLanguageValues(old.Content, new.Content)
	return old
}
