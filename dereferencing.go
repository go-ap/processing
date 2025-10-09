package processing

import (
	"git.sr.ht/~mariusor/lw"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// dereferenceIRI checks if the received IRI is local or remote, and it tries to load it accordingly.
// If it's local it tries to load it from storage, and if remote it loads using the ActivityPub client.
//
// If the IRI can't be loaded we return it together with an error that can be logged.
func (p P) dereferenceIRI(iri vocab.IRI) (maybeFull vocab.Item, err error) {
	maybeFull, err = p.s.Load(iri)
	if err != nil {
		err = errors.Annotatef(err, "unable to load IRI from local storage")
	}
	if !p.IsLocalIRI(iri) && vocab.IsNil(maybeFull) {
		if maybeFull, err = p.c.LoadIRI(iri); err != nil {
			err = errors.Annotatef(err, "unable to fetch remote IRI")
		}
	}
	if vocab.IsNil(maybeFull) {
		maybeFull = iri
	}
	return maybeFull, err
}

// aggregateIRIs works as a filter for the collection passed to the returning function.
// It splits them into two slices, one of IRIs to dereference and one corresponding to the received collection which
// gets copied to the slice to keep.
//
// The purpose is to allow the calling code to dereference the IRIs, and then replace them in the toKeep collection
// transparently.
func aggregateIRIs(toDeref *vocab.IRIs, toKeep *vocab.ItemCollection) func(col vocab.CollectionInterface) error {
	return func(col vocab.CollectionInterface) error {
		for _, it := range col.Collection() {
			switch {
			case vocab.IsNil(it):
				continue
			case vocab.IsIRI(it):
				_ = toKeep.Append(it.GetLink())
				_ = toDeref.Append(it.GetLink())
			case vocab.IsItemCollection(it):
				if err := vocab.OnCollectionIntf(it, aggregateIRIs(toDeref, toKeep)); err != nil {
					return errors.Annotatef(err, "unable to dereference items in %T", it)
				}
			case vocab.IsObject(it), vocab.IsLink(it):
				_ = toKeep.Append(it)
			}
		}
		return nil
	}
}

// DereferenceItem checks if the received argument needs dereferencing, or normalization in the case of collections.
// It can be used in the calling code to ensure that before operating on an item, we dereference and de-normalize it
// to an ActivityPub object, or slice of dereferenced objects.
//
// The method does nothing when encountering IRIs which can not be dereferenced.
// We would normally log these entries but otherwise leave them alone.
func (p P) DereferenceItem(it vocab.Item) (vocab.Item, error) {
	toDeref := make(vocab.IRIs, 0)
	toKeep := make(vocab.ItemCollection, 0)

	switch {
	case vocab.IsNil(it):
		return it, errors.NotValidf("unable to dereference nil item")
	case vocab.IsObject(it), vocab.IsLink(it):
		return it, nil
	case vocab.IsIRI(it):
		_ = toDeref.Append(it.GetLink())
		_ = toKeep.Append(it.GetLink())
	case vocab.IsIRIs(it), vocab.IsItemCollection(it):
		if err := vocab.OnCollectionIntf(it, aggregateIRIs(&toDeref, &toKeep)); err != nil {
			return it, err
		}
	}

	if toDeref.Count() > 0 {
		for i, iri := range toDeref {
			full, err := p.dereferenceIRI(iri)
			if err != nil {
				p.l.WithContext(lw.Ctx{"iri": iri, "err": err.Error()}).Warnf("error dereferencing IRI")
			}
			if !vocab.IsNil(full) {
				// NOTE(marius): we rely on the fact that we have the same cursor on both toKeep and toDereference slices
				// referring to the same items/IRIs, so we can easily replace them.
				toKeep[i] = full
			}
		}
	}
	if cnt := toKeep.Count(); cnt > 0 {
		if cnt == 1 {
			return firstOrItem(toKeep), nil
		}
		return toKeep, nil
	}
	return it, nil
}
