package processing

import (
	"sync"
	"testing"

	"git.sr.ht/~mariusor/lw"
	vocab "github.com/go-ap/activitypub"
	c "github.com/go-ap/client"
	"github.com/go-ap/errors"
)

func emptyCol(id vocab.IRI) *vocab.OrderedCollection {
	return &vocab.OrderedCollection{
		ID:           id,
		Type:         vocab.OrderedCollectionType,
		OrderedItems: make(vocab.ItemCollection, 0),
	}
}

func mockStorage(t *testing.T, l lw.Logger) Store {
	store := mockStore{Map: &sync.Map{}}

	_, err := store.Save(defaultActor)
	if err != nil {
		t.Errorf("Failed to save default actor %v", err)
	}
	for _, col := range vocab.ActivityPubCollections {
		_, err = store.Create(emptyCol(col.IRI(defaultActor)))
	}
	return store
}

func mockClient(t *testing.T, l lw.Logger) c.Basic {
	return c.New(c.WithLogger(l), c.SkipTLSValidation(true))
}

func mockProcessor(t *testing.T, base vocab.IRI) *P {
	l := lw.Dev(lw.SetOutput(t.Output()))
	return &P{
		baseIRI:         vocab.IRIs{base},
		async:           false,
		c:               mockClient(t, l),
		s:               mockStorage(t, l),
		l:               l,
		localIRICheckFn: defaultLocalIRICheck,
		createIDFn:      defaultIDGenerator(base),
		actorKeyGenFn:   defaultKeyGenerator(),
	}
}

