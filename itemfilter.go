package storage

import (
	"github.com/go-ap/activitypub"
	as "github.com/go-ap/activitystreams"
)

type itemFilter struct {
	item as.Item
}

func FilterItem(i as.Item) itemFilter {
	return itemFilter{ item: i}
}

func (i itemFilter) GetLink() as.IRI {
	return i.item.GetLink()
}

func (i itemFilter) Types() as.ActivityVocabularyTypes {
	return as.ActivityVocabularyTypes{i.item.GetType()}
}

func (i itemFilter) IRIs() as.IRIs {
	iri := i.item.GetLink()
	if len(iri) > 0 {
		return as.IRIs{iri}
	}
	return nil
}
func (i itemFilter) Actors() as.IRIs {
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
func (i itemFilter) Objects() as.IRIs {
	iris := make(as.IRIs, 0)
	if as.ActivityTypes.Contains(i.item.GetType()) {
		activitypub.OnActivity(i.item, func(a *as.Activity) error {
			iris = append(iris, a.Object.GetLink())
			return nil
		})
	}
	return iris
}

func (i itemFilter) Targets() as.IRIs {
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

func (i itemFilter) AttributedTo() as.IRIs {
	iris := make(as.IRIs, 0)
	if as.ObjectTypes.Contains(i.item.GetType()) {
		activitypub.OnObject(i.item, func(o *activitypub.Object) error {
			iris = append(iris, o.AttributedTo.GetLink())
			return nil
		})
	}
	return iris
}
func (i itemFilter) InReplyTo() as.IRIs {
	iris := make(as.IRIs, 0)
	if as.ObjectTypes.Contains(i.item.GetType()) {
		activitypub.OnObject(i.item, func(o *activitypub.Object) error {
			iris = append(iris, o.InReplyTo.GetLink())
			return nil
		})
	}
	return iris
}
func (i itemFilter) MediaTypes() []as.MimeType {
	types := make([]as.MimeType, 0)
	if as.ObjectTypes.Contains(i.item.GetType()) {
		activitypub.OnObject(i.item, func(o *activitypub.Object) error {
			types = append(types, o.MediaType)
			return nil
		})
	}
	return types
}
func (i itemFilter) Names() []string {
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
func (i itemFilter) URLs() as.IRIs {
	iris := make(as.IRIs, 0)
	activitypub.OnObject(i.item, func(o *activitypub.Object) error {
		iris = append(iris, o.URL.GetLink())
		return nil
	})
	return iris
}
func (i itemFilter) Audience() as.IRIs {
	iris := make(as.IRIs, 0)
	activitypub.OnObject(i.item, func(o *activitypub.Object) error {
		iris = append(iris, o.Audience.GetLink())
		return nil
	})
	return iris
}
func (i itemFilter) Context() as.IRIs {
	iris := make(as.IRIs, 0)
	activitypub.OnObject(i.item, func(o *activitypub.Object) error {
		iris = append(iris, o.Context.GetLink())
		return nil
	})
	return iris
}
