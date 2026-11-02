package integration

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/client"
	"github.com/go-ap/errors"
	"github.com/go-ap/filters"
	"github.com/go-ap/processing"
	conformance "github.com/go-ap/storage-conformance-suite"
	"github.com/google/go-cmp/cmp"
)

type (
	LocalStorage conformance.MemStorage

	RemoteStorage conformance.MemStorage

	LocalStorageInitFn  func(storage *LocalStorage)
	RemoteStorageInitFn func(storage *RemoteStorage)
	RemoteInitFn        func(storage *MockRemote)
	InitFn              any
)

func (l *LocalStorage) Load(iri vocab.IRI, check ...filters.Check) (vocab.Item, error) {
	return (*conformance.MemStorage)(l).Load(iri, check...)
}

func (l *LocalStorage) Save(item vocab.Item) (vocab.Item, error) {
	return (*conformance.MemStorage)(l).Save(item)
}

func (l *LocalStorage) Delete(item vocab.Item) error {
	return (*conformance.MemStorage)(l).Delete(item)
}

func (l *LocalStorage) AddTo(iri vocab.IRI, item ...vocab.Item) error {
	return (*conformance.MemStorage)(l).AddTo(iri, item...)
}

func (l *LocalStorage) RemoveFrom(iri vocab.IRI, item ...vocab.Item) error {
	return (*conformance.MemStorage)(l).RemoveFrom(iri, item...)
}

func (r *RemoteStorage) Load(iri vocab.IRI, check ...filters.Check) (vocab.Item, error) {
	return (*conformance.MemStorage)(r).Load(iri, check...)
}

func (r *RemoteStorage) Save(item vocab.Item) (vocab.Item, error) {
	return (*conformance.MemStorage)(r).Save(item)
}

func (r *RemoteStorage) Delete(item vocab.Item) error {
	return (*conformance.MemStorage)(r).Delete(item)
}

func (r *RemoteStorage) AddTo(iri vocab.IRI, item ...vocab.Item) error {

	return (*conformance.MemStorage)(r).AddTo(iri, item...)
}

func (r *RemoteStorage) RemoveFrom(iri vocab.IRI, item ...vocab.Item) error {
	ms := (*conformance.MemStorage)(r)
	return ms.RemoveFrom(iri, item...)
}

var _ processing.Store = new(LocalStorage)
var _ processing.Store = new(RemoteStorage)

func (mp *MockPlumbing) InitP(fns ...InitFn) *processing.P {
	initFns := make([]processing.OptionFn, 0, len(fns))
	localStorage := new(LocalStorage)

	for _, fn := range fns {
		switch initFn := fn.(type) {
		case LocalStorageInitFn:
			initFn(localStorage)
		case processing.OptionFn:
			initFns = append(initFns, initFn)
		}
	}

	initFns = append(initFns, processing.WithStorage(localStorage))
	initFns = append(initFns, processing.WithIDGenerator(mp.GenerateID))
	initFns = append(initFns, processing.WithLocalIRIChecker(mp.LocalIRI))
	initFns = append(initFns, processing.WithTimeFn(mp.Now))

	pp := processing.New(initFns...)
	return pp
}

func saveItemsToStorage(storage processing.Store, it ...vocab.Item) {
	p := processing.New(processing.WithStorage(storage))
	for _, i := range it {
		if _, err := storage.Save(i); err != nil {
			panic(fmt.Errorf("failed saving item %s: %w", i.GetID(), err))
		}
		if err := p.CreateCollectionsForObject(i); err != nil {
			panic(fmt.Errorf("failed saving item %s: %w", i.GetID(), err))
		}
	}
}

func WithRemote(mr *MockRemote) processing.OptionFn {
	return func(p *processing.P) {
		processing.WithClient(mr.C)(p)
	}
}

func WithRemoteMocks(it ...vocab.Item) RemoteStorageInitFn {
	return func(storage *RemoteStorage) {
		saveItemsToStorage(storage, it...)
	}
}

func WithLocalMocks(it ...vocab.Item) LocalStorageInitFn {
	return func(storage *LocalStorage) {
		saveItemsToStorage(storage, it...)
	}
}

type MockPlumbing struct {
	time  time.Time
	times []time.Time

	typCounts sync.Map
	iris      []vocab.IRI
}

const maxSec = int32(24 * time.Hour / time.Second)

