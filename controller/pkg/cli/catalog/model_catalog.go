package catalog

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
)

type ModelCatalog struct {
	Providers map[string]Provider `json:"providers"`
}

func (c *ModelCatalog) Validate() error {
	for provider, p := range c.Providers {
		for model, m := range p.Models {
			if err := m.validate(); err != nil {
				return fmt.Errorf("%s/%s: %w", provider, model, err)
			}
		}
	}
	return nil
}

// Merge overlays each catalog onto the previous, left to right, so later
// catalogs take precedence. Merging is deep and per field: a source that
// contributes only tags (or only some rates) keeps the values earlier sources
// set for everything it leaves unspecified. This mirrors the data-plane merge
// in crates/agentgateway/src/llm/catalog/model.rs (Catalog::override_with) so
// the catalog agctl emits and the one the proxy assembles agree.
func Merge(catalogs ...*ModelCatalog) *ModelCatalog {
	out := &ModelCatalog{Providers: map[string]Provider{}}
	for _, cat := range catalogs {
		if cat != nil {
			out.overrideWith(cat)
		}
	}
	return out
}

func (c *ModelCatalog) overrideWith(overlay *ModelCatalog) {
	for pid, op := range overlay.Providers {
		base, ok := c.Providers[pid]
		if !ok || base.Models == nil {
			base = Provider{Models: map[string]Model{}}
		}
		for mid, om := range op.Models {
			base.Models[mid] = base.Models[mid].overrideWith(om)
		}
		c.Providers[pid] = base
	}
}

type Provider struct {
	Models map[string]Model `json:"models"`
}

type Model struct {
	Rates Rates    `json:"rates,omitzero"`
	Tiers []Tier   `json:"tiers,omitempty"`
	Tags  []string `json:"tags,omitempty"`
}

func (m Model) IsZero() bool {
	return m.Rates.IsZero() && len(m.Tiers) == 0 && len(m.Tags) == 0
}

// overrideWith deep-merges overlay onto m: rates are overlaid field by field,
// a non-empty tier set replaces m's tiers wholesale (tiers are ordered and only
// meaningful together), and tags are unioned.
func (m Model) overrideWith(overlay Model) Model {
	m.Rates = m.Rates.overlay(overlay.Rates)
	if len(overlay.Tiers) > 0 {
		m.Tiers = overlay.Tiers
	}
	if len(overlay.Tags) > 0 {
		m.Tags = mergeTags(m.Tags, overlay.Tags)
	}
	return m
}

// mergeTags returns the sorted, de-duplicated union of two tag lists.
func mergeTags(base, overlay []string) []string {
	seen := make(map[string]struct{}, len(base)+len(overlay))
	out := make([]string, 0, len(base)+len(overlay))
	for _, tag := range slices.Concat(base, overlay) {
		if _, dup := seen[tag]; dup {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	slices.Sort(out)
	return out
}

type Rates struct {
	Input       Money `json:"input,omitempty"`
	Output      Money `json:"output,omitempty"`
	CacheRead   Money `json:"cacheRead,omitempty"`
	CacheWrite  Money `json:"cacheWrite,omitempty"`
	Reasoning   Money `json:"reasoning,omitempty"`
	InputAudio  Money `json:"inputAudio,omitempty"`
	OutputAudio Money `json:"outputAudio,omitempty"`
}

type Tier struct {
	ContextOver uint64 `json:"contextOver"`
	Rates       Rates  `json:"rates,omitzero"`
}

type Money string

func (r Rates) IsZero() bool {
	return r == Rates{}
}

// overlay returns r with every rate that delta sets replaced by delta's value;
// rates delta leaves empty fall through to r.
func (r Rates) overlay(delta Rates) Rates {
	pick := func(base, d Money) Money {
		if d != "" {
			return d
		}
		return base
	}
	return Rates{
		Input:       pick(r.Input, delta.Input),
		Output:      pick(r.Output, delta.Output),
		CacheRead:   pick(r.CacheRead, delta.CacheRead),
		CacheWrite:  pick(r.CacheWrite, delta.CacheWrite),
		Reasoning:   pick(r.Reasoning, delta.Reasoning),
		InputAudio:  pick(r.InputAudio, delta.InputAudio),
		OutputAudio: pick(r.OutputAudio, delta.OutputAudio),
	}
}

func (m Money) Decimal() (decimal.Decimal, error) {
	if m == "" {
		return decimal.Zero, nil
	}
	d, err := decimal.NewFromString(string(m))
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid money %q: %w", string(m), err)
	}
	return d, nil
}

// maxFractionalDigits bounds rate precision. Money is exact decimal, never float;
// rates are USD per 1,000,000 tokens and never need more than micro-dollar precision.
const maxFractionalDigits = 6

func (m Money) validate() error {
	if m == "" {
		return nil
	}
	d, err := m.Decimal()
	if err != nil {
		return err
	}
	if d.IsNegative() {
		return fmt.Errorf("money %q is negative", string(m))
	}
	if d.Exponent() < -maxFractionalDigits {
		return fmt.Errorf("money %q exceeds %d fractional digits", string(m), maxFractionalDigits)
	}
	for _, r := range string(m) {
		if r == 'e' || r == 'E' {
			return fmt.Errorf("money %q uses scientific notation", string(m))
		}
	}
	return nil
}

func (m *Model) validate() error {
	if err := m.Rates.validate(); err != nil {
		return err
	}
	var prev uint64
	for i, t := range m.Tiers {
		if i > 0 && t.ContextOver <= prev {
			return fmt.Errorf("tier %d threshold %d not strictly greater than previous %d", i, t.ContextOver, prev)
		}
		prev = t.ContextOver
		if err := t.Rates.validate(); err != nil {
			return fmt.Errorf("tier %d: %w", i, err)
		}
	}
	return nil
}

func (r *Rates) validate() error {
	v := reflect.ValueOf(*r)
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		m, ok := reflect.TypeAssert[Money](v.Field(i))
		if !ok {
			continue
		}
		if err := m.validate(); err != nil {
			return fmt.Errorf("rate %s: %w", jsonFieldName(t.Field(i)), err)
		}
	}
	return nil
}

func jsonFieldName(field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	if name == "" {
		return field.Name
	}
	return name
}

func marshalCatalog(cat *ModelCatalog, pretty bool) ([]byte, error) {
	marshal := json.Marshal
	if pretty {
		marshal = func(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }
	}
	data, err := marshal(cat)
	if err != nil {
		return nil, fmt.Errorf("marshal catalog: %w", err)
	}
	return append(data, '\n'), nil
}
