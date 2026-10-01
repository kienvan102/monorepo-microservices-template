package mongoclient

import (
	"fmt"
	"reflect"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type FilterBuilder struct {
	fields []string
	conds  map[string]bson.D
	ors    [][]*FilterBuilder
	ands   []*FilterBuilder
	nors   []*FilterBuilder
	err    error
}

func Filter() *FilterBuilder {
	return &FilterBuilder{conds: map[string]bson.D{}}
}

func (f *FilterBuilder) Eq(field string, value any) *FilterBuilder {
	return f.add(field, "$eq", value)
}

func (f *FilterBuilder) Ne(field string, value any) *FilterBuilder {
	return f.add(field, "$ne", value)
}

func (f *FilterBuilder) Gt(field string, value any) *FilterBuilder {
	return f.add(field, "$gt", value)
}

func (f *FilterBuilder) Gte(field string, value any) *FilterBuilder {
	return f.add(field, "$gte", value)
}

func (f *FilterBuilder) Lt(field string, value any) *FilterBuilder {
	return f.add(field, "$lt", value)
}

func (f *FilterBuilder) Lte(field string, value any) *FilterBuilder {
	return f.add(field, "$lte", value)
}

func (f *FilterBuilder) In(field string, values ...any) *FilterBuilder {
	return f.add(field, "$in", list(values))
}

func (f *FilterBuilder) Nin(field string, values ...any) *FilterBuilder {
	return f.add(field, "$nin", list(values))
}

func (f *FilterBuilder) All(field string, values ...any) *FilterBuilder {
	return f.add(field, "$all", list(values))
}

func (f *FilterBuilder) Exists(field string, exists bool) *FilterBuilder {
	return f.add(field, "$exists", exists)
}

func (f *FilterBuilder) Size(field string, n int) *FilterBuilder {
	return f.add(field, "$size", n)
}

func (f *FilterBuilder) Regex(field, pattern, options string) *FilterBuilder {
	f.add(field, "$regex", pattern)
	if options != "" {
		f.add(field, "$options", options)
	}
	return f
}

func (f *FilterBuilder) ElemMatch(field string, cond *FilterBuilder) *FilterBuilder {
	if cond == nil {
		f.setErr(fmt.Errorf("mongoclient: ElemMatch on %q needs a condition", field))
		return f
	}
	return f.add(field, "$elemMatch", cond)
}

func (f *FilterBuilder) Or(conds ...*FilterBuilder) *FilterBuilder {
	if err := checkGroup("Or", conds); err != nil {
		f.setErr(err)
		return f
	}
	f.ors = append(f.ors, conds)
	return f
}

func (f *FilterBuilder) And(conds ...*FilterBuilder) *FilterBuilder {
	if err := checkGroup("And", conds); err != nil {
		f.setErr(err)
		return f
	}
	f.ands = append(f.ands, conds...)
	return f
}

func (f *FilterBuilder) Nor(conds ...*FilterBuilder) *FilterBuilder {
	if err := checkGroup("Nor", conds); err != nil {
		f.setErr(err)
		return f
	}
	f.nors = append(f.nors, conds...)
	return f
}

func (f *FilterBuilder) MarshalBSON() ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	doc := make(bson.D, 0, len(f.fields)+3)
	for _, field := range f.fields {
		doc = append(doc, bson.E{Key: field, Value: f.conds[field]})
	}
	ands := group(f.ands)
	switch len(f.ors) {
	case 0:
	case 1:
		doc = append(doc, bson.E{Key: "$or", Value: group(f.ors[0])})
	default:
		for _, conds := range f.ors {
			ands = append(ands, bson.D{{Key: "$or", Value: group(conds)}})
		}
	}
	if len(ands) > 0 {
		doc = append(doc, bson.E{Key: "$and", Value: ands})
	}
	if len(f.nors) > 0 {
		doc = append(doc, bson.E{Key: "$nor", Value: group(f.nors)})
	}
	return bson.Marshal(doc)
}

func (f *FilterBuilder) add(field, op string, value any) *FilterBuilder {
	if _, ok := f.conds[field]; !ok {
		f.fields = append(f.fields, field)
	}
	f.conds[field] = append(f.conds[field], bson.E{Key: op, Value: value})
	return f
}

func (f *FilterBuilder) setErr(err error) {
	if f.err == nil {
		f.err = err
	}
}

func checkGroup(op string, conds []*FilterBuilder) error {
	if len(conds) == 0 {
		return fmt.Errorf("mongoclient: %s needs at least one condition", op)
	}
	for _, c := range conds {
		if c == nil {
			return fmt.Errorf("mongoclient: %s got a nil condition", op)
		}
	}
	return nil
}

func group(conds []*FilterBuilder) bson.A {
	a := make(bson.A, 0, len(conds))
	for _, c := range conds {
		a = append(a, c)
	}
	return a
}

func list(values []any) any {
	if len(values) == 1 {
		v := reflect.ValueOf(values[0])
		if (v.Kind() == reflect.Slice || v.Kind() == reflect.Array) && v.Type().Elem().Kind() != reflect.Uint8 {
			return values[0]
		}
	}
	return values
}
