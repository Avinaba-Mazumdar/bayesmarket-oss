package amm

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/shopspring/decimal"
)

// Outcome represents the binary outcome token type.
type Outcome string

const (
	OutcomeYES Outcome = "YES"
	OutcomeNO  Outcome = "NO"
)

// Standard precision constants.
const (
	StoragePrecision     int32 = 8
	InternalPrecision    int32 = 18
	CalculationPrecision int32 = 28
)

var (
	OneHundred = decimal.NewFromInt(100)
	Two        = decimal.NewFromInt(2)
	Four       = decimal.NewFromInt(4)
	One        = decimal.NewFromInt(1)
	Zero       = decimal.Zero
)

// Common AMM domain errors.
var (
	ErrZeroOrNegativeReserves    = errors.New("pool reserves must be strictly positive")
	ErrNegativeCollateral        = errors.New("collateral reserve cannot be negative")
	ErrZeroOrNegativeDeposit     = errors.New("deposit USDC amount must be strictly positive")
	ErrZeroOrNegativeShares      = errors.New("shares amount to sell must be strictly positive")
	ErrInvalidOutcome            = errors.New("invalid outcome: must be YES or NO")
	ErrInsufficientPoolLiquidity = errors.New("insufficient pool virtual liquidity for trade")
	ErrInsufficientCollateral    = errors.New("insufficient pool collateral to fund share liquidation")
	ErrNegativeDiscriminant      = errors.New("negative discriminant encountered in quadratic sell solver")
	ErrSpotPriceSumViolation     = errors.New("spot prices sum deviates from 1.000000 beyond allowed tolerance")
)

// PoolReserves represents the on-chain/in-database liquidity pool state.
type PoolReserves struct {
	ReserveYes        decimal.Decimal `json:"reserve_yes"`
	ReserveNo         decimal.Decimal `json:"reserve_no"`
	CollateralReserve decimal.Decimal `json:"collateral_reserve"`
}

// BuyQuote encapsulates the result of a complete-set share purchase calculation.
type BuyQuote struct {
	Outcome        Outcome         `json:"outcome"`
	DepositUSDC    decimal.Decimal `json:"deposit_usdc"`
	SharesReceived decimal.Decimal `json:"shares_received"`
	AvgPrice       decimal.Decimal `json:"avg_price"`
	InitialPrice   decimal.Decimal `json:"initial_price"`
	NewPrice       decimal.Decimal `json:"new_price"`
	PriceImpactPct decimal.Decimal `json:"price_impact_pct"`
	NewReserveYes  decimal.Decimal `json:"new_reserve_yes"`
	NewReserveNo   decimal.Decimal `json:"new_reserve_no"`
	NewCollateral  decimal.Decimal `json:"new_collateral"`
}

// SellQuote encapsulates the result of a complete-set share sale ("cash out") calculation.
type SellQuote struct {
	Outcome        Outcome         `json:"outcome"`
	SharesToSell   decimal.Decimal `json:"shares_to_sell"`
	PayoutUSDC     decimal.Decimal `json:"payout_usdc"`
	AvgPrice       decimal.Decimal `json:"avg_price"`
	InitialPrice   decimal.Decimal `json:"initial_price"`
	NewPrice       decimal.Decimal `json:"new_price"`
	PriceImpactPct decimal.Decimal `json:"price_impact_pct"`
	NewReserveYes  decimal.Decimal `json:"new_reserve_yes"`
	NewReserveNo   decimal.Decimal `json:"new_reserve_no"`
	NewCollateral  decimal.Decimal `json:"new_collateral"`
}

// ValidatePoolReserves checks that pool reserves conform to mathematical invariants.
func ValidatePoolReserves(pool PoolReserves) error {
	if pool.ReserveYes.LessThanOrEqual(Zero) || pool.ReserveNo.LessThanOrEqual(Zero) {
		return ErrZeroOrNegativeReserves
	}
	if pool.CollateralReserve.IsNegative() {
		return ErrNegativeCollateral
	}
	return nil
}

