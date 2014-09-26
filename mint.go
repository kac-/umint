package umint

import (
	"math/big"
)

type TxOut struct {
	Value    int64
	PkScript []byte
}

type CoinStakeSource struct {
	BlockTime     int64
	StakeModifier uint64
	TxOffset      uint32
	TxTime        int64

	TxSha []byte

	Outputs map[string]*TxOut
}

// BigToCompact converts a whole number N to a compact representation using
// an unsigned 32-bit number.  The compact representation only provides 23 bits
// of precision, so values larger than (2^23 - 1) only encode the most
// significant digits of the number.  See CompactToBig for details.
func BigToCompact(n *big.Int) uint32 {
	// No need to do any work if it's zero.
	if n.Sign() == 0 {
		return 0
	}

	// Since the base for the exponent is 256, the exponent can be treated
	// as the number of bytes.  So, shift the number right or left
	// accordingly.  This is equivalent to:
	// mantissa = mantissa / 256^(exponent-3)
	var mantissa uint32
	exponent := uint(len(n.Bytes()))
	if exponent <= 3 {
		mantissa = uint32(n.Bits()[0])
		mantissa <<= 8 * (3 - exponent)
	} else {
		// Use a copy to avoid modifying the caller's original number.
		tn := new(big.Int).Set(n)
		mantissa = uint32(tn.Rsh(tn, 8*(exponent-3)).Bits()[0])
	}

	// When the mantissa already has the sign bit set, the number is too
	// large to fit into the available 23-bits, so divide the number by 256
	// and increment the exponent accordingly.
	if mantissa&0x00800000 != 0 {
		mantissa >>= 8
		exponent++
	}

	// Pack the exponent, sign bit, and mantissa into an unsigned 32-bit
	// int and return it.
	compact := uint32(exponent<<24) | mantissa
	if n.Sign() < 0 {
		compact |= 0x00800000
	}
	return compact
}

func CompactToDiff(bits uint32) (diff float32) {
	nShift := (bits >> 24) & 0xff
	diff = float32(0x0000ffff) / float32(bits&0x00ffffff)
	for ; nShift < 29; nShift++ {
		diff *= 256.0
	}
	for ; nShift > 29; nShift-- {
		diff /= 256.0
	}
	return
}

func IncCompact(compact uint32) uint32 {
	mantissa := compact & 0x007fffff
	neg := compact & 0x00800000
	exponent := uint(compact >> 24)

	if exponent <= 3 {
		mantissa += uint32(1 << (8 * (3 - exponent)))
	} else {
		mantissa++
	}

	if mantissa >= 0x00800000 {
		mantissa >>= 8
		exponent++
	}
	return uint32(exponent<<24) | mantissa | neg
}

func DiffToTarget(diff float32) (target *big.Int) {
	mantissa := 0x0000ffff / diff
	exp := 1
	tmp := mantissa
	for tmp >= 256.0 {
		tmp /= 256.0
		exp++
	}
	for i := 0; i < exp; i++ {
		mantissa *= 256.0
	}
	target = new(big.Int).Lsh(big.NewInt(int64(mantissa)), uint(26-exp)*8)
	return
}
