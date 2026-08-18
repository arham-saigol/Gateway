package accounting

import "math"

const (
	TokensPerMillion = 1000000
)

// CalculateAttemptCost calculates exact total cost in micro-USD.
// Rates are integer micro-USD per 1,000,000 tokens.
// Returns (costMicroUSD, true) if tokens are known and valid (>= 0), or (0, false) if unknown.
func CalculateAttemptCost(inputTokens, cachedInputTokens, outputTokens, inputRate, cachedRate, outputRate int64) (int64, bool) {
	if inputTokens < 0 || cachedInputTokens < 0 || outputTokens < 0 || inputRate < 0 || cachedRate < 0 || outputRate < 0 {
		return 0, false
	}

	uncachedInput := inputTokens - cachedInputTokens
	if uncachedInput < 0 {
		uncachedInput = 0
	}

	// Cost calculation with standard integer half-up rounding:
	// cost = (tokens * rate + 500,000) / 1,000,000
	inputCost, ok := roundedCost(uncachedInput, inputRate)
	if !ok {
		return 0, false
	}
	cachedCost, ok := roundedCost(cachedInputTokens, cachedRate)
	if !ok {
		return 0, false
	}
	outputCost, ok := roundedCost(outputTokens, outputRate)
	if !ok || inputCost > math.MaxInt64-cachedCost || inputCost+cachedCost > math.MaxInt64-outputCost {
		return 0, false
	}

	return inputCost + cachedCost + outputCost, true
}

func roundedCost(tokens, rate int64) (int64, bool) {
	if rate != 0 && tokens > (math.MaxInt64-TokensPerMillion/2)/rate {
		return 0, false
	}
	return (tokens*rate + TokensPerMillion/2) / TokensPerMillion, true
}
