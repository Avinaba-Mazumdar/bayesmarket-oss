package rest

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/bayesmarket/bayesmarket/internal/amm"
	"github.com/shopspring/decimal"
)

// ParseOutcome validates and normalizes an outcome string to amm.Outcome ("YES" | "NO").
func ParseOutcome(raw string) (amm.Outcome, *AppError) {
	trimmed := strings.ToUpper(strings.TrimSpace(raw))
	if trimmed != string(amm.OutcomeYES) && trimmed != string(amm.OutcomeNO) {
		return "", &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "invalid_outcome",
			Message:    "Outcome must be 'YES' or 'NO'",
		}
	}
	return amm.Outcome(trimmed), nil
}

// MaxTradeAmountUSDC defines the maximum order or collateral amount allowed per transaction (1,000,000 USDC).
var MaxTradeAmountUSDC = decimal.NewFromInt(1_000_000)

// ParsePositiveDecimal parses a string into a strictly positive decimal.Decimal bounded by MaxTradeAmountUSDC.
func ParsePositiveDecimal(raw string, fieldName string) (decimal.Decimal, *AppError) {
	return ParseBoundedPositiveDecimal(raw, fieldName, MaxTradeAmountUSDC)
}

// ParseBoundedPositiveDecimal parses a string into a strictly positive decimal.Decimal and enforces an upper bound and storage precision (8 decimal places).
func ParseBoundedPositiveDecimal(raw string, fieldName string, maxVal decimal.Decimal) (decimal.Decimal, *AppError) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return decimal.Zero, &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "invalid_" + fieldName,
			Message:    fmt.Sprintf("%s is required", fieldName),
		}
	}

	// Reject precision beyond database NUMERIC(..., 8) storage precision
	parts := strings.Split(trimmed, ".")
	if len(parts) == 2 {
		fractional := strings.TrimRight(parts[1], "0")
		if len(fractional) > int(amm.StoragePrecision) {
			return decimal.Zero, &AppError{
				StatusCode: http.StatusBadRequest,
				ErrorCode:  "invalid_" + fieldName,
				Message:    fmt.Sprintf("%s exceeds maximum supported precision of %d decimal places", fieldName, amm.StoragePrecision),
			}
		}
	}

	val, err := decimal.NewFromString(trimmed)
	if err != nil || val.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "invalid_" + fieldName,
			Message:    fmt.Sprintf("%s must be a positive decimal string", fieldName),
		}
	}

	val = val.Truncate(amm.StoragePrecision)
	if val.IsZero() {
		return decimal.Zero, &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "invalid_" + fieldName,
			Message:    fmt.Sprintf("%s is too small, must be at least 0.00000001", fieldName),
		}
	}

	if !maxVal.IsZero() && val.GreaterThan(maxVal) {
		return decimal.Zero, &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "invalid_" + fieldName,
			Message:    fmt.Sprintf("%s exceeds maximum allowed limit of %s", fieldName, maxVal.StringFixed(2)),
		}
	}

	return val, nil
}

// ParseSlippagePct parses a slippage percentage string or returns the default value.
func ParseSlippagePct(raw string, defaultPct decimal.Decimal) (decimal.Decimal, *AppError) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultPct, nil
	}

	val, err := decimal.NewFromString(trimmed)
	if err != nil || val.IsNegative() {
		return decimal.Zero, &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "invalid_slippage",
			Message:    "max_slippage_pct must be non-negative",
		}
	}

	if val.GreaterThan(decimal.NewFromInt(100)) {
		return decimal.Zero, &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "invalid_slippage",
			Message:    "max_slippage_pct cannot exceed 100%",
		}
	}

	return val, nil
}

// ParseOptionalBoundedPositiveDecimal parses an optional string into a non-negative decimal bounded by maxVal.
// If empty, it returns defaultVal without error.
func ParseOptionalBoundedPositiveDecimal(raw string, fieldName string, defaultVal, maxVal decimal.Decimal) (decimal.Decimal, *AppError) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultVal, nil
	}

	val, err := decimal.NewFromString(trimmed)
	if err != nil || val.IsNegative() {
		return decimal.Zero, &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "invalid_" + fieldName,
			Message:    fmt.Sprintf("%s must be a non-negative decimal string", fieldName),
		}
	}

	if !maxVal.IsZero() && val.GreaterThan(maxVal) {
		return decimal.Zero, &AppError{
			StatusCode: http.StatusBadRequest,
			ErrorCode:  "invalid_" + fieldName,
			Message:    fmt.Sprintf("%s exceeds maximum allowed limit of %s", fieldName, maxVal.StringFixed(2)),
		}
	}

	return val, nil
}

