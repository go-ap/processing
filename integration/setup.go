package integration

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/processing"
	"github.com/go-ap/processing/internal"
)

type StorageInitFn func(storage *internal.MockStorage)
type InitFn any

func (mp *MockPlumbing) InitP(fns ...InitFn) *processing.P {
	initFns := make([]processing.OptionFn, 0, len(fns))
	storage := new(internal.MockStorage)

	for _, fn := range fns {
		switch initFn := fn.(type) {
		case StorageInitFn:
			initFn(storage)
		case processing.OptionFn:
			initFns = append(initFns, initFn)
		}
	}

	initFns = append(initFns, processing.WithStorage(storage))
	initFns = append(initFns, processing.WithIDGenerator(mp.GenerateID))
	initFns = append(initFns, processing.WithLocalIRIChecker(mp.LocalIRI))
	initFns = append(initFns, processing.WithTimeFn(mp.Now))

	return processing.New(initFns...)
}

func WithMocks(it ...vocab.Item) StorageInitFn {
	return func(storage *internal.MockStorage) {
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
		panic(fmt.Errorf("trying to pop value from empty IRI slice"))
	}
	iri := mp.iris[0]
	mp.iris = mp.iris[1:]
	return iri
}

func (mp *MockPlumbing) PopTime() time.Time {
	if len(mp.times) == 0 {
		panic(fmt.Errorf("trying to pop value from empty time.Time slice"))
	}
	tt := mp.times[len(mp.times)-1]
	mp.times = mp.times[:len(mp.times)-1]
	return tt
}
