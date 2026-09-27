package mongoclient

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type testDoc struct {
	ID        string    `bson:"_id"`
	Name      string    `bson:"name"`
	Score     int       `bson:"score"`
	CreatedAt time.Time `bson:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt"`
}

type objectIDTestDoc struct {
	ID   bson.ObjectID `bson:"_id"`
	Name string        `bson:"name"`
}

func (d *testDoc) SetCreatedAt(t time.Time) { d.CreatedAt = t }
func (d *testDoc) SetUpdatedAt(t time.Time) { d.UpdatedAt = t }

type scoreTotal struct {
	Total int `bson:"total"`
}

func TestCollectionAgainstMongoDB(t *testing.T) {
	uri := os.Getenv("MONGOCLIENT_TEST_URI")
	if uri == "" {
		t.Skip("set MONGOCLIENT_TEST_URI to run against a real MongoDB")
	}
	ctx := context.Background()

	var seq atomic.Int64
	client, err := NewConnector().Connect(ctx, uri, "mongoclient_test",
		WithPingTimeout(5*time.Second),
		WithIDGenerator(func(context.Context) (any, error) {
			return fmt.Sprintf("doc-%d", seq.Add(1)), nil
		}))
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer client.Disconnect()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}

	suffix := time.Now().UnixNano()
	coll, err := NewCollection[testDoc](client, fmt.Sprintf("docs_%d", suffix))
	if err != nil {
		t.Fatalf("NewCollection: %v", err)
	}
	defer func() { _ = coll.Raw().Drop(context.Background()) }()

	docs := []*testDoc{{Name: "a", Score: 1}, {Name: "b", Score: 2}, {Name: "c", Score: 3}}
	ids, err := coll.Insert(ctx, docs...)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if len(ids) != len(docs) {
		t.Fatalf("Insert returned %d ids, want %d", len(ids), len(docs))
	}
	for i, d := range docs {
		if d.ID != fmt.Sprintf("doc-%d", i+1) || ids[i] != d.ID {
			t.Fatalf("doc %d: ID = %q, inserted id = %v", i, d.ID, ids[i])
		}
		if d.CreatedAt.IsZero() || !d.UpdatedAt.Equal(d.CreatedAt) {
			t.Fatalf("doc %d: CreatedAt = %v, UpdatedAt = %v", i, d.CreatedAt, d.UpdatedAt)
		}
	}

	if _, err := coll.Insert(ctx, &testDoc{ID: docs[0].ID, Name: "dup", Score: 99}); !errors.Is(err, ErrDuplicateID) {
		t.Fatalf("Insert(existing id) err = %v, want ErrDuplicateID", err)
	}
	kept, err := coll.FindOne(ctx, Filter().Eq("_id", docs[0].ID))
	if err != nil || kept.Name != "a" || kept.Score != 1 {
		t.Fatalf("record after duplicate insert = %+v, err = %v", kept, err)
	}

	either, err := coll.Count(ctx, Filter().Or(Filter().Eq("name", "a"), Filter().Gte("score", 3)))
	if err != nil || either != 2 {
		t.Fatalf("Count(Or) = %d, err = %v", either, err)
	}

	got, err := coll.FindOne(ctx, Filter().Eq("_id", docs[1].ID))
	if err != nil {
		t.Fatalf("FindOne: %v", err)
	}
	if got.Name != "b" || !got.CreatedAt.Equal(docs[1].CreatedAt.Truncate(time.Millisecond)) {
		t.Fatalf("FindOne = %+v", got)
	}
	if _, err := coll.FindOne(ctx, bson.M{"name": "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("FindOne(missing) err = %v, want ErrNotFound", err)
	}

	byScore := options.Find().SetSort(bson.D{{Key: "score", Value: -1}})
	page, err := coll.Find(ctx, bson.M{}, byScore.SetSkip(1).SetLimit(1))
	if err != nil || len(page) != 1 || page[0].Name != "b" {
		t.Fatalf("Find offset page = %+v, err = %v", page, err)
	}
	next, err := coll.Find(ctx, bson.M{"score": bson.M{"$lt": 3}},
		options.Find().SetSort(bson.D{{Key: "score", Value: -1}}).SetLimit(1))
	if err != nil || len(next) != 1 || next[0].Name != "b" {
		t.Fatalf("Find cursor page = %+v, err = %v", next, err)
	}
	none, err := coll.Find(ctx, bson.M{"name": "missing"})
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("Find(missing) = %#v, err = %v", none, err)
	}

	count, err := coll.Count(ctx, bson.M{"score": bson.M{"$gte": 2}})
	if err != nil || count != 2 {
		t.Fatalf("Count = %d, err = %v", count, err)
	}

	matched, modified, err := coll.Update(ctx, bson.M{"score": bson.M{"$gte": 2}}, bson.M{"$inc": bson.M{"score": 10}})
	if err != nil || matched != 2 || modified != 2 {
		t.Fatalf("Update matched = %d, modified = %d, err = %v", matched, modified, err)
	}
	matched, _, err = coll.Update(ctx, bson.M{"name": "missing"}, bson.M{"$set": bson.M{"score": 0}})
	if err != nil || matched != 0 {
		t.Fatalf("Update(missing) matched = %d, err = %v", matched, err)
	}

	totals, err := Aggregate[scoreTotal](ctx, coll, bson.A{
		bson.M{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$score"}}},
	})
	if err != nil || len(totals) != 1 || totals[0].Total != 26 {
		t.Fatalf("Aggregate = %+v, err = %v", totals, err)
	}

	deleted, err := coll.Delete(ctx, bson.M{"name": bson.M{"$in": bson.A{"a", "b"}}})
	if err != nil || deleted != 2 {
		t.Fatalf("Delete deleted = %d, err = %v", deleted, err)
	}
	left, err := coll.Count(ctx, bson.M{})
	if err != nil || left != 1 {
		t.Fatalf("Count after Delete = %d, err = %v", left, err)
	}

	plain, err := NewConnector().Connect(ctx, uri, "mongoclient_test", WithPingTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("Connect without id generator: %v", err)
	}
	defer plain.Disconnect()
	oids, err := NewCollection[objectIDTestDoc](plain, fmt.Sprintf("oids_%d", suffix))
	if err != nil {
		t.Fatalf("NewCollection without id generator: %v", err)
	}
	defer func() { _ = oids.Raw().Drop(context.Background()) }()
	doc := &objectIDTestDoc{Name: "x"}
	if _, err := oids.Insert(ctx, doc); err != nil {
		t.Fatalf("Insert without id generator: %v", err)
	}
	if doc.ID.IsZero() {
		t.Fatal("Insert without id generator left the ObjectID empty")
	}
	found, err := oids.FindOne(ctx, Filter().Eq("_id", doc.ID))
	if err != nil || found.Name != "x" {
		t.Fatalf("FindOne(ObjectID) = %+v, err = %v", found, err)
	}
}