// CalculateSpotPrices computes current implied probabilities for YES and NO outcomes.
//
// In a CPMM prediction market with complete-set backing:
//
//	P_YES = R_NO / (R_YES + R_NO)
//	P_NO  = R_YES / (R_YES + R_NO)
//
// Enforces P_YES + P_NO = 1.000000 to at least 6 decimal places.
func CalculateSpotPrices(pool PoolReserves) (pYes decimal.Decimal, pNo decimal.Decimal, err error) {
	if err := ValidatePoolReserves(pool); err != nil {
		return Zero, Zero, err
	}

	sumReserves := pool.ReserveYes.Add(pool.ReserveNo)
	if sumReserves.IsZero() {
		return Zero, Zero, ErrZeroOrNegativeReserves
	}

	pYes = pool.ReserveNo.DivRound(sumReserves, CalculationPrecision)
	pNo = pool.ReserveYes.DivRound(sumReserves, CalculationPrecision)

	// Validate spot price sum invariant: P_YES + P_NO == 1.000000 to 6 decimals
	sum := pYes.Add(pNo)
	diff := sum.Sub(One).Abs()
	tolerance := decimal.New(1, -6) // 10^-6 = 0.000001
	if diff.GreaterThan(tolerance) {
		return Zero, Zero, fmt.Errorf("%w: P_YES + P_NO = %s", ErrSpotPriceSumViolation, sum.StringFixed(6))
	}

	return pYes.Truncate(StoragePrecision), pNo.Truncate(StoragePrecision), nil
}

// CalculateCompleteSetBuy calculates share output, price, and updated reserves when depositing USDC.
//
// Complete-Set Buy Mechanics:
//  1. User deposits d USDC into the market.
//  2. Exactly d complete sets (d YES + d NO) are minted, increasing collateral reserve by d.
//  3. If buying YES:
//     - The d NO shares are swapped into the pool's virtual reserve: R_NO' = R_NO + d
//     - The constant product invariant k = R_YES * R_NO demands R_YES' = k / R_NO'
//     - The user receives minted d YES + swapped out YES: Delta YES = R_YES + d - R_YES'
//  4. Symmetrically for NO:
//     - R_YES' = R_YES + d
//     - R_NO' = k / R_YES'
//     - Delta NO = R_NO + d - R_NO'
//
// All calculations use pure fixed-point arithmetic with zero floats.
func CalculateCompleteSetBuy(depositUSDC decimal.Decimal, outcome Outcome, pool PoolReserves) (BuyQuote, error) {
	if err := ValidatePoolReserves(pool); err != nil {
		return BuyQuote{}, err
	}
	if depositUSDC.LessThanOrEqual(Zero) {
		return BuyQuote{}, ErrZeroOrNegativeDeposit
	}
	if outcome != OutcomeYES && outcome != OutcomeNO {
		return BuyQuote{}, ErrInvalidOutcome
	}

	pYesInit, pNoInit, err := CalculateSpotPrices(pool)
	if err != nil {
		return BuyQuote{}, err
	}

	k := pool.ReserveYes.Mul(pool.ReserveNo)
	var sharesReceived, newReserveYes, newReserveNo, initialPrice decimal.Decimal

	if outcome == OutcomeYES {
		initialPrice = pYesInit
		newReserveNo = pool.ReserveNo.Add(depositUSDC)
		newReserveYes = k.DivRound(newReserveNo, CalculationPrecision)
		// Delta YES = R_YES + d - R_YES'
		sharesReceived = pool.ReserveYes.Add(depositUSDC).Sub(newReserveYes)
	} else {
		initialPrice = pNoInit
		newReserveYes = pool.ReserveYes.Add(depositUSDC)
		newReserveNo = k.DivRound(newReserveYes, CalculationPrecision)
		// Delta NO = R_NO + d - R_NO'
		sharesReceived = pool.ReserveNo.Add(depositUSDC).Sub(newReserveNo)
	}

	if sharesReceived.LessThanOrEqual(Zero) || sharesReceived.Truncate(StoragePrecision).IsZero() {
		return BuyQuote{}, ErrInsufficientPoolLiquidity
	}

	// Average execution price = depositUSDC / sharesReceived
	avgPrice := depositUSDC.DivRound(sharesReceived, CalculationPrecision)

	// Post-trade spot price
	newPool := PoolReserves{
		ReserveYes:        newReserveYes,
		ReserveNo:         newReserveNo,
		CollateralReserve: pool.CollateralReserve.Add(depositUSDC),
	}
	pYesNew, pNoNew, err := CalculateSpotPrices(newPool)
	if err != nil {
		return BuyQuote{}, err
	}

	var newPrice decimal.Decimal
	if outcome == OutcomeYES {
		newPrice = pYesNew
	} else {
		newPrice = pNoNew
	}

	// Marginal price impact / slippage: ((AvgPrice - InitialPrice) / InitialPrice) * 100%
	var priceImpactPct decimal.Decimal
	if initialPrice.IsPositive() {
		priceImpactPct = avgPrice.Sub(initialPrice).DivRound(initialPrice, CalculationPrecision).Mul(OneHundred)
	}

	return BuyQuote{
		Outcome:        outcome,
		DepositUSDC:    depositUSDC.Truncate(StoragePrecision),
		SharesReceived: sharesReceived.Truncate(StoragePrecision),
		AvgPrice:       avgPrice.Truncate(StoragePrecision),
		InitialPrice:   initialPrice,
		NewPrice:       newPrice,
		PriceImpactPct: priceImpactPct.Truncate(StoragePrecision),
		NewReserveYes:  newReserveYes.Truncate(StoragePrecision),
		NewReserveNo:   newReserveNo.Truncate(StoragePrecision),
		NewCollateral:  newPool.CollateralReserve.Truncate(StoragePrecision),
	}, nil
}

