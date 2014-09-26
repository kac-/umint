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
	stpl0 string = `{"BlockFromTime":1394219584,
	"StakeModifier":15161125480764745506,
	"PrevTxOffset":160,
	"PrevTxTime":1394219584,
	"PrevTxOutIndex":1,
	"PrevTxOutValue":210090000,
	"IsProtocolV03":true,
	"StakeMinAge":2592000,
	"Bits":471087779,
	"TxTime":1411634680}`

	tpl0Hash []byte = []byte{
		0x00, 0x00, 0x00, 0xdb, 0x33, 0x30, 0x88, 0x15,
		0x19, 0xa4, 0xf3, 0x2b, 0x90, 0x91, 0xb0, 0x93,
		0x0f, 0x24, 0xec, 0x6f, 0xb0, 0x90, 0x0a, 0xcf,
		0xbf, 0xb0, 0xc2, 0x26, 0xc7, 0xbc, 0x31, 0x92,
	}
)

func TestCheckFunction(t *testing.T) {

	tpl := umint.StakeKernelTemplate{}
	err := json.Unmarshal([]byte(stpl0), &tpl)
	if err != nil {
		t.Errorf("unmarshalling: %v", err)
		return
	}
	hash, success, err, minTarget := umint.CheckStakeKernelHash(&tpl)
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

	// check if template satisfies min target
	tpl.Bits = umint.IncCompact(umint.BigToCompact(minTarget))
	_, success, _, _ = umint.CheckStakeKernelHash(&tpl)
	if !success {
		t.Errorf("wrong check result on retarget, have %v want %v", success, true)
		return
	}

	//  modify and test failure
	tpl.StakeModifier++
	hash, success, err, _ = umint.CheckStakeKernelHash(&tpl)
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
