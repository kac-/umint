package umint_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kac-/umint"
	"testing"
	"time"
)

var (
	stpl0 string = `{"BlockFromTime":1404111258,
	"StakeModifier":11119442999521180503,
	"PrevTxOffset":81,
	"PrevTxTime":1404111247,
	"PrevTxOutIndex":0,
	"PrevTxOutValue":300000000,
	"IsProtocolV03":true,
	"StakeMinAge":2592000,
	"Bits":471063663,
	"TxTime":1411662109}`

	tpl0Hash []byte = []byte{
		0x00, 0x00, 0x02, 0xf8, 0x26, 0xdf, 0x56, 0xdf,
		0x76, 0x6a, 0x16, 0xab, 0x4f, 0xa8, 0x3e, 0x5f,
		0x06, 0xf2, 0x8b, 0xed, 0x73, 0x6f, 0xa5, 0x69,
		0x25, 0xb8, 0xc5, 0xdd, 0x71, 0x12, 0x12, 0x39,
	}
)

func TestCheckFunction(t *testing.T) {

	tpl := umint.StakeKernelTemplate{}
	err := json.Unmarshal([]byte(stpl0), &tpl)
	if err != nil {
		t.Errorf("unmarshalling: %v", err)
		return
	}
	hash, success, err := umint.CheckStakeKernelHash(&tpl)
	if err != nil {
		t.Errorf("checking good template: %v", err)
		return
	}
	if !success {
		t.Errorf("wrong check result, have %v want %v", success, true)
		return
	}
	if !bytes.Equal(tpl0Hash, hash) {
		t.Errorf("wrong kernel hash, have %v want %v", hash, tpl0Hash)
		return
	}

	//  modify and test failure
	tpl.StakeModifier++
	hash, success, err = umint.CheckStakeKernelHash(&tpl)
	if err != nil {
		t.Errorf("checking good template: %v", err)
		return
	}
	if success {
		t.Errorf("wrong check result, have %v want %v", success, false)
		return
	}
}

func TestCheckFunctionPerformance(t *testing.T) {
	tpl := umint.StakeKernelTemplate{}
	err := json.Unmarshal([]byte(stpl0), &tpl)
	if err != nil {
		t.Errorf("unmarshalling: %v", err)
		return
	}
	start := time.Now()
	for i := 0; i < 100000; i++ {
		umint.CheckStakeKernelHash(&tpl)
	}
	fmt.Printf("100k checks took %v\n", time.Now().Sub(start))
}
