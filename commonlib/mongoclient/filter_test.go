package mongoclient

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func assertBSON(t *testing.T, got any, want bson.D) {
	t.Helper()
	gotBytes, err := bson.Marshal(got)
	if err != nil {
		t.Fatalf("marshal filter: %v", err)
	}
	wantBytes, err := bson.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBytes, wantBytes) {
		t.Fatalf("filter = %s\nwant     %s", bson.Raw(gotBytes), bson.Raw(wantBytes))
	}
}

func eqDoc(field string, value any) bson.D {
	return bson.D{{Key: field, Value: bson.D{{Key: "$eq", Value: value}}}}
}

func TestFilterComparisons(t *testing.T) {
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	assertBSON(t, Filter().
		Eq("customer", "c1").
		Gte("createdAt", from).
		Lt("createdAt", to).
		Ne("status", "cancelled").
		Gt("total", 100).
		Lte("items", 5).
		In("topic_types", 3, 4).
		Nin("last_status", []int{4, 5}),
		bson.D{
			{Key: "customer", Value: bson.D{{Key: "$eq", Value: "c1"}}},
			{Key: "createdAt", Value: bson.D{{Key: "$gte", Value: from}, {Key: "$lt", Value: to}}},
			{Key: "status", Value: bson.D{{Key: "$ne", Value: "cancelled"}}},
			{Key: "total", Value: bson.D{{Key: "$gt", Value: 100}}},
			{Key: "items", Value: bson.D{{Key: "$lte", Value: 5}}},
			{Key: "topic_types", Value: bson.D{{Key: "$in", Value: bson.A{3, 4}}}},
			{Key: "last_status", Value: bson.D{{Key: "$nin", Value: bson.A{4, 5}}}},
		})
}

func TestFilterArrayRegexExists(t *testing.T) {
	assertBSON(t, Filter().
		All("tags", "go", "mongo").
		Size("tags", 2).
		ElemMatch("items", Filter().Eq("sku", "A1").Gte("qty", 2)).
		Regex("name", "^tech", "i").
		Regex("code", "^X", "").
		Exists("deletedAt", false),
		bson.D{
			{Key: "tags", Value: bson.D{{Key: "$all", Value: bson.A{"go", "mongo"}}, {Key: "$size", Value: 2}}},
			{Key: "items", Value: bson.D{{Key: "$elemMatch", Value: bson.D{
				{Key: "sku", Value: bson.D{{Key: "$eq", Value: "A1"}}},
				{Key: "qty", Value: bson.D{{Key: "$gte", Value: 2}}},
			}}}},
			{Key: "name", Value: bson.D{{Key: "$regex", Value: "^tech"}, {Key: "$options", Value: "i"}}},
			{Key: "code", Value: bson.D{{Key: "$regex", Value: "^X"}}},
			{Key: "deletedAt", Value: bson.D{{Key: "$exists", Value: false}}},
		})
}

func TestFilterLogical(t *testing.T) {
	assertBSON(t, Filter().
		Eq("platform", 10).
		Or(Filter().Eq("priority", 1), Filter().Gte("subscriber_count", 10000)),
		bson.D{
			{Key: "platform", Value: bson.D{{Key: "$eq", Value: 10}}},
			{Key: "$or", Value: bson.A{
				eqDoc("priority", 1),
				bson.D{{Key: "subscriber_count", Value: bson.D{{Key: "$gte", Value: 10000}}}},
			}},
		})

	assertBSON(t, Filter().
		Or(Filter().Eq("a", 1), Filter().Eq("b", 1)).
		Or(Filter().Eq("c", 1), Filter().Eq("d", 1)).
		And(Filter().Eq("e", 1)),
		bson.D{
			{Key: "$and", Value: bson.A{
				eqDoc("e", 1),
				bson.D{{Key: "$or", Value: bson.A{eqDoc("a", 1), eqDoc("b", 1)}}},
				bson.D{{Key: "$or", Value: bson.A{eqDoc("c", 1), eqDoc("d", 1)}}},
			}},
		})

	assertBSON(t, Filter().Nor(Filter().Eq("a", 1)).Nor(Filter().Eq("b", 1)),
		bson.D{{Key: "$nor", Value: bson.A{eqDoc("a", 1), eqDoc("b", 1)}}})
}

func TestFilterInvalidGroups(t *testing.T) {
	tests := map[string]*FilterBuilder{
		"empty Or":         Filter().Or(),
		"nil in And":       Filter().And(nil),
		"empty Nor":        Filter().Nor(),
		"nil ElemMatch":    Filter().ElemMatch("items", nil),
		"invalid nested":   Filter().Or(Filter().Or()),
		"first error kept": Filter().Or().Eq("a", 1),
	}
	for name, f := range tests {
		if _, err := bson.Marshal(f); err == nil {
			t.Errorf("%s: want a marshal error", name)
		}
	}
}

func TestFilterBuilderWithUpdateAndDelete(t *testing.T) {
	coll := &Collection[unitDoc]{}
	ctx := context.Background()
	if _, _, err := coll.Update(ctx, Filter(), bson.M{"$set": bson.M{"name": "x"}}); !errors.Is(err, ErrEmptyFilter) {
		t.Errorf("Update(empty builder) err = %v, want ErrEmptyFilter", err)
	}
	if _, err := coll.Delete(ctx, Filter()); !errors.Is(err, ErrEmptyFilter) {
		t.Errorf("Delete(empty builder) err = %v, want ErrEmptyFilter", err)
	}
	if err := checkFilter(Filter().Eq("name", "x")); err != nil {
		t.Errorf("checkFilter(non-empty builder) = %v", err)
	}
}