// CalculateCompleteSetSell calculates USDC payout, price impact, and new reserves when liquidating shares.
//
// Complete-Set Sell ("Cash Out") Mechanics:
//  1. User sells s shares of an outcome back to the pool to receive d USDC.
//  2. To extract d USDC from collateral, d complete sets (d YES + d NO) must be burned.
//  3. For selling YES shares:
//     - The pool supplies d NO shares: R_NO' = R_NO - d
//     - The pool absorbs (s - d) YES shares: R_YES' = R_YES + s - d
//     - Preserving k = R_YES * R_NO:
//     (R_YES + s - d)(R_NO - d) = k = R_YES * R_NO
//     d^2 - (R_YES + R_NO + s)d + s*R_NO = 0
//     - Let A = R_YES + R_NO + s.
//     - Quadratic solution: d = (A - sqrt(A^2 - 4*s*R_NO)) / 2
//  4. For selling NO shares:
//     - Symmetrically: d = (A - sqrt(A^2 - 4*s*R_YES)) / 2
//     - R_YES' = R_YES - d
//     - R_NO' = R_NO + s - d
//  5. Collateral decreases by d: C' = C - d.
func CalculateCompleteSetSell(sharesToSell decimal.Decimal, outcome Outcome, pool PoolReserves) (SellQuote, error) {
	if err := ValidatePoolReserves(pool); err != nil {
		return SellQuote{}, err
	}
	if sharesToSell.LessThanOrEqual(Zero) {
		return SellQuote{}, ErrZeroOrNegativeShares
	}
	if outcome != OutcomeYES && outcome != OutcomeNO {
		return SellQuote{}, ErrInvalidOutcome
	}

	pYesInit, pNoInit, err := CalculateSpotPrices(pool)
	if err != nil {
		return SellQuote{}, err
	}

	// A = R_YES + R_NO + s
	A := pool.ReserveYes.Add(pool.ReserveNo).Add(sharesToSell)
	ASquared := A.Mul(A)

	var fourSR, initialPrice decimal.Decimal
	if outcome == OutcomeYES {
		initialPrice = pYesInit
		fourSR = Four.Mul(sharesToSell).Mul(pool.ReserveNo)
	} else {
		initialPrice = pNoInit
		fourSR = Four.Mul(sharesToSell).Mul(pool.ReserveYes)
	}

	discriminant := ASquared.Sub(fourSR)
	if discriminant.IsNegative() {
		return SellQuote{}, ErrNegativeDiscriminant
	}

	sqrtDiscriminant, err := DecimalSqrt(discriminant)
	if err != nil {
		return SellQuote{}, fmt.Errorf("failed to solve quadratic square root: %w", err)
	}

	// d = (A - sqrt(A^2 - 4sR)) / 2
	payoutUSDC := A.Sub(sqrtDiscriminant).DivRound(Two, CalculationPrecision)

	// Boundary checks
	if payoutUSDC.LessThanOrEqual(Zero) || payoutUSDC.Truncate(StoragePrecision).IsZero() {
		return SellQuote{}, ErrInsufficientPoolLiquidity
	}
	if payoutUSDC.GreaterThan(pool.CollateralReserve) {
		return SellQuote{}, ErrInsufficientCollateral
	}

	var newReserveYes, newReserveNo decimal.Decimal
	if outcome == OutcomeYES {
		if payoutUSDC.GreaterThanOrEqual(pool.ReserveNo) {
			return SellQuote{}, ErrInsufficientPoolLiquidity
		}
		newReserveNo = pool.ReserveNo.Sub(payoutUSDC)
		newReserveYes = pool.ReserveYes.Add(sharesToSell).Sub(payoutUSDC)
	} else {
		if payoutUSDC.GreaterThanOrEqual(pool.ReserveYes) {
			return SellQuote{}, ErrInsufficientPoolLiquidity
		}
		newReserveYes = pool.ReserveYes.Sub(payoutUSDC)
		newReserveNo = pool.ReserveNo.Add(sharesToSell).Sub(payoutUSDC)
	}

	newCollateral := pool.CollateralReserve.Sub(payoutUSDC)

	if newReserveYes.LessThanOrEqual(Zero) || newReserveNo.LessThanOrEqual(Zero) || newCollateral.IsNegative() {
		return SellQuote{}, ErrInsufficientPoolLiquidity
	}

	// Average sale price = payoutUSDC / sharesToSell
	avgPrice := payoutUSDC.DivRound(sharesToSell, CalculationPrecision)

	// Post-sale spot price
	newPool := PoolReserves{
		ReserveYes:        newReserveYes,
		ReserveNo:         newReserveNo,
		CollateralReserve: newCollateral,
	}
	pYesNew, pNoNew, err := CalculateSpotPrices(newPool)
	if err != nil {
		return SellQuote{}, err
	}

	var newPrice decimal.Decimal
	if outcome == OutcomeYES {
		newPrice = pYesNew
	} else {
		newPrice = pNoNew
	}

	// Slippage / Price impact: ((InitialPrice - AvgPrice) / InitialPrice) * 100%
	var priceImpactPct decimal.Decimal
	if initialPrice.IsPositive() {
		priceImpactPct = initialPrice.Sub(avgPrice).DivRound(initialPrice, CalculationPrecision).Mul(OneHundred)
	}

	return SellQuote{
		Outcome:        outcome,
		SharesToSell:   sharesToSell.Truncate(StoragePrecision),
		PayoutUSDC:     payoutUSDC.Truncate(StoragePrecision),
		AvgPrice:       avgPrice.Truncate(StoragePrecision),
		InitialPrice:   initialPrice,
		NewPrice:       newPrice,
		PriceImpactPct: priceImpactPct.Truncate(StoragePrecision),
		NewReserveYes:  newReserveYes.Truncate(StoragePrecision),
		NewReserveNo:   newReserveNo.Truncate(StoragePrecision),
		NewCollateral:  newCollateral.Truncate(StoragePrecision),
	}, nil
}

