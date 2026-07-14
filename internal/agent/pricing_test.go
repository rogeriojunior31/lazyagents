package agent

import "testing"

func TestEstimateCost(t *testing.T) {
	u := Usage{Input: 1_000_000, Output: 1_000_000, CacheRead: 1_000_000, CacheWrite: 1_000_000, Model: "claude-sonnet-4-5-20250929"}
	cost, ok := EstimateCost(u)
	if !ok {
		t.Fatal("modelo conhecido deveria estimar custo")
	}
	want := 3.0 + 15.0 + 0.3 + 3.75
	if cost != want {
		t.Fatalf("custo = %v, quer %v", cost, want)
	}

	if _, ok := EstimateCost(Usage{Model: "modelo-desconhecido-xyz"}); ok {
		t.Fatal("modelo desconhecido deveria ser ok=false")
	}
	if _, ok := EstimateCost(Usage{}); ok {
		t.Fatal("sem modelo deveria ser ok=false")
	}
}
