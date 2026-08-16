package accounting

import (
	"fmt"
	"strings"
)

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

// FormatMicroUSD formats an amount in micro-USD (1 USD = 1,000,000 micro-USD) into a human-readable USD string.
func FormatMicroUSD(microUSD int64) string {
	isNegative := false
	if microUSD < 0 {
		isNegative = true
		microUSD = -microUSD
	}

	dollars := microUSD / 1000000
	micros := microUSD % 1000000

	var formatted string
	if micros == 0 {
		formatted = fmt.Sprintf("$%d.00", dollars)
	} else if micros%10000 == 0 {
		// Exactly two decimals
		formatted = fmt.Sprintf("$%d.%02d", dollars, micros/10000)
	} else {
		// Trim trailing zeros after 2 decimal digits up to 6 digits
		s := fmt.Sprintf("%06d", micros)
		s = strings.TrimRight(s, "0")
		if len(s) < 2 {
			s = s + strings.Repeat("0", 2-len(s))
		}
		formatted = fmt.Sprintf("$%d.%s", dollars, s)
	}

	if isNegative {
		return "-" + formatted
	}
	return formatted
}