func TestP_AddActivity(t *testing.T) {
	tests := []struct {
		name    string
		base    vocab.IRI
		add     *vocab.Activity
		want    *vocab.Activity
		wantErr error
	}{
		{
			name:    "empty",
			base:    "https://example.local",
			wantErr: InvalidActivity("nil Add activity"),
		},
		{
			name: "add jdoe to his own followers",
			base: "https://jdoe.example.local",
			add: &vocab.Activity{
				Target: vocab.IRI("https://jdoe.example.com/followers"),
				Object: &vocab.Actor{ID: "https://jdoe.example.com"},
			},
			want: &vocab.Activity{
				Target: vocab.IRI("https://jdoe.example.com/followers"),
				Object: &vocab.Actor{ID: "https://jdoe.example.com"},
			},
		},
		{
			name: "add jdoe to his own followers and following",
			base: "https://jdoe.example.local",
			add: &vocab.Activity{
				Target: vocab.IRIs{"https://jdoe.example.com/followers", "https://jdoe.example.com/following"},
				Object: &vocab.Actor{ID: "https://jdoe.example.com"},
			},
			want: &vocab.Activity{
				Target: vocab.IRIs{"https://jdoe.example.com/followers", "https://jdoe.example.com/following"},
				Object: &vocab.Actor{ID: "https://jdoe.example.com"},
			},
		},
		{
			name: "add random object to inbox",
			base: "https://jdoe.example.local",
			add: &vocab.Activity{
				Target: vocab.IRI("https://jdoe.example.com/inbox"),
				Object: &vocab.Object{ID: "https://example.com", Type: vocab.ProfileType, Content: vocab.NaturalLanguageValuesNew(vocab.DefaultLangRef("test"))},
			},
			want: &vocab.Activity{
				Target: vocab.IRI("https://jdoe.example.com/inbox"),
				Object: &vocab.Object{ID: "https://example.com", Type: vocab.ProfileType, Content: vocab.NaturalLanguageValuesNew(vocab.DefaultLangRef("test"))},
			},
		},
		{
			name: "add IRI to outbox",
			base: "https://jdoe.example.local",
			add: &vocab.Activity{
				Target: vocab.IRI("https://jdoe.example.com/outbox"),
				Object: vocab.IRI("https://example.com"),
			},
			want: &vocab.Activity{
				Target: vocab.IRI("https://jdoe.example.com/outbox"),
				Object: vocab.IRI("https://example.com"),
			},
		},
		{
			name: "add multiple IRIs to outbox",
			base: "https://jdoe.example.local",
			add: &vocab.Activity{
				Target: vocab.IRI("https://jdoe.example.com/outbox"),
				Object: vocab.IRIs{"https://example.com", "https://jdoe.example.com"},
			},
			want: &vocab.Activity{
				Target: vocab.IRI("https://jdoe.example.com/outbox"),
				Object: vocab.IRIs{"https://example.com", "https://jdoe.example.com"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := mockProcessor(t, tt.base)
			got, err := p.AddActivity(tt.add)
			if (err != nil) && !errors.Is(tt.wantErr, err) {
				t.Errorf("AddActivity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !vocab.ItemsEqual(got, tt.want) {
				t.Errorf("AddActivity() got = %v, want %v", got, tt.want)
			}
			if vocab.IsNil(got) {
				return
			}
			_ = vocab.OnItem(got.Target, func(tgt vocab.Item) error {
				col, err := p.s.Load(tgt.GetLink())
				if err != nil {
					t.Errorf("AddActivity() unable to load target form storage: %v", err)
					return nil
				}
				target, ok := col.(vocab.CollectionInterface)
				if !ok {
					t.Errorf("AddActivity() got Activity %T target %T is not %T", got, tgt, vocab.CollectionInterface(nil))
					return nil
				}
				return vocab.OnItem(got.Object, func(item vocab.Item) error {
					if !target.Contains(item) {
						t.Errorf("AddActivity() got Activity %T object %s can't be found in %#v", got, item.GetLink(), tgt)
					}
					return nil
				})
			})
		})
	}
}

func TestP_RemoveActivity(t *testing.T) {
	tests := []struct {
		name    string
		base    vocab.IRI
		remove  *vocab.Activity
		items   vocab.ItemCollection
		want    *vocab.Activity
		wantErr error
	}{
		{
			name:    "empty",
			base:    "https://example.local",
			wantErr: InvalidActivity("nil Remove activity"),
		},
		{
			name:  "remove jdoe from his own followers",
			base:  "https://jdoe.example.local",
			items: vocab.ItemCollection{&vocab.Actor{ID: "https://jdoe.example.com"}},
			remove: &vocab.Activity{
				Target: vocab.IRI("https://jdoe.example.com/followers"),
				Object: vocab.IRI("https://jdoe.example.com"),
			},
			want: &vocab.Activity{
				Target: vocab.IRI("https://jdoe.example.com/followers"),
				Object: vocab.IRI("https://jdoe.example.com"),
			},
		},
		{
			name:  "remove random object from inbox",
			base:  "https://jdoe.example.local",
			items: vocab.ItemCollection{&vocab.Object{ID: "https://example.com", Type: vocab.ProfileType, Content: vocab.NaturalLanguageValuesNew(vocab.DefaultLangRef("test"))}},
			remove: &vocab.Activity{
				Target: vocab.IRI("https://jdoe.example.com/inbox"),
				Object: vocab.IRI("https://example.com"),
			},
			want: &vocab.Activity{
				Target: vocab.IRI("https://jdoe.example.com/inbox"),
				Object: vocab.IRI("https://example.com"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := mockProcessor(t, tt.base)
			// NOTE(marius): add items to collection
			for _, it := range tt.items {
				_ = p.s.AddTo(tt.remove.Target.GetLink(), it)
			}

			got, err := p.RemoveActivity(tt.remove)
			if (err != nil) && !errors.Is(tt.wantErr, err) {
				t.Errorf("RemoveActivity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !vocab.ItemsEqual(got, tt.want) {
				t.Errorf("RemoveActivity() got = %v, want %v", got, tt.want)
			}
			if vocab.IsNil(got) {
				return
			}
			_ = vocab.OnItem(got.Target, func(tgt vocab.Item) error {
				col, err := p.s.Load(tgt.GetLink())
				if err != nil {
					t.Errorf("RemoveActivity() unable to load target form storage: %v", err)
					return nil
				}
				target, ok := col.(vocab.CollectionInterface)
				if !ok {
					t.Errorf("RemoveActivity() got Activity %T target %T is not %T", got, tgt, vocab.CollectionInterface(nil))
					return nil
				}
				return vocab.OnItem(got.Object, func(item vocab.Item) error {
					if !target.Contains(item) {
						t.Errorf("RemoveActivity() got Activity %T object %s can't be found in %#v", got, item.GetLink(), tgt)
					}
					return nil
				})
			})
		})
	}
}

func TestP_MoveActivity(t *testing.T) {
	tests := []struct {
		name    string
		base    vocab.IRI
		remove  *vocab.Activity
		items   vocab.ItemCollection
		want    *vocab.Activity
		wantErr error
	}{
		{
			name:    "empty",
			base:    "https://example.local",
			wantErr: InvalidActivity("nil Move activity"),
		},
		{
			name:  "move jdoe from followers to following",
			base:  "https://jdoe.example.local",
			items: vocab.ItemCollection{&vocab.Actor{ID: "https://jdoe.example.com"}},
			remove: &vocab.Activity{
				Origin: vocab.IRI("https://jdoe.example.com/followers"),
				Target: vocab.IRI("https://jdoe.example.com/following"),
				Object: vocab.IRI("https://jdoe.example.com"),
			},
			want: &vocab.Activity{
				Origin: vocab.IRI("https://jdoe.example.com/followers"),
				Target: vocab.IRI("https://jdoe.example.com/following"),
				Object: vocab.IRI("https://jdoe.example.com"),
			},
		},
		{
			name: "move random object from inbox to outbox",
			base: "https://jdoe.example.local",
			items: vocab.ItemCollection{
				&vocab.Object{ID: "https://example.com", Type: vocab.ProfileType, Content: vocab.NaturalLanguageValuesNew(vocab.DefaultLangRef("test"))},
			},
			remove: &vocab.Activity{
				Origin: vocab.IRI("https://jdoe.example.com/inbox"),
				Target: vocab.IRI("https://jdoe.example.com/outbox"),
				Object: vocab.IRI("https://example.com"),
			},
			want: &vocab.Activity{
				Origin: vocab.IRI("https://jdoe.example.com/inbox"),
				Target: vocab.IRI("https://jdoe.example.com/outbox"),
				Object: vocab.IRI("https://example.com"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := mockProcessor(t, tt.base)
			// NOTE(marius): add items to origin collection
			for _, it := range tt.items {
				_ = p.s.AddTo(tt.remove.Origin.GetLink(), it)
			}

			got, err := p.MoveActivity(tt.remove)
			if (err != nil) && !errors.Is(tt.wantErr, err) {
				t.Errorf("MoveActivity() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !vocab.ItemsEqual(got, tt.want) {
				t.Errorf("MoveActivity() got = %v, want %v", got, tt.want)
			}
			if vocab.IsNil(got) {
				return
			}
			_ = vocab.OnItem(got.Origin, func(orig vocab.Item) error {
				col, err := p.s.Load(orig.GetLink())
				if err != nil {
					t.Errorf("MoveActivity() unable to load origin form storage: %v", err)
					return nil
				}
				origin, ok := col.(vocab.CollectionInterface)
				if !ok {
					t.Errorf("MoveActivity() got Activity %T origin %T is not %T", got, orig, vocab.CollectionInterface(nil))
					return nil
				}
				return vocab.OnItem(got.Object, func(item vocab.Item) error {
					if origin.Contains(item) {
						t.Errorf("MoveActivity() got Activity %T object %s still exists origin %#v", got, item.GetLink(), orig)
					}
					return nil
				})
			})
			_ = vocab.OnItem(got.Target, func(tgt vocab.Item) error {
				col, err := p.s.Load(tgt.GetLink())
				if err != nil {
					t.Errorf("MoveActivity() unable to load target form storage: %v", err)
					return nil
				}
				target, ok := col.(vocab.CollectionInterface)
				if !ok {
					t.Errorf("MoveActivity() got Activity %T target %T is not %T", got, tgt, vocab.CollectionInterface(nil))
					return nil
				}
				return vocab.OnItem(got.Object, func(item vocab.Item) error {
					if !target.Contains(item) {
						t.Errorf("MoveActivity() got Activity %T object %s can't be found in target %#v", got, item.GetLink(), tgt)
					}
					return nil
				})
			})
		})
	}
}
