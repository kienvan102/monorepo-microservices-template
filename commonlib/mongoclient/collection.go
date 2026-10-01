package mongoclient

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	ErrNotFound          = errors.New("mongoclient: document not found")
	ErrEmptyFilter       = errors.New("mongoclient: empty filter")
	ErrNoDocuments       = errors.New("mongoclient: no documents to insert")
	ErrUnsupportedClient = errors.New("mongoclient: client was not created by Connector.Connect")
	ErrIDType            = errors.New("mongoclient: id does not fit the _id field")
	ErrDuplicateID       = errors.New("mongoclient: document id already exists")
)

var objectIDType = reflect.TypeFor[bson.ObjectID]()

type IDGenerator func(ctx context.Context) (any, error)

type IDSetter interface{ SetID(id any) }

type CreatedAtSetter interface{ SetCreatedAt(time.Time) }

type UpdatedAtSetter interface{ SetUpdatedAt(time.Time) }

func RawCollection(c Client, name string) (*mongo.Collection, error) {
	dc, ok := c.(driverClient)
	if !ok {
		return nil, ErrUnsupportedClient
	}
	return dc.database.Collection(name), nil
}

type Collection[T any] struct {
	coll  *mongo.Collection
	id    *idField
	newID IDGenerator
}

type idField struct {
	index []int
	name  string
	typ   reflect.Type
}

func NewCollection[T any](c Client, name string) (*Collection[T], error) {
	dc, ok := c.(driverClient)
	if !ok {
		return nil, ErrUnsupportedClient
	}
	t := reflect.TypeFor[T]()
	id := findIDField(t)
	if id != nil && dc.newID == nil && !objectIDType.AssignableTo(id.typ) {
		return nil, fmt.Errorf("%w: %s.%s is %s and no id generator is configured; change the field to bson.ObjectID or pass mongoclient.WithIDGenerator to Connect",
			ErrIDType, t.Name(), id.name, id.typ)
	}
	return &Collection[T]{coll: dc.database.Collection(name), id: id, newID: dc.newID}, nil
}

func (c *Collection[T]) FindOne(ctx context.Context, filter any, opts ...options.Lister[options.FindOneOptions]) (T, error) {
	var doc T
	err := c.coll.FindOne(ctx, filter, opts...).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		var zero T
		return zero, ErrNotFound
	}
	if err != nil {
		var zero T
		return zero, err
	}
	return doc, nil
}

