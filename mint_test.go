package umint_test

import (
	"fmt"
	"github.com/kac-/umint"
	"math/big"
	"testing"
)

func printNum(num int64) {
	b := big.NewInt(num)
	compact := umint.BigToCompact(b)
	mantissa := compact & 0x007fffff
	isNegative := compact&0x00800000 != 0
	exponent := uint(compact >> 24)
	fmt.Printf("c: %08x m: %08x e: %02x n: %v b: %x md: %d\n", compact, mantissa, exponent, isNegative, b, mantissa)
}

func printCompact(compact uint32) string {
	mantissa := compact & 0x007fffff
	isNegative := compact&0x00800000 != 0
	exponent := uint(compact >> 24)
	return fmt.Sprintf("c: %08x m: %08x e: %02x n: %v md: %d", compact, mantissa, exponent, isNegative, mantissa)
}

func incTest(num int64) error {
	b0 := big.NewInt(num)
	c0 := umint.BigToCompact(b0)
	var b1 *big.Int
	if num < 0x800000 {
		b1 = big.NewInt(num + 1)
	} else {
		b1 = big.NewInt(num)
		bytes := b1.BitLen()/8 + 1
		b1.Add(b1, new(big.Int).Lsh(big.NewInt(1), uint(8*(bytes-3))))
	}
	c1 := umint.BigToCompact(b1)
	if c1 == c0 {
		return fmt.Errorf("incTest: c1 == c0 for %v(0x%x), bad test code", num, num)
	}
	c1_ := umint.IncCompact(c0)
	if c1_ != c1 {
		return fmt.Errorf(`incTest: c1_ != c1 for %v(0x%x)
c0   %s
have %s
want %s`, num, num, printCompact(c0), printCompact(c1_), printCompact(c1))
	}
	return nil
}

func testCompact(compact uint32) {
	target := umint.CompactToBig(compact)
	diff := umint.CompactToDiff(compact)
	fmantissa := 0x0000ffff / diff
	tmp := fmantissa
	exp := 1
	for tmp >= 256.0 {
		tmp /= 256.0
		exp++
	}
	tmp2 := fmantissa
	for i := 0; i < exp; i++ {
		tmp2 *= 256.0
	}
	tmp3 := big.NewInt(int64(tmp2))
	tmp3.Lsh(tmp3, uint(26-exp)*8)

	fmt.Printf(`diff %v
fman %v
exp  %v
tmp  %v
tmp2 %v
tmp3 %v
tgt  %v
`, diff, fmantissa, exp, tmp, tmp2, tmp3, target)
}

func TestPrint(t *testing.T) {
	c0 := uint32(0x1c147e17)
	c1 := umint.BigToCompact(umint.DiffToTarget(umint.CompactToDiff(c0)))
	if c0 != c1 {
		t.Errorf("c0 != c1 : %08x != %08x", c0, c1)
		return
	}
}

func TestIncrement(t *testing.T) {
	tests := []int64{
		0x1, 0x2, 0x12, 0x123, 0x1234, 0x12345, 0x123456,
		0x800000,
		umint.CompactToBig(umint.BigToCompact(big.NewInt(0x1234567))).Int64(),
		umint.CompactToBig(umint.BigToCompact(big.NewInt(0x1234567))).Int64() - 1,
		0x1234567,
		0x7fffff, 0x7ffffffff,
	}
	for _, num := range tests {
		//printNum(num)
		err := incTest(num)
		if err != nil {
			t.Error(err)
			return
		}
	}
}

func OffTestPrintDiffToTarget(t *testing.T) {
	//testCompact(uint32(0x1c147e17))
	bi := umint.CompactToBig(0x1c147e17)
	for i, z := 0, int64(2); i < 4; i, z = i+1, z*z {
		testCompact(umint.BigToCompact(new(big.Int).Mul(bi, big.NewInt(z))))
	}
	for i, z := 0, int64(2); i < 4; i, z = i+1, z*z {
		testCompact(umint.BigToCompact(new(big.Int).Div(bi, big.NewInt(z))))
	}

}
