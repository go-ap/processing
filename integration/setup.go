package integration

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/carlmjohnson/requests"
	rfc "github.com/dadrus/httpsig"
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
	"github.com/go-ap/filters"
	"github.com/go-ap/jsonld"
	"github.com/go-ap/processing"
	conformance "github.com/go-ap/storage-conformance-suite"
	"github.com/google/go-cmp/cmp"
)

type (
	LocalStorage conformance.MemStorage

	RemoteStorage conformance.MemStorage

	LocalStorageInitFn  func(*LocalStorage)
	RemoteStorageInitFn func(*RemoteStorage)
	RemoteInitFn        func(*MockRemote)
	ClientInitFn        func(*MockClient)
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

	for _, fn := range fns {
		switch initFn := fn.(type) {
		case processing.OptionFn:
			initFns = append(initFns, initFn)
		}
	}

	// NOTE(marius): we usually want the processing to be done synchronously
	initFns = append(initFns, processing.WithSynchronousProcessing)
	initFns = append(initFns, processing.WithDisseminationRetryCount(0))
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

func WithClientActorKey(act *vocab.Actor, key crypto.PublicKey) ClientInitFn {
	return func(cl *MockClient) {
		cl.Actor = act
		cl.Key = key
	}
}

func WithClientTestingT(t TestingT) ClientInitFn {
	return func(cl *MockClient) {
		cl.T = t
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

type AuthorizedMap map[vocab.IRI]struct {
	Actor *vocab.Actor
	Key   crypto.PrivateKey
}

func Auth(act *vocab.Actor, key crypto.PrivateKey) AuthorizedMap {
	return AuthorizedMap{
		act.PublicKey.ID: {Actor: act, Key: key},
	}
}

func (am AuthorizedMap) Find(id string) (*vocab.Actor, crypto.PrivateKey, error) {
	pair, ok := am[vocab.IRI(id)]
	if !ok {
		return nil, nil, fmt.Errorf("keyId %s not found", id)
	}
	return pair.Actor, pair.Key, nil
}

func (am AuthorizedMap) ResolveKey(ctx context.Context, info rfc.SignatureInfo) (rfc.KeyInfo, error) {
	key := rfc.KeyInfo{}
	keyID, _ := info.Parameters.KeyID()
	act, prv, err := am.Find(keyID)
	if err != nil {
		return key, err
	}
	pub, err := act.PublicKey.CryptoValue()
	if err != nil {
		return key, err
	}

	switch pk := prv.(type) {
	case *rsa.PrivateKey:
		switch pk.Size() {
		case 256, 384:
			key.Key.Algorithm = rfc.RsaPkcs1v15Sha384
		case 512:
			key.Key.Algorithm = rfc.RsaPkcs1v15Sha512
		}
	case ed25519.PrivateKey:
		key.Key.Algorithm = rfc.Ed25519
	}
	if alg, ok := info.Parameters.Alg(); ok && alg != key.Key.Algorithm {
		return key, fmt.Errorf("mismatched algorithm parameter")
	}
	key.Key.Material = pub
	return key, nil
}

func (am AuthorizedMap) VerifySignature(req *http.Request) error {
	verifier, err := rfc.NewVerifier(
		am,
		rfc.WithValidityTolerance(time.Hour),
		rfc.WithMaxAge(time.Hour),
		rfc.WithCreatedTimestamp(false),
		rfc.WithExpiresTimestamp(false),
		rfc.WithValidateAllSignatures(),
	)
	if err != nil {
		return err
	}
	valid, err := verifier.Verify(rfc.MessageFromRequest(req))
	if err != nil {
		return err
	}
	for _, v := range valid {
		keyID, _ := v.Parameters.KeyID()
		if act, _, _ := am.Find(keyID); act != nil {
			return nil
		}
	}
	return fmt.Errorf("no authorized actor found in signatures %v", valid)
}

type TestingT interface {
	Name() string
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fail()
	FailNow()
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Skip(args ...interface{})
	Skipf(format string, args ...interface{})
	SkipNow()
	Skipped() bool
}

type MockRemote struct {
	C *MockClient

	assertions map[vocab.IRI][]http.Handler
	m          sync.Mutex
}

func (mr *MockRemote) Success(t TestingT) error {
	if len(mr.assertions) != 0 {
		for iri := range mr.assertions {
			t.Errorf("Request assertion handler wasn't called for iri: %s", iri)
		}
	}
	return nil
}

func ServeItemHandler(t TestingT, authMap AuthorizedMap, it vocab.Item) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = fmt.Fprintf(w, "invalid method for assertion handler")
			t.Errorf("invalid method %s, expected %s", req.Method, http.MethodGet)
			return
		}

		isPublic := true
		if hasRecipients, ok := it.(vocab.HasRecipients); ok {
			recipients := hasRecipients.Recipients()
			isPublic = recipients.Contains(vocab.PublicNS)
		}
		if !isPublic {
			if authMap == nil {
				w.WriteHeader(http.StatusForbidden)
				_, _ = fmt.Fprintf(w, "item is not public and there's no valid authentication mechanism")
				t.Errorf("item is not public and no authentication map was provided")
				return
			}
			if err := authMap.VerifySignature(req); err != nil {
				w.WriteHeader(http.StatusForbidden)
				_, _ = fmt.Fprintf(w, "%v", err)
				t.Errorf("failed to verify HTTP Signature against the authorization map: %v", err)
				return
			}
		}

		raw, err := vocab.MarshalJSON(it)
		if err != nil {
			errors.HandleError(err).ServeHTTP(w, req)
			t.Errorf("failed to marshal item: %v", err)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
	})
}

