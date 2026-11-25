package charset

import (
	"math"
	"math/big"
)

// pythonRound rounds the exact binary value to a decimal place, using ties to
// even, then converts the decimal result back to the nearest binary float.
// Multiplying a float by a decimal scale before rounding would lose tie bits.
func pythonRound(value float64, digits int) float64 {
	if value == 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return value
	}
	// These bounds also avoid constructing huge powers for arbitrary callers.
	// Every finite binary64 rounds to itself beyond 323 decimal places, and to
	// signed zero below -308 (Python may overflow at -308 for large values).
	if digits > 323 {
		return value
	}
	if digits < -308 {
		return math.Copysign(0, value)
	}
	negative := math.Signbit(value)
	exact := new(big.Rat).SetFloat64(math.Abs(value))
	places := digits
	if places < 0 {
		places = -places
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	if digits >= 0 {
		exact.Mul(exact, new(big.Rat).SetInt(scale))
	} else {
		exact.Quo(exact, new(big.Rat).SetInt(scale))
	}
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(exact.Num(), exact.Denom(), remainder)
	comparison := new(big.Int).Lsh(remainder, 1).Cmp(exact.Denom())
	if comparison > 0 || comparison == 0 && quotient.Bit(0) == 1 {
		quotient.Add(quotient, big.NewInt(1))
	}
	rounded := new(big.Rat).SetInt(quotient)
	if digits >= 0 {
		rounded.Quo(rounded, new(big.Rat).SetInt(scale))
	} else {
		rounded.Mul(rounded, new(big.Rat).SetInt(scale))
	}
	result, _ := rounded.Float64()
	if negative {
		result = math.Copysign(result, -1)
	}
	return result
}
