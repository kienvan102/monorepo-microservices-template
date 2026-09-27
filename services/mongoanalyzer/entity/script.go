package entity

import "encoding/json"

// Valid plan groups: which scripts a plan.json group selects.
const (
	PlanGroupInspect = "inspect"
	PlanGroupDefault = "default"
)

// ScriptResult is what one analysis script produced.
type ScriptResult struct {
	Script         string            `json:"script"`
	DurationMs     int64             `json:"duration_ms"`
	Output         json.RawMessage   `json:"output_json,omitempty"`
	PartialResults []json.RawMessage `json:"partial_results,omitempty"`
	Stdout         string            `json:"stdout,omitempty"`
}