func ReceivedActivityMatches(t TestingT, fns ...ItemAssertFn) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			errors.HandleError(err).ServeHTTP(w, req)
			t.Errorf("unable to read request body: %v", err)
			return
		}
		got, err := vocab.UnmarshalJSON(raw)
		if err != nil {
			errors.HandleError(err).ServeHTTP(w, req)
			t.Errorf("unable to unmarshal activity from request %v", err)
			return
		}

		for _, fn := range fns {
			fn(t, got)
		}
	}
}

func ReceivedActivityIdenticalTo(t TestingT, expected vocab.Item) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			errors.HandleError(err).ServeHTTP(w, req)
			t.Errorf("unable to read request body: %v", err)
			return
		}
		got, err := vocab.UnmarshalJSON(raw)
		if err != nil {
			errors.HandleError(err).ServeHTTP(w, req)
			t.Errorf("unable to unmarshal activity from request %v", err)
			return
		}

		if !cmp.Equal(expected, got) {
			t.Errorf("received item is different than expected: %s", cmp.Diff(expected, got))
		}
	}
}

func (mr *MockRemote) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	mr.m.Lock()
	defer mr.m.Unlock()

	if len(mr.assertions) == 0 {
		errors.HandleError(fmt.Errorf("there are no assertions to run")).ServeHTTP(w, req)
		return
	}

	iri := vocab.IRI("http://" + req.Host + req.RequestURI)
	currentAsserts, ok := mr.assertions[iri]
	if !ok {
		errors.HandleError(fmt.Errorf("no assertion for iri %s", iri)).ServeHTTP(w, req)
		return
	}
	for _, currentAssert := range currentAsserts {
		currentAssert.ServeHTTP(w, req)
	}
	delete(mr.assertions, iri)
}

type HandlerMap map[vocab.IRI]http.Handler

func Handler(iri vocab.IRI, fn http.Handler) HandlerMap {
	return HandlerMap{iri: fn}
}

func WithHandlers(assertFns ...HandlerMap) RemoteInitFn {
	return func(mr *MockRemote) {
		for _, mapPairs := range assertFns {
			for iri, hFn := range mapPairs {
				fns, ok := mr.assertions[iri]
				if !ok {
					fns = []http.Handler{hFn}
				} else {
					fns = append(fns, hFn)
				}
				mr.assertions[iri] = fns
			}
		}
	}
}

func Storage(fns ...InitFn) *LocalStorage {
	localStorage := new(LocalStorage)

	for _, fn := range fns {
		switch initFn := fn.(type) {
		case LocalStorageInitFn:
			initFn(localStorage)
		}
	}

	return localStorage
}

