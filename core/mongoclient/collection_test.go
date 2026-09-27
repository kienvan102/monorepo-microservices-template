package mongoclient

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type fakeClient struct{}

func (fakeClient) Database() Database         { return nil }
func (fakeClient) Ping(context.Context) error { return nil }
func (fakeClient) Disconnect()                {}

type unitDoc struct {
	Name string `bson:"name"`
}

func TestRawCollectionRejectsForeignClient(t *testing.T) {
	for _, c := range []Client{fakeClient{}, nil} {
		if _, err := RawCollection(c, "docs"); !errors.Is(err, ErrUnsupportedClient) {
			t.Errorf("RawCollection(%T) err = %v, want ErrUnsupportedClient", c, err)
		}
		if _, err := NewCollection[unitDoc](c, "docs"); !errors.Is(err, ErrUnsupportedClient) {
			t.Errorf("NewCollection(%T) err = %v, want ErrUnsupportedClient", c, err)
		}
	}
}

func TestUpdateAndDeleteRejectEmptyFilter(t *testing.T) {
	coll := &Collection[unitDoc]{}
	var nilMap bson.M
	for _, filter := range []any{nil, bson.M{}, bson.D{}, nilMap, map[string]any{}} {
		if _, _, err := coll.Update(context.Background(), filter, bson.M{"$set": bson.M{"name": "x"}}); !errors.Is(err, ErrEmptyFilter) {
			t.Errorf("Update(%#v) err = %v, want ErrEmptyFilter", filter, err)
		}
		if _, err := coll.Delete(context.Background(), filter); !errors.Is(err, ErrEmptyFilter) {
			t.Errorf("Delete(%#v) err = %v, want ErrEmptyFilter", filter, err)
		}
	}
}

func TestCheckFilterAcceptsNonEmpty(t *testing.T) {
	for _, filter := range []any{bson.M{"_id": 1}, bson.D{{Key: "name", Value: "x"}}} {
		if err := checkFilter(filter); err != nil {
			t.Errorf("checkFilter(%#v) = %v", filter, err)
		}
	}
}

func TestInsertRejectsNoDocuments(t *testing.T) {
	coll := &Collection[unitDoc]{}
	if _, err := coll.Insert(context.Background()); !errors.Is(err, ErrNoDocuments) {
		t.Fatalf("Insert() err = %v, want ErrNoDocuments", err)
	}
}

type stringIDDoc struct {
	ID   string `bson:"_id"`
	Name string `bson:"name"`
}

type objectIDDoc struct {
	ID   bson.ObjectID `bson:"_id"`
	Name string        `bson:"name"`
}

type orderID string

type typedIDDoc struct {
	ID orderID `bson:"_id"`
}

type Base struct {
	ID string `bson:"_id"`
}

type inlineIDDoc struct {
	Base `bson:",inline"`
	Name string `bson:"name"`
}

func testCollection[T any](gen IDGenerator) *Collection[T] {
	return &Collection[T]{id: findIDField(reflect.TypeFor[T]()), newID: gen}
}

func sequence(ids ...any) (IDGenerator, *int) {
	calls := 0
	return func(context.Context) (any, error) {
		id := ids[calls]
		calls++
		return id, nil
	}, &calls
}

func TestPrepareCallsGeneratorOnlyForMissingIDs(t *testing.T) {
	gen, calls := sequence("sys-1", "sys-2")
	docs := []*stringIDDoc{{Name: "a"}, {ID: "given", Name: "b"}, {Name: "c"}}
	if _, err := testCollection[stringIDDoc](gen).prepare(context.Background(), docs); err != nil {
		t.Fatal(err)
	}
	if *calls != 2 {
		t.Errorf("generator called %d times, want 2", *calls)
	}
	if docs[0].ID != "sys-1" || docs[1].ID != "given" || docs[2].ID != "sys-2" {
		t.Errorf("ids = %q, %q, %q", docs[0].ID, docs[1].ID, docs[2].ID)
	}
}

func TestPrepareUsesObjectIDWithoutGenerator(t *testing.T) {
	docs := []*objectIDDoc{{Name: "a"}, {Name: "b"}}
	if _, err := testCollection[objectIDDoc](nil).prepare(context.Background(), docs); err != nil {
		t.Fatal(err)
	}
	if docs[0].ID.IsZero() || docs[1].ID.IsZero() || docs[0].ID == docs[1].ID {
		t.Errorf("ids = %v, %v; want two different ObjectIDs", docs[0].ID, docs[1].ID)
	}
}