// DecimalSqrt calculates the square root of a positive decimal using arbitrary-precision
// integer arithmetic (math/big.Int.Sqrt) combined with high-precision Newton-Raphson refinement.
//
// ZERO FLOATING POINT CONVERSIONS. Pure fixed-point deterministic arithmetic.
func DecimalSqrt(d decimal.Decimal) (decimal.Decimal, error) {
	if d.IsNegative() {
		return Zero, errors.New("cannot compute square root of negative decimal")
	}
	if d.IsZero() {
		return Zero, nil
	}

	// Scaling strategy:
	// Multiply d by 10^36, extract integer coefficient, compute exact integer square root,
	// and shift back by 10^-18 to obtain an exact initial guess with 18 digits of precision.
	const scaleExponent = 36
	scaleFactor := decimal.New(1, scaleExponent)
	scaled := d.Mul(scaleFactor)

	// Extract big.Int floor of scaled decimal
	bigIntPart := scaled.BigInt()
	bigSqrt := new(big.Int).Sqrt(bigIntPart)

	// Create initial guess x0 = bigSqrt / 10^18
	x := decimal.NewFromBigInt(bigSqrt, -18)

	if x.IsZero() {
		x = decimal.New(1, -9)
	}

	// Execute 3 Newton-Raphson iterations with 28 digits division precision:
	// x_{n+1} = (x_n + d / x_n) / 2
	for i := 0; i < 3; i++ {
		quotient := d.DivRound(x, CalculationPrecision)
		x = x.Add(quotient).DivRound(Two, CalculationPrecision)
	}

	return x, nil
}
