package main

import (
	"encoding/json"
	"fmt"
	"github.com/kac-/umint"
	"github.com/mably/btcchain"
	"github.com/mably/btcdb"
	_ "github.com/mably/btcdb/ldb"
	"github.com/mably/btcnet"
	"github.com/mably/btcutil"
	"github.com/mably/btcwire"
	"math/big"
	"time"
)

func findStake(outPoint *btcwire.OutPoint, db btcdb.Db, c *btcchain.BlockChain,
	params *btcnet.Params, fromTime int64, maxTime int64, diff float32) (err error) {
	var blockFromSha *btcwire.ShaHash
	var block *btcutil.Block
	var tx *btcutil.Tx

	repl, ferr := db.FetchTxBySha(&outPoint.Hash)
	if ferr != nil {
		err = fmt.Errorf("tx by sha: %v", err)
		return
	}
	blockFromSha = repl[0].BlkSha
	tx = btcutil.NewTx(repl[0].Tx)
	block, err = db.FetchBlockBySha(blockFromSha)
	if err != nil {
		err = fmt.Errorf("fetch block by sha: %v", err)
		return
	}

	locs, err := block.TxLoc()
	if err != nil {
		fmt.Printf("tx locs: %v\n", err)
		return
	}
	var txOffset uint32 = 0
	for i, itx := range block.Transactions() {
		if itx.Sha().IsEqual(tx.Sha()) {
			txOffset = uint32(locs[i].TxStart)
			break
		}
	}
	if txOffset == 0 {
		err = fmt.Errorf("tx not found")
		return
	}

	var bits uint32
	if diff > 0 {
		bits = umint.BigToCompact(umint.DiffToTarget(diff))
	} else {
		bits, err = c.PPCCalcNextRequiredDifficulty(true)
		if err != nil {
			fmt.Printf("calc diff: %v\n", err)
			return
		}
		if diff < 0 {
			diff = -1000 / diff
			t := umint.CompactToBig(bits)
			bits = umint.BigToCompact(t.Mul(t, big.NewInt(int64(diff))).Div(t, big.NewInt(1000)))
		}
	}
	fmt.Printf("minDiff: %v\n", umint.CompactToDiff(bits))

	nStakeModifier, _, _, err := c.GetKernelStakeModifier(blockFromSha, true)
	if err != nil {
		fmt.Printf("get kernel stake modifier: %v\n", err)
		return
	}
	stpl := umint.StakeKernelTemplate{
		BlockFromTime:  block.MsgBlock().Header.Timestamp.Unix(),
		StakeModifier:  nStakeModifier,
		PrevTxOffset:   txOffset,
		PrevTxTime:     tx.MsgTx().Time.Unix(),
		PrevTxOutIndex: outPoint.Index,
		PrevTxOutValue: tx.MsgTx().TxOut[outPoint.Index].Value,
		IsProtocolV03:  isProtocolV03(params, fromTime),
		StakeMinAge:    params.StakeMinAge,
		Bits:           bits,
		TxTime:         fromTime,
	}
	for true {
		hashPoS, succ, ferr, minTarget := umint.CheckStakeKernelHash(&stpl)
		if ferr != nil {
			err = fmt.Errorf("check kernel hash error :%v\n", ferr)
			return
		}
		if succ {
			hashInt := new(big.Int).SetBytes(hashPoS)
			comp := umint.IncCompact(umint.BigToCompact(minTarget))
			maximumDiff := umint.CompactToDiff(comp)
			mar, _ := json.Marshal(stpl)
			fmt.Printf("%v %v %v %v\n", time.Unix(stpl.TxTime, 0),
				hashInt.BitLen(), maximumDiff, string(mar))
		}
		stpl.TxTime++
		if stpl.TxTime > maxTime {
			break
		}
	}
	return
}

func isProtocolV03(params *btcnet.Params, nTime int64) bool {
	var switchTime int64
	if params.Name == "testnet3" {
		switchTime = nProtocolV03TestSwitchTime
	} else {
		switchTime = nProtocolV03SwitchTime
	}
	return nTime >= switchTime
}