func (c *Collection[T]) Find(ctx context.Context, filter any, opts ...options.Lister[options.FindOptions]) ([]T, error) {
	cursor, err := c.coll.Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	docs := []T{}
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

func (c *Collection[T]) Count(ctx context.Context, filter any, opts ...options.Lister[options.CountOptions]) (int64, error) {
	return c.coll.CountDocuments(ctx, filter, opts...)
}

func (c *Collection[T]) Insert(ctx context.Context, docs ...*T) ([]any, error) {
	if len(docs) == 0 {
		return nil, ErrNoDocuments
	}
	values, err := c.prepare(ctx, docs)
	if err != nil {
		return nil, err
	}
	res, err := c.coll.InsertMany(ctx, values)
	if err != nil {
		if index, ok := duplicateID(err); ok {
			return nil, fmt.Errorf("%w (document %d): %w", ErrDuplicateID, index, err)
		}
		return nil, err
	}
	for i, id := range res.InsertedIDs {
		if s, ok := any(docs[i]).(IDSetter); ok {
			s.SetID(id)
		}
	}
	return res.InsertedIDs, nil
}

func (c *Collection[T]) Update(ctx context.Context, filter, update any, opts ...options.Lister[options.UpdateManyOptions]) (matched, modified int64, err error) {
	if err := checkFilter(filter); err != nil {
		return 0, 0, err
	}
	res, err := c.coll.UpdateMany(ctx, filter, update, opts...)
	if err != nil {
		return 0, 0, err
	}
	return res.MatchedCount, res.ModifiedCount, nil
}

func (c *Collection[T]) Delete(ctx context.Context, filter any, opts ...options.Lister[options.DeleteManyOptions]) (deleted int64, err error) {
	if err := checkFilter(filter); err != nil {
		return 0, err
	}
	res, err := c.coll.DeleteMany(ctx, filter, opts...)
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

func (c *Collection[T]) Raw() *mongo.Collection { return c.coll }

func Aggregate[R, T any](ctx context.Context, c *Collection[T], pipeline any, opts ...options.Lister[options.AggregateOptions]) ([]R, error) {
	cursor, err := c.coll.Aggregate(ctx, pipeline, opts...)
	if err != nil {
		return nil, err
	}
	results := []R{}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (c *Collection[T]) prepare(ctx context.Context, docs []*T) ([]any, error) {
	now := time.Now().UTC()
	values := make([]any, len(docs))
	for i, doc := range docs {
		if doc == nil {
			return nil, fmt.Errorf("mongoclient: document %d is nil", i)
		}
		if err := c.ensureID(ctx, doc); err != nil {
			return nil, fmt.Errorf("document %d: %w", i, err)
		}
		if s, ok := any(doc).(CreatedAtSetter); ok {
			s.SetCreatedAt(now)
		}
		if s, ok := any(doc).(UpdatedAtSetter); ok {
			s.SetUpdatedAt(now)
		}
		values[i] = doc
	}
	return values, nil
}

func (c *Collection[T]) ensureID(ctx context.Context, doc *T) error {
	if c.id == nil {
		return nil
	}
	field := reflect.ValueOf(doc).Elem().FieldByIndex(c.id.index)
	if !field.IsZero() {
		return nil
	}
	if c.newID == nil {
		if !objectIDType.AssignableTo(field.Type()) {
			return fmt.Errorf("%w: %s is %s and no id generator is configured", ErrIDType, c.id.name, field.Type())
		}
		field.Set(reflect.ValueOf(bson.NewObjectID()))
		return nil
	}
	id, err := c.newID(ctx)
	if err != nil {
		return fmt.Errorf("generate id: %w", err)
	}
	return setID(field, id)
}

func setID(field reflect.Value, id any) error {
	v := reflect.ValueOf(id)
	if !v.IsValid() || v.IsZero() {
		return fmt.Errorf("%w: the id generator returned an empty id", ErrIDType)
	}
	switch {
	case v.Type().AssignableTo(field.Type()):
		field.Set(v)
	case v.Kind() == field.Kind() && v.Type().ConvertibleTo(field.Type()):
		field.Set(v.Convert(field.Type()))
	default:
		return fmt.Errorf("%w: the id generator returned %T, the field is %s", ErrIDType, id, field.Type())
	}
	return nil
}

func findIDField(t reflect.Type) *idField {
	if t.Kind() != reflect.Struct {
		return nil
	}
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name, opts, _ := strings.Cut(f.Tag.Get("bson"), ",")
		if name == "_id" {
			return &idField{index: []int{i}, name: f.Name, typ: f.Type}
		}
		if f.Type.Kind() == reflect.Struct && hasTagOption(opts, "inline") {
			if sub := findIDField(f.Type); sub != nil {
				return &idField{index: append([]int{i}, sub.index...), name: f.Name + "." + sub.name, typ: sub.typ}
			}
		}
	}
	return nil
}

func hasTagOption(opts, want string) bool {
	for _, opt := range strings.Split(opts, ",") {
		if opt == want {
			return true
		}
	}
	return false
}

func duplicateID(err error) (int, bool) {
	var bwe mongo.BulkWriteException
	if !errors.As(err, &bwe) {
		return 0, false
	}
	for _, we := range bwe.WriteErrors {
		if we.Code == 11000 && isIDKey(we.WriteError) {
			return we.Index, true
		}
	}
	return 0, false
}

func isIDKey(we mongo.WriteError) bool {
	if pattern, err := we.Raw.LookupErr("keyPattern"); err == nil {
		if doc, ok := pattern.DocumentOK(); ok {
			_, err := doc.LookupErr("_id")
			return err == nil
		}
	}
	return strings.Contains(we.Message, "index: _id_ ")
}

func checkFilter(filter any) error {
	if filter == nil {
		return ErrEmptyFilter
	}
	raw, err := bson.Marshal(filter)
	if err != nil {
		return err
	}
	elements, err := bson.Raw(raw).Elements()
	if err != nil {
		return err
	}
	if len(elements) == 0 {
		return ErrEmptyFilter
	}
	return nil
}
