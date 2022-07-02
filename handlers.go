package processing

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	json "github.com/go-ap/jsonld"
)

// CtxtKey type alias for the key under which we're storing the Collection Storage in the Request's context
type CtxtKey string

var RepositoryKey = CtxtKey("__repo")

// Typer is the static package variable that determines a CollectionPath type for a particular request
// It can be overloaded from outside packages.
// @TODO(marius): This should be moved as a property on an instantiable package object, instead of keeping it here
var Typer CollectionTyper = pathTyper{}

// CollectionTyper allows external packages to tell us which CollectionPath the current HTTP request addresses
type CollectionTyper interface {
	Type(r *http.Request) vocab.CollectionPath
}

type pathTyper struct{}

func (d pathTyper) Type(r *http.Request) vocab.CollectionPath {
	if r.URL == nil || len(r.URL.Path) == 0 {
		return vocab.Unknown
	}
	col := vocab.Unknown
	pathElements := strings.Split(r.URL.Path[1:], "/") // Skip first /
	for i := len(pathElements) - 1; i >= 0; i-- {
		col = vocab.CollectionPath(pathElements[i])
		if vocab.ValidObjectCollection(col) || vocab.ValidActivityCollection(col) {
			return col
		}
	}

	return col
}

// MethodValidator is the interface need to be implemented to specify if an HTTP request's method
// is supported by the implementor object
type MethodValidator interface {
	ValidMethod(r *http.Request) bool
}

// RequestValidator is the interface need to be implemented to specify if the whole HTTP request
// is valid in the context of the implementor object
type RequestValidator interface {
	ValidateRequest(r *http.Request) (int, error)
}

// ActivityHandlerFn is the type that we're using to represent handlers that process requests containing
// an ActivityStreams Activity. It needs to implement the http.Handler interface.
//
// It is considered that following the execution of the handler, we return a pair formed of a HTTP status together with
//  an IRI representing a new Object - in the case of transitive activities that had a side effect, or
//  an error.
// In the case of intransitive activities the iri will always be empty.
type ActivityHandlerFn func(vocab.CollectionPath, *http.Request, Store) (vocab.Item, int, error)

func (a ActivityHandlerFn) Storage(r *http.Request) (Store, error) {
	ctxVal := r.Context().Value(RepositoryKey)
	st, ok := ctxVal.(Store)
	if !ok {
		return nil, errors.Newf("Unable to find storage repository")
	}
	return st, nil
}

// ValidMethod validates if the current handler can process the current request
func (a ActivityHandlerFn) ValidMethod(r *http.Request) bool {
	return r.Method == http.MethodPost
}

// ValidateRequest validates if the current handler can process the current request
func (a ActivityHandlerFn) ValidateRequest(r *http.Request) (int, error) {
	if !a.ValidMethod(r) {
		return http.StatusNotAcceptable, errors.MethodNotAllowedf("Invalid HTTP method %s", r.Method)
	}
	return http.StatusOK, nil
}

// ServeHTTP implements the http.Handler interface for the ActivityHandlerFn type
func (a ActivityHandlerFn) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var dat []byte
	var it vocab.Item
	var err error
	var status = http.StatusInternalServerError

	if status, err = a.ValidateRequest(r); err != nil {
		errors.HandleError(err).ServeHTTP(w, r)
		return
	}

	// TODO(marius): we need a better mechanism than loading the storage object from the Request Context
	st, err := a.Storage(r)
	if err != nil {
		errors.HandleError(err).ServeHTTP(w, r)
		return
	}

	if it, status, err = a(Typer.Type(r), r, st); err != nil {
		errors.HandleError(err).ServeHTTP(w, r)
		return
	}

	typ := it.GetType()
	if vocab.ActivityTypes.Contains(typ) {
		err = vocab.OnActivity(it, func(act *vocab.Activity) error {
			if act.Object.IsLink() {
				if it, _ := st.Load(act.Object.GetLink()); it != nil {
					if it.IsCollection() {
						vocab.OnCollectionIntf(it, func(c vocab.CollectionInterface) error {
							act.Object = c.Collection()
							return nil
						})
					} else {
						act.Object = it
					}
				}
			}
			// For non-intransitive activities we want to output the object in the response
			if dat, err = vocab.MarshalJSON(act.Object); err != nil {
				return err
			}
			return nil
		})
	} else if vocab.IntransitiveActivityTypes.Contains(typ) {
		status = http.StatusNoContent
	} else {
		err = errors.BadRequestf("Invalid activity type %s received", typ)
	}
	if err != nil {
		errors.HandleError(err).ServeHTTP(w, r)
		return
	}

	switch status {
	case http.StatusCreated:
		if len(it.GetLink()) > 0 {
			w.Header().Set("Location", it.GetLink().String())
		}
	case http.StatusGone:
		if len(it.GetLink()) > 0 {
			w.Header().Set("Location", it.GetLink().String())
		}
	case http.StatusNoContent:
		if len(it.GetLink()) > 0 {
			w.Header().Set("Location", it.GetLink().String())
		}
	default:
		w.Header().Set("Content-Type", json.ContentType)
		dat, _ = vocab.MarshalJSON(it)
	}
	w.WriteHeader(status)
	w.Write(dat)
}

