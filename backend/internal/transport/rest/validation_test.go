package rest_test

import (
	"testing"

	"github.com/bayesmarket/bayesmarket/internal/amm"
	"github.com/bayesmarket/bayesmarket/internal/transport/rest"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestParseOutcome(t *testing.T) {
	tests := []struct {
		input       string
		expected    amm.Outcome
		expectError bool
	}{
		{"yes", amm.OutcomeYES, false},
		{"YES", amm.OutcomeYES, false},
		{"  No  ", amm.OutcomeNO, false},
		{"MAYBE", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		got, err := rest.ParseOutcome(tt.input)
		if tt.expectError {
			assert.NotNil(t, err, "expected error for %q", tt.input)
		} else {
			assert.Nil(t, err, "unexpected error for %q", tt.input)
			assert.Equal(t, tt.expected, got)
		}
	}
}

func TestParsePositiveDecimal(t *testing.T) {
	// Valid
	d, err := rest.ParsePositiveDecimal("100.50", "amount")
	assert.Nil(t, err)
	assert.True(t, d.Equal(decimal.NewFromFloat(100.50)))

	// Exceeds max 1,000,000
	_, err = rest.ParsePositiveDecimal("1000001", "amount")
	assert.NotNil(t, err)
	assert.Equal(t, "invalid_amount", err.ErrorCode)

	// Zero or Negative
	_, err = rest.ParsePositiveDecimal("0", "amount")
	assert.NotNil(t, err)
	_, err = rest.ParsePositiveDecimal("-5", "amount")
	assert.NotNil(t, err)

	// Non-numeric or empty
	_, err = rest.ParsePositiveDecimal("", "amount")
	assert.NotNil(t, err)
	_, err = rest.ParsePositiveDecimal("abc", "amount")
	assert.NotNil(t, err)
}

func TestParseOptionalBoundedPositiveDecimal(t *testing.T) {
	// Empty string should return default without error
	d, err := rest.ParseOptionalBoundedPositiveDecimal("", "min_payout_usdc", decimal.Zero, rest.MaxTradeAmountUSDC)
	assert.Nil(t, err)
	assert.True(t, d.IsZero())

	// Whitespace should return default
	d, err = rest.ParseOptionalBoundedPositiveDecimal("   ", "min_payout_usdc", decimal.Zero, rest.MaxTradeAmountUSDC)
	assert.Nil(t, err)
	assert.True(t, d.IsZero())

	// Valid amount > 100 USDC (previously broken when using ParseSlippagePct)
	d, err = rest.ParseOptionalBoundedPositiveDecimal("250.75", "min_payout_usdc", decimal.Zero, rest.MaxTradeAmountUSDC)
	assert.Nil(t, err)
	assert.True(t, d.Equal(decimal.RequireFromString("250.75")))

	// Exceeds upper limit of 1M USDC
	_, err = rest.ParseOptionalBoundedPositiveDecimal("1000001", "min_payout_usdc", decimal.Zero, rest.MaxTradeAmountUSDC)
	assert.NotNil(t, err)
	assert.Equal(t, "invalid_min_payout_usdc", err.ErrorCode)

	// Negative value is rejected
	_, err = rest.ParseOptionalBoundedPositiveDecimal("-1", "min_payout_usdc", decimal.Zero, rest.MaxTradeAmountUSDC)
	assert.NotNil(t, err)
	assert.Equal(t, "invalid_min_payout_usdc", err.ErrorCode)
}