func Remote(fns ...InitFn) *MockRemote {
	mr := MockRemote{
		assertions: make(map[vocab.IRI][]http.Handler),
	}

	srv := httptest.NewServer(&mr)
	mr.C = &MockClient{
		Client: *srv.Client(),
	}

	for _, fn := range fns {
		switch initFn := fn.(type) {
		case ClientInitFn:
			initFn(mr.C)
		case RemoteInitFn:
			initFn(&mr)
		}
	}

	return &mr
}

type MockClient struct {
	http.Client
	Actor *vocab.Actor
	Key   crypto.PrivateKey

	T TestingT
}

func (m MockClient) CtxToCollection(ctx context.Context, it vocab.Item, iris ...vocab.IRI) (vocab.IRI, vocab.Item, error) {
	cont, err := jsonld.WithContext(jsonld.IRI(vocab.ActivityBaseURI), jsonld.IRI(vocab.SecurityContextURI)).Marshal(it)
	if err != nil {
		return "", nil, errors.Errorf("unable to marshal activity")
	}

	results := make(vocab.ItemCollection, 0, len(iris))
	errs := make([]error, 0, len(iris))

	for _, iri := range iris {
		rb := requests.URL(string(iri)).Method(http.MethodPost)
		// NOTE(marius): these are required when signing requests with our draft HTTP-Signatures
		rb.Header("Accept", "application/activity+json")
		if u, err := rb.URL(); err == nil {
			rb.Header("Host", u.Host)
		}
		rb.Header("Date", time.Now().UTC().Format(http.TimeFormat))

		rb.ContentType(jsonld.ContentType)
		rb.BodyBytes(cont)

		if it, err = m.do(ctx, rb); err != nil {
			errs = append(errs, err)
			if m.T != nil {
				m.T.Errorf("Received unexpected error: %v", err)
			}
			continue
		}
		_ = results.Append(it)
	}

	return "", results.Normalize(), errors.Join(errs...)
}

func (m MockClient) do(ctx context.Context, rb *requests.Builder) (vocab.Item, error) {
	req, err := rb.Request(ctx)
	if err != nil {
		return nil, err
	}

	cl := m.Client

	if err = m.Sign(req); err != nil {
		return nil, err
	}

	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, errors.FromResponse(resp)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return vocab.UnmarshalJSON(raw)
}

func rfcAlgorithmFromPrivateKey(key crypto.PrivateKey) rfc.SignatureAlgorithm {
	// NOTE(marius): I'm not sure what purpose it serves to validate the public key of the actor
	// against the private key
	switch pk := key.(type) {
	case *rsa.PrivateKey:
		switch pk.Size() {
		case 128, 256:
			return rfc.RsaPkcs1v15Sha256
		case 384:
			return rfc.RsaPkcs1v15Sha384
		case 512:
			return rfc.RsaPkcs1v15Sha512
		}
	case ed25519.PrivateKey:
		return rfc.Ed25519
	default:
	}
	return "unknown"
}

func (m MockClient) Sign(req *http.Request) error {
	if m.Actor == nil || m.Key == nil {
		return nil
	}
	key := rfc.Key{
		Algorithm: rfcAlgorithmFromPrivateKey(m.Key),
		Material:  m.Key,
	}
	initFns := []rfc.SignerOption{
		rfc.WithKeyID(string(m.Actor.PublicKey.ID)),
		rfc.WithAlg(),
		rfc.WithTTL(time.Hour),
	}

	initFns = append(initFns, rfc.WithComponents(`"@method"`, `"@path"`))
	signer, err := rfc.NewSigner(key, initFns...)
	if err != nil {
		return err
	}
	signFns := make([]rfc.SignOption, 0)
	msg := rfc.MessageFromRequest(req)
	headersWithSignature, err := signer.Sign(msg, signFns...)
	if err != nil {
		return err
	}
	req.Header = headersWithSignature
	return nil
}

func (m MockClient) CtxLoadIRI(ctx context.Context, iri vocab.IRI) (vocab.Item, error) {
	rb := requests.URL(string(iri)).Method(http.MethodGet)
	// NOTE(marius): these are required when signing requests with our draft HTTP-Signatures
	rb.Header("Accept", "application/activity+json")
	if u, err := rb.URL(); err == nil {
		rb.Header("Host", u.Host)
	}
	rb.Header("Date", time.Now().UTC().Format(http.TimeFormat))

	return m.do(ctx, rb)
}

var _ processing.ActivityPubClient = MockClient{}