// CollectionHandlerFn is the type that we're using to represent handlers that will return ActivityStreams
// Collection or OrderedCollection objects. It needs to implement the http.Handler interface.
type CollectionHandlerFn func(vocab.CollectionPath, *http.Request, ReadStore) (vocab.CollectionInterface, error)

func (c CollectionHandlerFn) Storage(r *http.Request) (Store, error) {
	ctxVal := r.Context().Value(RepositoryKey)
	repo, ok := ctxVal.(Store)
	if !ok {
		return nil, errors.Newf("Unable to find Collection storage")
	}
	return repo, nil
}

// ValidMethod validates if the current handler can process the current request
func (c CollectionHandlerFn) ValidMethod(r *http.Request) bool {
	return r.Method == http.MethodGet || r.Method == http.MethodHead
}

// ValidateRequest validates if the current handler can process the current request
func (c CollectionHandlerFn) ValidateRequest(r *http.Request) (int, error) {
	if !c.ValidMethod(r) {
		return http.StatusMethodNotAllowed, errors.MethodNotAllowedf("Invalid HTTP method %s", r.Method)
	}
	return http.StatusOK, nil
}

// ServeHTTP implements the http.Handler interface for the CollectionHandlerFn type
func (c CollectionHandlerFn) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var dat []byte

	var status = http.StatusInternalServerError
	var err error

	status, err = c.ValidateRequest(r)
	if err != nil {
		errors.HandleError(err).ServeHTTP(w, r)
		return
	}

	st, err := c.Storage(r)
	if err != nil {
		errors.HandleError(err).ServeHTTP(w, r)
		return
	}

	col, err := c(Typer.Type(r), r, st)
	if err != nil {
		errors.HandleError(err).ServeHTTP(w, r)
		return
	}
	if dat, err = json.WithContext(json.IRI(vocab.ActivityBaseURI), json.IRI(vocab.SecurityContextURI)).Marshal(col); err != nil {
		errors.HandleError(err).ServeHTTP(w, r)
		return
	}

	vocab.OnObject(col, func(o *vocab.Object) error {
		updatedAt := o.Published
		if !o.Updated.IsZero() {
			updatedAt = o.Updated
		}
		if !updatedAt.IsZero() {
			w.Header().Set("Last-Modified", updatedAt.Format(time.RFC1123))
		}
		return nil
	})
	status = http.StatusOK
	w.Header().Set("Content-Type", json.ContentType)
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(24*time.Hour.Seconds())))
	}
	w.WriteHeader(status)
	if r.Method == http.MethodGet {
		w.Write(dat)
	}
}

// ItemHandlerFn is the type that we're using to represent handlers that return ActivityStreams
// objects. It needs to implement the http.Handler interface
type ItemHandlerFn func(*http.Request, ReadStore) (vocab.Item, error)

func (i ItemHandlerFn) Storage(r *http.Request) (Store, error) {
	ctxVal := r.Context().Value(RepositoryKey)
	st, ok := ctxVal.(Store)
	if !ok {
		return nil, errors.Newf("Unable to find Object storage")
	}
	return st, nil
}

// ValidMethod validates if the current handler can process the current request
func (i ItemHandlerFn) ValidMethod(r *http.Request) bool {
	return r.Method == http.MethodGet || r.Method == http.MethodHead
}

// ValidateRequest validates if the current handler can process the current request
func (i ItemHandlerFn) ValidateRequest(r *http.Request) (int, error) {
	if !i.ValidMethod(r) {
		return http.StatusMethodNotAllowed, errors.MethodNotAllowedf("Invalid HTTP method %s", r.Method)
	}
	return http.StatusOK, nil
}

// ServeHTTP implements the http.Handler interface for the ItemHandlerFn type
func (i ItemHandlerFn) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var dat []byte
	var err error
	status := http.StatusInternalServerError

	status, err = i.ValidateRequest(r)
	if err != nil {
		errors.HandleError(err).ServeHTTP(w, r)
		return
	}

	st, err := i.Storage(r)
	if err != nil {
		errors.HandleError(err).ServeHTTP(w, r)
		return
	}

	it, err := i(r, st)
	if err != nil {
		errors.HandleError(err).ServeHTTP(w, r)
		return
	}
	if it == nil {
		errors.HandleError(errors.NotFoundf("")).ServeHTTP(w, r)
		return
	}
	if dat, err = json.WithContext(json.IRI(vocab.ActivityBaseURI), json.IRI(vocab.SecurityContextURI)).Marshal(it); err != nil {
		errors.HandleError(err).ServeHTTP(w, r)
		return
	}

	vocab.OnObject(it, func(o *vocab.Object) error {
		updatedAt := o.Published
		if !o.Updated.IsZero() {
			updatedAt = o.Updated
		}
		if !updatedAt.IsZero() {
			w.Header().Set("Last-Modified", updatedAt.Format(time.RFC1123))
		}
		if vocab.ActivityTypes.Contains(o.Type) {
			w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d, immutable", int(8766*time.Hour.Seconds())))
		} else {
			w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", int(24*time.Hour.Seconds())))
		}
		return nil
	})
	status = http.StatusOK
	if it.GetType() == vocab.TombstoneType {
		status = http.StatusGone
	}
	w.Header().Set("Content-Type", json.ContentType)
	w.WriteHeader(status)
	if r.Method == http.MethodGet {
		w.Write(dat)
	}
}
