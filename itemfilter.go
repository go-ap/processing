package processing

import (
	vocab "github.com/go-ap/activitypub"
)

type itemFilter struct {
	item vocab.Item
}

func FilterItem(i vocab.Item) itemFilter {
	return itemFilter{item: i}
}

func (i itemFilter) GetLink() vocab.IRI {
	return i.item.GetLink()
}

func (i itemFilter) Types() vocab.ActivityVocabularyTypes {
	return vocab.ActivityVocabularyTypes{i.item.GetType()}
}

func (i itemFilter) IRIs() vocab.IRIs {
	iri := i.item.GetLink()
	if len(iri) > 0 {
		return vocab.IRIs{iri}
	}
	return nil
}

func (i itemFilter) Actors() vocab.IRIs {
	iris := make(vocab.IRIs, 0)
	if vocab.ActivityTypes.Contains(i.item.GetType()) {
		_ = vocab.OnActivity(i.item, func(a *vocab.Activity) error {
			iris = append(iris, a.Actor.GetLink())
			return nil
		})
	}
	if vocab.IntransitiveActivityTypes.Contains(i.item.GetType()) {
		_ = vocab.OnIntransitiveActivity(i.item, func(a *vocab.IntransitiveActivity) error {
			iris = append(iris, a.Actor.GetLink())
			return nil
		})
	}
	return iris
}

func (i itemFilter) Objects() vocab.IRIs {
	iris := make(vocab.IRIs, 0)
	if vocab.ActivityTypes.Contains(i.item.GetType()) {
		_ = vocab.OnActivity(i.item, func(a *vocab.Activity) error {
			iris = append(iris, a.Object.GetLink())
			return nil
		})
	}
	return iris
}

func (i itemFilter) Targets() vocab.IRIs {
	iris := make(vocab.IRIs, 0)
	if vocab.ActivityTypes.Contains(i.item.GetType()) {
		_ = vocab.OnActivity(i.item, func(a *vocab.Activity) error {
			iris = append(iris, a.Target.GetLink())
			return nil
		})
	}
	if vocab.IntransitiveActivityTypes.Contains(i.item.GetType()) {
		_ = vocab.OnIntransitiveActivity(i.item, func(a *vocab.IntransitiveActivity) error {
			iris = append(iris, a.Target.GetLink())
			return nil
		})
	}
	return iris
}

func (i itemFilter) AttributedTo() vocab.IRIs {
	iris := make(vocab.IRIs, 0)
	if vocab.ObjectTypes.Contains(i.item.GetType()) {
		_ = vocab.OnObject(i.item, func(o *vocab.Object) error {
			iris = append(iris, o.AttributedTo.GetLink())
			return nil
		})
	}
	return iris
}

func (i itemFilter) InReplyTo() vocab.IRIs {
	iris := make(vocab.IRIs, 0)
	if vocab.ObjectTypes.Contains(i.item.GetType()) {
		vocab.OnObject(i.item, func(o *vocab.Object) error {
			iris = append(iris, o.InReplyTo.GetLink())
			return nil
		})
	}
	return iris
}

func (i itemFilter) MediaTypes() []vocab.MimeType {
	types := make([]vocab.MimeType, 0)
	if vocab.ObjectTypes.Contains(i.item.GetType()) {
		vocab.OnObject(i.item, func(o *vocab.Object) error {
			types = append(types, o.MediaType)
			return nil
		})
	}
	return types
}

func (i itemFilter) Names() []vocab.Content {
	names := make([]vocab.Content, 0)
	if vocab.ActivityTypes.Contains(i.item.GetType()) {
		_ = vocab.OnActivity(i.item, func(a *vocab.Activity) error {
			for _, name := range a.Name {
				names = append(names, name)
			}
			return nil
		})
	}
	if vocab.ObjectTypes.Contains(i.item.GetType()) {
		_ = vocab.OnObject(i.item, func(o *vocab.Object) error {
			for _, name := range o.Name {
				names = append(names, name)
			}
			return nil
		})
	}
	if vocab.ActivityTypes.Contains(i.item.GetType()) {
		_ = vocab.OnActor(i.item, func(p *vocab.Actor) error {
			for _, name := range p.Name {
				names = append(names, name)
			}
			for _, name := range p.PreferredUsername {
				names = append(names, name)
			}
			return nil
		})
	}
	return names
}

func (i itemFilter) URLs() vocab.IRIs {
	iris := make(vocab.IRIs, 0)
	_ = vocab.OnObject(i.item, func(o *vocab.Object) error {
		iris = append(iris, o.URL.GetLink())
		return nil
	})
	return iris
}

func (i itemFilter) Audience() vocab.IRIs {
	iris := make(vocab.IRIs, 0)
	_ = vocab.OnObject(i.item, func(o *vocab.Object) error {
		iris = append(iris, o.Audience.GetLink())
		return nil
	})
	return iris
}

func (i itemFilter) Context() vocab.IRIs {
	iris := make(vocab.IRIs, 0)
	_ = vocab.OnObject(i.item, func(o *vocab.Object) error {
		iris = append(iris, o.Context.GetLink())
		return nil
	})
	return iris
}

func (i itemFilter) Generator() vocab.IRIs {
	iris := make(vocab.IRIs, 0)
	_ = vocab.OnObject(i.item, func(o *vocab.Object) error {
		iris = append(iris, o.Generator.GetLink())
		return nil
	})
	return iris
}