func TestPrepareConvertsIDAndReachesInlineField(t *testing.T) {
	gen, _ := sequence("sys-1")
	typed := &typedIDDoc{}
	if _, err := testCollection[typedIDDoc](gen).prepare(context.Background(), []*typedIDDoc{typed}); err != nil {
		t.Fatal(err)
	}
	if typed.ID != orderID("sys-1") {
		t.Errorf("typed id = %q", typed.ID)
	}

	gen, _ = sequence("sys-2")
	inline := &inlineIDDoc{Name: "a"}
	if _, err := testCollection[inlineIDDoc](gen).prepare(context.Background(), []*inlineIDDoc{inline}); err != nil {
		t.Fatal(err)
	}
	if inline.ID != "sys-2" {
		t.Errorf("inline id = %q", inline.ID)
	}
}

func TestPrepareRejectsBadIDs(t *testing.T) {
	boom := errors.New("id service down")
	tests := []struct {
		name string
		gen  IDGenerator
		want error
	}{
		{"wrong type", func(context.Context) (any, error) { return 42, nil }, ErrIDType},
		{"empty id", func(context.Context) (any, error) { return "", nil }, ErrIDType},
		{"nil id", func(context.Context) (any, error) { return nil, nil }, ErrIDType},
		{"generator error", func(context.Context) (any, error) { return nil, boom }, boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &stringIDDoc{Name: "a"}
			if _, err := testCollection[stringIDDoc](tt.gen).prepare(context.Background(), []*stringIDDoc{doc}); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}

	gen, _ := sequence("sys-1")
	if _, err := testCollection[stringIDDoc](gen).prepare(context.Background(), []*stringIDDoc{nil}); err == nil || !strings.Contains(err.Error(), "document 0 is nil") {
		t.Fatalf("nil document err = %v", err)
	}
}

func TestNewCollectionChecksIDField(t *testing.T) {
	_, err := NewCollection[stringIDDoc](driverClient{}, "docs")
	if !errors.Is(err, ErrIDType) || !strings.Contains(err.Error(), "stringIDDoc.ID is string") ||
		!strings.Contains(err.Error(), "WithIDGenerator") {
		t.Fatalf("string id without generator err = %v", err)
	}
	_, err = NewCollection[inlineIDDoc](driverClient{}, "docs")
	if !errors.Is(err, ErrIDType) || !strings.Contains(err.Error(), "inlineIDDoc.Base.ID is string") {
		t.Fatalf("inline string id without generator err = %v", err)
	}

	mc, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1"))
	if err != nil {
		t.Fatal(err)
	}
	defer disconnect(mc)
	gen, _ := sequence("sys-1")
	withGen := driverClient{client: mc, database: mc.Database("test"), newID: gen}
	plain := driverClient{client: mc, database: mc.Database("test")}
	if _, err := NewCollection[stringIDDoc](withGen, "docs"); err != nil {
		t.Errorf("string id with generator: %v", err)
	}
	if _, err := NewCollection[objectIDDoc](plain, "docs"); err != nil {
		t.Errorf("ObjectID id without generator: %v", err)
	}
	if _, err := NewCollection[unitDoc](plain, "docs"); err != nil {
		t.Errorf("no _id field without generator: %v", err)
	}
}

func TestDuplicateID(t *testing.T) {
	raw := func(d bson.D) bson.Raw {
		b, err := bson.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	idPattern := raw(bson.D{{Key: "keyPattern", Value: bson.D{{Key: "_id", Value: 1}}}})
	emailPattern := raw(bson.D{{Key: "keyPattern", Value: bson.D{{Key: "email", Value: 1}}}})
	bulk := func(we mongo.WriteError) error {
		return mongo.BulkWriteException{WriteErrors: []mongo.BulkWriteError{{WriteError: we}}}
	}
	tests := []struct {
		name      string
		err       error
		wantIndex int
		want      bool
	}{
		{"_id key pattern", bulk(mongo.WriteError{Index: 2, Code: 11000, Raw: idPattern}), 2, true},
		{"other unique index", bulk(mongo.WriteError{Code: 11000, Raw: emailPattern}), 0, false},
		{"old server message", bulk(mongo.WriteError{Index: 1, Code: 11000,
			Message: `E11000 duplicate key error collection: shop.orders index: _id_ dup key: { _id: "A1" }`}), 1, true},
		{"other write error", bulk(mongo.WriteError{Code: 121, Raw: idPattern}), 0, false},
		{"wrapped", fmt.Errorf("insert: %w", bulk(mongo.WriteError{Index: 3, Code: 11000, Raw: idPattern})), 3, true},
		{"not a write error", errors.New("boom"), 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			index, ok := duplicateID(tt.err)
			if ok != tt.want || (ok && index != tt.wantIndex) {
				t.Fatalf("duplicateID = (%d, %v), want (%d, %v)", index, ok, tt.wantIndex, tt.want)
			}
		})
	}
}
