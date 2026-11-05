package entity

type Metric struct {
	Name  string
	Value float64
}

type Result struct {
	ID         string
	DurationMS int64
	Metrics    []Metric
	Err        error
}
