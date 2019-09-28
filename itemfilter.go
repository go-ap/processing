package storage

import (
	"github.com/go-ap/activitypub"
	as "github.com/go-ap/activitystreams"
)

type ItemFilter struct {
	item as.Item
}

func (i ItemFilter) GetLink() as.IRI {
	return i.item.GetLink()
}

func (i ItemFilter) Types() as.ActivityVocabularyTypes {
	return as.ActivityVocabularyTypes{i.item.GetType()}
}

func (i ItemFilter) IRIs() as.IRIs {
	return as.IRIs{i.item.GetLink()}
}
func (i ItemFilter) Actors() as.IRIs {
	iris := make(as.IRIs, 0)
	if as.ActivityTypes.Contains(i.item.GetType()) {
		activitypub.OnActivity(i.item, func(a *as.Activity) error {
			iris = append(iris, a.Actor.GetLink())
			return nil
		})
	}
	if as.IntransitiveActivityTypes.Contains(i.item.GetType()) {
		activitypub.OnIntransitiveActivity(i.item, func(a *as.IntransitiveActivity) error {
			iris = append(iris, a.Actor.GetLink())
			return nil
		})
	}
	return iris
}
func (i ItemFilter) Objects() as.IRIs {
	iris := make(as.IRIs, 0)
	if as.ActivityTypes.Contains(i.item.GetType()) {
		activitypub.OnActivity(i.item, func(a *as.Activity) error {
			iris = append(iris, a.Object.GetLink())
			return nil
		})
	}
	return iris
}

func (i ItemFilter) Targets() as.IRIs {
	iris := make(as.IRIs, 0)
	if as.ActivityTypes.Contains(i.item.GetType()) {
		activitypub.OnActivity(i.item, func(a *as.Activity) error {
			iris = append(iris, a.Target.GetLink())
			return nil
		})
	}
	if as.IntransitiveActivityTypes.Contains(i.item.GetType()) {
		activitypub.OnIntransitiveActivity(i.item, func(a *as.IntransitiveActivity) error {
			iris = append(iris, a.Target.GetLink())
			return nil
		})
	}
	return iris
}

func (i ItemFilter) AttributedTo() as.IRIs {
	iris := make(as.IRIs, 0)
	if as.ObjectTypes.Contains(i.item.GetType()) {
		activitypub.OnObject(i.item, func(o *activitypub.Object) error {
			iris = append(iris, o.AttributedTo.GetLink())
			return nil
		})
	}
	return iris
}
func (i ItemFilter) InReplyTo() as.IRIs {
	iris := make(as.IRIs, 0)
	if as.ObjectTypes.Contains(i.item.GetType()) {
		activitypub.OnObject(i.item, func(o *activitypub.Object) error {
			iris = append(iris, o.InReplyTo.GetLink())
			return nil
		})
	}
	return iris
}
func (i ItemFilter) MediaTypes() []as.MimeType {
	types := make([]as.MimeType, 0)
	if as.ObjectTypes.Contains(i.item.GetType()) {
		activitypub.OnObject(i.item, func(o *activitypub.Object) error {
			types = append(types, o.MediaType)
			return nil
		})
	}
	return types
}
func (i ItemFilter) Names() []string {
	names := make([]string, 0)
	if as.ActivityTypes.Contains(i.item.GetType()) {
		activitypub.OnActivity(i.item, func(a *as.Activity) error {
			for _, name := range a.Name {
				names = append(names, name.Value)
			}
			return nil
		})
	}
	if as.ObjectTypes.Contains(i.item.GetType()) {
		activitypub.OnObject(i.item, func(o *activitypub.Object) error {
			for _, name := range o.Name {
				names = append(names, name.Value)
			}
			return nil
		})
	}
	if as.ActivityTypes.Contains(i.item.GetType()) {
		activitypub.OnPerson(i.item, func(p *activitypub.Person) error {
			for _, name := range p.Name {
				names = append(names, name.Value)
			}
			for _, name := range p.PreferredUsername {
				names = append(names, name.Value)
			}
			return nil
		})
	}
	return names
}
func (i ItemFilter) URLs() as.IRIs {
	iris := make(as.IRIs, 0)
	activitypub.OnObject(i.item, func(o *activitypub.Object) error {
		iris = append(iris, o.URL.GetLink())
		return nil
	})
	return iris
}
func (i ItemFilter) Audience() as.IRIs {
	iris := make(as.IRIs, 0)
	activitypub.OnObject(i.item, func(o *activitypub.Object) error {
		iris = append(iris, o.Audience.GetLink())
		return nil
	})
	return iris
}
func (i ItemFilter) Context() as.IRIs {
	iris := make(as.IRIs, 0)
	activitypub.OnObject(i.item, func(o *activitypub.Object) error {
		iris = append(iris, o.Context.GetLink())
		return nil
	})
	return iris
}