func (mp *MockPlumbing) Now() time.Time {
	if mp.time.IsZero() {
		mp.time = Author.Published
	}
	if mp.times == nil {
		mp.times = make([]time.Time, 0, 2)
	}

	randSec := rand.Int31n(maxSec)
	mp.time = mp.time.Add(time.Duration(randSec) * time.Second).Round(time.Millisecond)
	mp.times = append(mp.times, mp.time)

	return mp.time
}

func (mp *MockPlumbing) GenerateID(it vocab.Item, byActivity vocab.Item) (vocab.ID, error) {
	typ := vocab.ActivityVocabularyType("Tag")
	if itTyp := it.GetType(); itTyp != nil {
		for _, tt := range itTyp.AsTypes() {
			typ = tt
			break
		}
	}
	if mp.iris == nil {
		mp.iris = make([]vocab.IRI, 0, 2)
	}

	iri := RootID.AddPath(strings.ToLower(string(typ)), strconv.Itoa(mp.nextTypeCount(typ)))
	mp.iris = append(mp.iris, iri)

	err := vocab.OnObject(it, func(ob *vocab.Object) error {
		ob.ID = iri
		return nil
	})
	return iri, err
}

func (mp *MockPlumbing) nextTypeCount(typ vocab.ActivityVocabularyType) int {
	vv, ok := mp.typCounts.Load(typ)
	if !ok {
		vv = 0
	}
	mp.typCounts.Store(typ, vv)
	return vv.(int)
}

func (mp *MockPlumbing) LocalIRI(i vocab.IRI) bool {
	return strings.HasPrefix(string(i), string(RootID))
}

func (mp *MockPlumbing) ShiftIRI() vocab.IRI {
	if len(mp.iris) == 0 {
		return ""
	}
	iri := mp.iris[0]
	mp.iris = mp.iris[1:]
	return iri
}

func (mp *MockPlumbing) PopTime() time.Time {
	if len(mp.times) == 0 {
		return time.Time{}
	}
	tt := mp.times[len(mp.times)-1]
	mp.times = mp.times[:len(mp.times)-1]
	return tt
}

type MockRemote struct {
	asserts  map[vocab.IRI]vocab.ItemCollection
	received map[vocab.IRI]vocab.ItemCollection

	S *RemoteStorage
	C *client.C
}

func (mr *MockRemote) Run(t *testing.T, _ vocab.Item, _ error) {
	if !cmp.Equal(mr.asserts, mr.received) {
		t.Fatalf("invalid requests received %s", cmp.Diff(mr.asserts, mr.received))
	}
}

func (mr *MockRemote) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	iri := vocab.IRI("http://" + req.Host + req.RequestURI)
	if req.Method == http.MethodGet {
		it, err := mr.S.Load(iri)
		if err != nil {
			errors.HandleError(err).ServeHTTP(w, req)
			return
		}
		raw, err := vocab.MarshalJSON(it)
		if err != nil {
			errors.HandleError(err).ServeHTTP(w, req)
			return
		}
		w.Write(raw)
	}

	raw, err := io.ReadAll(req.Body)
	if err != nil {
		errors.HandleError(err).ServeHTTP(w, req)
		return
	}
	it, err := vocab.UnmarshalJSON(raw)
	if err != nil {
		errors.HandleError(err).ServeHTTP(w, req)
		return
	}
	colIts, exists := mr.received[iri]
	if !exists {
		colIts = make(vocab.ItemCollection, 0)
	}
	colIts = append(colIts, it)
	mr.received[iri] = colIts
}

func Request(iri vocab.IRI, items ...vocab.Item) RemoteInitFn {
	return func(mr *MockRemote) {
		mr.asserts[iri] = items
	}
}

func (mr *MockRemote) WillReceive(initFns ...RemoteInitFn) TestAssertion {
	for _, fn := range initFns {
		fn(mr)
	}
	return mr
}

func Remote(fns ...InitFn) *MockRemote {
	mr := MockRemote{
		asserts:  make(map[vocab.IRI]vocab.ItemCollection),
		received: make(map[vocab.IRI]vocab.ItemCollection),
		S:        new(RemoteStorage),
	}
	for _, fn := range fns {
		switch initFn := fn.(type) {
		case RemoteStorageInitFn:
			initFn(mr.S)
		}
	}

	srv := httptest.NewServer(&mr)
	mr.C = client.New(client.WithHTTPClient(srv.Client()))
	return &mr
}
