package accounting

const (
	TokensPerMillion = 1000000
)

// CalculateAttemptCost calculates exact total cost in micro-USD.
// Rates are integer micro-USD per 1,000,000 tokens.
// Returns (costMicroUSD, true) if tokens are known and valid (>= 0), or (0, false) if unknown.
func CalculateAttemptCost(inputTokens, cachedInputTokens, outputTokens, inputRate, cachedRate, outputRate int64) (int64, bool) {
	if inputTokens < 0 || cachedInputTokens < 0 || outputTokens < 0 {
		return 0, false
	}

	uncachedInput := inputTokens - cachedInputTokens
	if uncachedInput < 0 {
		uncachedInput = 0
	}

	// Cost calculation with standard integer half-up rounding:
	// cost = (tokens * rate + 500,000) / 1,000,000
	inputCost := (uncachedInput*inputRate + TokensPerMillion/2) / TokensPerMillion
	cachedCost := (cachedInputTokens*cachedRate + TokensPerMillion/2) / TokensPerMillion
	outputCost := (outputTokens*outputRate + TokensPerMillion/2) / TokensPerMillion

	totalCost := inputCost + cachedCost + outputCost
	return totalCost, true
}
