package umint

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/conformal/fastsha256"
	"math/big"
)

const (
	stakeMaxAge int64 = 60 * 60 * 24 * 90
	coin        int64 = 1000000
)

func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

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
	Bits           uint32
	BlockFromTime  int64
	TxTime         int64
	StakeModifier  uint64
	PrevTxOffset   uint32
	PrevTxTime     int64
	PrevTxOutIndex uint32
	PrevTxOutValue int64
	IsProtocolV03  bool
	StakeMinAge    int64
}

func CheckStakeKernelHash(t StakeKernelTemplate) (hashProofOfStake []byte, success bool, err error) {
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
	var nTimeWeight int64 = minInt64(t.TxTime-t.PrevTxTime, stakeMaxAge) - timeReduction

	var bnCoinDayWeight *big.Int = new(big.Int).Div(new(big.Int).
		Div(
		new(big.Int).Mul(big.NewInt(t.PrevTxOutValue), big.NewInt(nTimeWeight)),
		new(big.Int).SetInt64(coin)),
		big.NewInt(24*60*60))

	buf := bytes.NewBuffer(make([]byte, 0, 28)) // TODO pre-calculate size?

	if t.IsProtocolV03 { // v0.3 protocol
		err = binary.Write(buf, binary.LittleEndian, t.StakeModifier)
		if err != nil {
			return
		}
	} else { // v0.2 protocol
		//ss << Bits;
		err = binary.Write(buf, binary.LittleEndian, t.Bits)
		if err != nil {
			return
		}
	}
	data := [5]uint32{uint32(t.BlockFromTime), uint32(t.PrevTxOffset),
		uint32(t.PrevTxTime), uint32(t.PrevTxOutIndex), uint32(t.TxTime)}
	for _, d := range data {
		err = binary.Write(buf, binary.LittleEndian, d)
		if err != nil {
			return
		}
	}

	hashProofOfStake = doubleSha256(buf.Bytes())
	if err != nil {
		return
	}

	hashProofOfStakeInt := new(big.Int).SetBytes(hashProofOfStake)
	targetInt := new(big.Int).Mul(bnCoinDayWeight, bnTargetPerCoinDay)
	if hashProofOfStakeInt.Cmp(targetInt) > 0 {
		return
	}

	success = true
	return
}
