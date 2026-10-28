package domain

import (
	"math"
	"testing"
	"time"
)

func TestSumCharges(t *testing.T) {
	total, err := SumCharges(700000, 150000, 100000)
	if err != nil || total != 950000 {
		t.Fatalf("SumCharges = %d, %v", total, err)
	}
	if _, err := SumCharges(math.MaxInt64, 1); err == nil {
		t.Fatal("expected overflow error")
	}
	if _, err := SumCharges(-1, 2); err == nil {
		t.Fatal("expected negative charge error")
	}
}

func TestRankLatestUsesOneLatestBidPerSupplier(t *testing.T) {
	base := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	bids := []Bid{
		{ID: 1, SupplierID: "b", TotalMinor: 900, SubmittedAt: base},
		{ID: 2, SupplierID: "a", TotalMinor: 1000, SubmittedAt: base},
		{ID: 3, SupplierID: "a", TotalMinor: 800, SubmittedAt: base.Add(time.Minute)},
		{ID: 4, SupplierID: "c", TotalMinor: 800, SubmittedAt: base},
	}
	ranked := RankLatest(bids)
	if len(ranked) != 3 {
		t.Fatalf("got %d ranks", len(ranked))
	}
	want := []string{"c", "a", "b"}
	for index, supplier := range want {
		if ranked[index].Rank != index+1 || ranked[index].Bid.SupplierID != supplier {
			t.Fatalf("rank %d = %+v, want supplier %s", index+1, ranked[index], supplier)
		}
	}
}

func TestRankLatestBreaksExactTiesBySupplierID(t *testing.T) {
	submittedAt := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	ranked := RankLatest([]Bid{
		{ID: 1, SupplierID: "supplier-b", TotalMinor: 100, SubmittedAt: submittedAt},
		{ID: 2, SupplierID: "supplier-a", TotalMinor: 100, SubmittedAt: submittedAt},
	})
	if ranked[0].Bid.SupplierID != "supplier-a" || ranked[1].Bid.SupplierID != "supplier-b" {
		t.Fatalf("tie ranking = %+v", ranked)
	}
}
