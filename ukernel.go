package umint

import (
	"errors"
	"github.com/conformal/fastsha256"
	"math/big"
)

const (
	day         int   = 60 * 60 * 24
	stakeMaxAge int64 = 90 * int64(day)
	coin        int64 = 1000000
	coinDay     int64 = coin * int64(day)
)

func doubleSha256(b []byte) []byte {
	hasher := fastsha256.New()
	hasher.Write(b)
	sum := hasher.Sum(nil)
	hasher.Reset()
	hasher.Write(sum)
	return hasher.Sum(nil)
}

func compactToBig(compact uint32) *big.Int {
	// Extract the mantissa, sign bit, and exponent.
	mantissa := compact & 0x007fffff
	isNegative := compact&0x00800000 != 0
	exponent := uint(compact >> 24)

	// Since the base for the exponent is 256, the exponent can be treated
	// as the number of bytes to represent the full 256-bit number.  So,
	// treat the exponent as the number of bytes and shift the mantissa
	// right or left accordingly.  This is equivalent to:
	// N = mantissa * 256^(exponent-3)
	var bn *big.Int
	if exponent <= 3 {
		mantissa >>= 8 * (3 - exponent)
		bn = big.NewInt(int64(mantissa))
	} else {
		bn = big.NewInt(int64(mantissa))
		bn.Lsh(bn, 8*(exponent-3))
	}

	// Make it negative if the sign bit is set.
	if isNegative {
		bn = bn.Neg(bn)
	}

	return bn
}

type StakeKernelTemplate struct {
	BlockFromTime int64
	StakeModifier uint64
	PrevTxOffset  uint32
	PrevTxTime    int64

	PrevTxOutIndex uint32
	PrevTxOutValue int64

	IsProtocolV03 bool
	StakeMinAge   int64
	Bits          uint32
	TxTime        int64
}

func CheckStakeKernelHash(t *StakeKernelTemplate) (hashProofOfStake []byte, success bool, err error) {
	success = false

	if t.TxTime < t.PrevTxTime { // Transaction timestamp violation
		err = errors.New("CheckStakeKernelHash() : nTime violation")
		return
	}

	if t.BlockFromTime+t.StakeMinAge > t.TxTime { // Min age requirement
		err = errors.New("CheckStakeKernelHash() : min age violation")
		return
	}

	bnTargetPerCoinDay := compactToBig(t.Bits)
	var timeReduction int64
	if t.IsProtocolV03 {
		timeReduction = t.StakeMinAge
	} else {
		timeReduction = 0
	}

	var nTimeWeight int64 = t.TxTime - t.PrevTxTime
	if nTimeWeight > stakeMaxAge {
		nTimeWeight = stakeMaxAge
	}
	nTimeWeight -= timeReduction

	var bnCoinDayWeight *big.Int
	valueTime := t.PrevTxOutValue * nTimeWeight
	if valueTime > 0 { // no overflow
		bnCoinDayWeight = new(big.Int).SetInt64(valueTime / coinDay)
	} else {
		// overflow, calc w/ big.Int or return error?
		err = errors.New("valueTime overflow")
		return
		bnCoinDayWeight = new(big.Int).Div(new(big.Int).
			Div(
			new(big.Int).Mul(big.NewInt(t.PrevTxOutValue), big.NewInt(nTimeWeight)),
			new(big.Int).SetInt64(coin)),
			big.NewInt(24*60*60))
	}
	targetInt := new(big.Int).Mul(bnCoinDayWeight, bnTargetPerCoinDay)

	buf := [28]byte{}
	o := 0

	if t.IsProtocolV03 { // v0.3 protocol
		d := t.StakeModifier
		for i := 0; i < 8; i++ {
			buf[o] = byte(d & 0xff)
			d >>= 8
			o++
		}
	} else { // v0.2 protocol
		d := t.Bits
		for i := 0; i < 4; i++ {
			buf[o] = byte(d & 0xff)
			d >>= 8
			o++
		}
	}
	data := [5]uint32{uint32(t.BlockFromTime), uint32(t.PrevTxOffset),
		uint32(t.PrevTxTime), uint32(t.PrevTxOutIndex), uint32(t.TxTime)}
	for _, d := range data {
		for i := 0; i < 4; i++ {
			buf[o] = byte(d & 0xff)
			d >>= 8
			o++
		}
	}
	hashProofOfStake = doubleSha256(buf[:o])
	hashProofOfStakeInt := new(big.Int).SetBytes(hashProofOfStake)

	if hashProofOfStakeInt.Cmp(targetInt) > 0 {
		return
	}

	success = true
	return
}
