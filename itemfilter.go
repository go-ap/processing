package storage

import (
	"github.com/go-ap/activitypub"
	pub "github.com/go-ap/activitypub"
)

type itemFilter struct {
	item pub.Item
}

func FilterItem(i pub.Item) itemFilter {
	return itemFilter{ item: i}
}

func (i itemFilter) GetLink() pub.IRI {
	return i.item.GetLink()
}

func (i itemFilter) Types() pub.ActivityVocabularyTypes {
	return pub.ActivityVocabularyTypes{i.item.GetType()}
}

func (i itemFilter) IRIs() pub.IRIs {
	iri := i.item.GetLink()
	if len(iri) > 0 {
		return pub.IRIs{iri}
	}
	return nil
}
func (i itemFilter) Actors() pub.IRIs {
	iris := make(pub.IRIs, 0)
	if pub.ActivityTypes.Contains(i.item.GetType()) {
		activitypub.OnActivity(i.item, func(a *pub.Activity) error {
			iris = append(iris, a.Actor.GetLink())
			return nil
		})
	}
	if pub.IntransitiveActivityTypes.Contains(i.item.GetType()) {
		activitypub.OnIntransitiveActivity(i.item, func(a *pub.IntransitiveActivity) error {
			iris = append(iris, a.Actor.GetLink())
			return nil
		})
	}
	return iris
}
func (i itemFilter) Objects() pub.IRIs {
	iris := make(pub.IRIs, 0)
	if pub.ActivityTypes.Contains(i.item.GetType()) {
		activitypub.OnActivity(i.item, func(a *pub.Activity) error {
			iris = append(iris, a.Object.GetLink())
			return nil
		})
	}
	return iris
}

func (i itemFilter) Targets() pub.IRIs {
	iris := make(pub.IRIs, 0)
	if pub.ActivityTypes.Contains(i.item.GetType()) {
		activitypub.OnActivity(i.item, func(a *pub.Activity) error {
			iris = append(iris, a.Target.GetLink())
			return nil
		})
	}
	if pub.IntransitiveActivityTypes.Contains(i.item.GetType()) {
		activitypub.OnIntransitiveActivity(i.item, func(a *pub.IntransitiveActivity) error {
			iris = append(iris, a.Target.GetLink())
			return nil
		})
	}
	return iris
}

func (i itemFilter) AttributedTo() pub.IRIs {
	iris := make(pub.IRIs, 0)
	if pub.ObjectTypes.Contains(i.item.GetType()) {
		activitypub.OnObject(i.item, func(o *activitypub.Object) error {
			iris = append(iris, o.AttributedTo.GetLink())
			return nil
		})
	}
	return iris
}
func (i itemFilter) InReplyTo() pub.IRIs {
	iris := make(pub.IRIs, 0)
	if pub.ObjectTypes.Contains(i.item.GetType()) {
		activitypub.OnObject(i.item, func(o *activitypub.Object) error {
			iris = append(iris, o.InReplyTo.GetLink())
			return nil
		})
	}
	return iris
}
func (i itemFilter) MediaTypes() []pub.MimeType {
	types := make([]pub.MimeType, 0)
	if pub.ObjectTypes.Contains(i.item.GetType()) {
		activitypub.OnObject(i.item, func(o *activitypub.Object) error {
			types = append(types, o.MediaType)
			return nil
		})
	}
	return types
}
func (i itemFilter) Names() []string {
	names := make([]string, 0)
	if pub.ActivityTypes.Contains(i.item.GetType()) {
		activitypub.OnActivity(i.item, func(a *pub.Activity) error {
			for _, name := range a.Name {
				names = append(names, name.Value)
			}
			return nil
		})
	}
	if pub.ObjectTypes.Contains(i.item.GetType()) {
		activitypub.OnObject(i.item, func(o *activitypub.Object) error {
			for _, name := range o.Name {
				names = append(names, name.Value)
			}
			return nil
		})
	}
	if pub.ActivityTypes.Contains(i.item.GetType()) {
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
func (i itemFilter) URLs() pub.IRIs {
	iris := make(pub.IRIs, 0)
	activitypub.OnObject(i.item, func(o *activitypub.Object) error {
		iris = append(iris, o.URL.GetLink())
		return nil
	})
	return iris
}
func (i itemFilter) Audience() pub.IRIs {
	iris := make(pub.IRIs, 0)
	activitypub.OnObject(i.item, func(o *activitypub.Object) error {
		iris = append(iris, o.Audience.GetLink())
		return nil
	})
	return iris
}
func (i itemFilter) Context() pub.IRIs {
	iris := make(pub.IRIs, 0)
	activitypub.OnObject(i.item, func(o *activitypub.Object) error {
		iris = append(iris, o.Context.GetLink())
		return nil
	})
	return iris
}
