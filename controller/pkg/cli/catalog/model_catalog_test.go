package catalog

import (
	"reflect"
	"testing"
)

func TestMergeDeepMergesModels(t *testing.T) {
	base := &ModelCatalog{Providers: map[string]Provider{
		"anthropic": {Models: map[string]Model{
			"claude": {
				Rates: Rates{Input: "3", Output: "15"},
				Tiers: []Tier{{ContextOver: 200000, Rates: Rates{Input: "6"}}},
				Tags:  []string{"chat"},
			},
		}},
	}}
	// Overlay contributes only tags and a single rate for the same model; it must
	// keep the base's other rates and its tiers rather than blanking them.
	overlay := &ModelCatalog{Providers: map[string]Provider{
		"anthropic": {Models: map[string]Model{
			"claude": {
				Rates: Rates{Output: "18"},
				Tags:  []string{"runtime", "chat"},
			},
		}},
	}}

	got := Merge(base, overlay)
	want := &ModelCatalog{Providers: map[string]Provider{
		"anthropic": {Models: map[string]Model{
			"claude": {
				Rates: Rates{Input: "3", Output: "18"},
				Tiers: []Tier{{ContextOver: 200000, Rates: Rates{Input: "6"}}},
				Tags:  []string{"chat", "runtime"},
			},
		}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Merge deep merge = %#v, want %#v", got, want)
	}
}

func TestMergeReplacesTiersAndAddsEntries(t *testing.T) {
	base := &ModelCatalog{Providers: map[string]Provider{
		"openai": {Models: map[string]Model{
			"gpt": {Tiers: []Tier{{ContextOver: 128000, Rates: Rates{Input: "10"}}}},
		}},
	}}
	overlay := &ModelCatalog{Providers: map[string]Provider{
		// New tier set on an existing model replaces wholesale.
		"openai": {Models: map[string]Model{
			"gpt": {Tiers: []Tier{{ContextOver: 200000, Rates: Rates{Input: "5"}}}},
		}},
		// Wholly new provider is carried through untouched.
		"aws.bedrock": {Models: map[string]Model{
			"nova": {Rates: Rates{Input: "1"}},
		}},
	}}

	got := Merge(base, overlay)
	want := &ModelCatalog{Providers: map[string]Provider{
		"openai": {Models: map[string]Model{
			"gpt": {Tiers: []Tier{{ContextOver: 200000, Rates: Rates{Input: "5"}}}},
		}},
		"aws.bedrock": {Models: map[string]Model{
			"nova": {Rates: Rates{Input: "1"}},
		}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Merge tiers/new = %#v, want %#v", got, want)
	}
}

func TestMergeLaterSourceWins(t *testing.T) {
	first := &ModelCatalog{Providers: map[string]Provider{
		"openai": {Models: map[string]Model{"gpt": {Rates: Rates{Input: "1"}}}},
	}}
	second := &ModelCatalog{Providers: map[string]Provider{
		"openai": {Models: map[string]Model{"gpt": {Rates: Rates{Input: "2"}}}},
	}}
	third := &ModelCatalog{Providers: map[string]Provider{
		"openai": {Models: map[string]Model{"gpt": {Rates: Rates{Input: "3"}}}},
	}}

	got := Merge(first, second, third)
	if in := got.Providers["openai"].Models["gpt"].Rates.Input; in != "3" {
		t.Fatalf("last source should win: input = %q, want %q", in, "3")
	}
}

func TestMergeSkipsNilAndDoesNotMutateInputs(t *testing.T) {
	base := &ModelCatalog{Providers: map[string]Provider{
		"openai": {Models: map[string]Model{"gpt": {Tags: []string{"a"}}}},
	}}
	overlay := &ModelCatalog{Providers: map[string]Provider{
		"openai": {Models: map[string]Model{"gpt": {Tags: []string{"b"}}}},
	}}

	got := Merge(nil, base, nil, overlay, nil)
	if tags := got.Providers["openai"].Models["gpt"].Tags; !reflect.DeepEqual(tags, []string{"a", "b"}) {
		t.Fatalf("union tags = %v, want [a b]", tags)
	}
	// The originals must be untouched so a caller can reuse a source catalog.
	if tags := base.Providers["openai"].Models["gpt"].Tags; !reflect.DeepEqual(tags, []string{"a"}) {
		t.Fatalf("base mutated: tags = %v, want [a]", tags)
	}
}
