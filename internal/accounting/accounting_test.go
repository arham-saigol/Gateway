package accounting_test

import (
	"math"
	"testing"

	"arham-gateway/internal/accounting"
)

func TestCostOverflowIsUnavailable(t *testing.T) {
	if _, ok := accounting.CalculateAttemptCost(math.MaxInt64, 0, 0, math.MaxInt64, 0, 0); ok {
		t.Fatal("expected overflowing cost to be unavailable")
	}
}

func TestCalculateCostExactIntegerArithmetic(t *testing.T) {
	// Scenario:
	// Rates in micro-USD per 1M tokens:
	// Input rate:  $0.14/M = 140,000 micro-USD
	// Cached rate: $0.014/M = 14,000 micro-USD
	// Output rate: $0.28/M = 280,000 micro-USD
	inputRate := int64(140000)
	cachedRate := int64(14000)
	outputRate := int64(280000)

	// Case 1: 1,000,000 uncached input, 0 cached, 1,000,000 output
	// Expected cost: $0.14 + $0.28 = $0.42 = 420,000 micro-USD
	cost, ok := accounting.CalculateAttemptCost(1000000, 0, 1000000, inputRate, cachedRate, outputRate)
	if !ok {
		t.Fatalf("expected calculation to succeed")
	}
	if cost != 420000 {
		t.Errorf("expected 420000 micro-USD, got %d", cost)
	}

	// Case 2: 10,000 input tokens where 8,000 are cached, 2,000 uncached; 1,000 output
	// uncached input = 2,000 tokens * 140,000 / 1,000,000 = 280 micro-USD
	// cached input = 8,000 tokens * 14,000 / 1,000,000 = 112 micro-USD
	// output = 1,000 tokens * 280,000 / 1,000,000 = 280 micro-USD
	// Total = 280 + 112 + 280 = 672 micro-USD ($0.000672)
	cost, ok = accounting.CalculateAttemptCost(10000, 8000, 1000, inputRate, cachedRate, outputRate)
	if !ok {
		t.Fatalf("expected calculation to succeed")
	}
	if cost != 672 {
		t.Errorf("expected 672 micro-USD, got %d", cost)
	}

	// Case 3: Cached tokens greater than total input tokens should clamp uncached to 0
	cost, ok = accounting.CalculateAttemptCost(500, 1000, 500, inputRate, cachedRate, outputRate)
	if !ok {
		t.Fatalf("expected calculation to succeed")
	}
	// uncached = 0
	// cached = 1000 * 14,000 / 1,000,000 = 14 micro-USD
	// output = 500 * 280,000 / 1,000,000 = 140 micro-USD
	// Total = 154 micro-USD
	if cost != 154 {
		t.Errorf("expected 154 micro-USD, got %d", cost)
	}
}

func TestUnknownUsageReturnsNilCost(t *testing.T) {
	_, ok := accounting.CalculateAttemptCost(-1, 0, 0, 140000, 14000, 280000)
	if ok {
		t.Errorf("expected negative input tokens to be rejected as invalid/unknown")
	}
}
