package main

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/conformal/goleveldb/leveldb"
	"github.com/kac-/umint"
	"github.com/kac-/umint/utxo"
	"github.com/mably/btcnet"
	"github.com/mably/btcwire"
	"time"
)

func findStake(outPoint *btcwire.OutPoint, db *leveldb.DB,
	params *btcnet.Params, fromTime int64, maxTime int64, diff float32) (err error) {
	utx, err := utxo.FetchUTXO(db, outPoint)
	if err != nil {
		return fmt.Errorf("fetch utxo(%v): %v", outPoint, err)
	}
	log.Infof("CHECK %v PPCs from %v https://bkchain.org/ppc/tx/%v#o%v",
		float64(utx.Value)/1000000.0, time.Unix(int64(utx.Time), 0).Format("2006-01-02"),
		outPoint.Hash, outPoint.Index)

	var bits uint32

	bits = umint.BigToCompact(umint.DiffToTarget(diff))

	stpl := umint.StakeKernelTemplate{
		BlockFromTime:  int64(utx.BlockTime),
		StakeModifier:  utx.StakeModifier,
		PrevTxOffset:   utx.OffsetInBlock,
		PrevTxTime:     int64(utx.Time),
		PrevTxOutIndex: outPoint.Index,
		PrevTxOutValue: int64(utx.Value),
		IsProtocolV03:  true,
		StakeMinAge:    params.StakeMinAge,
		Bits:           bits,
		TxTime:         fromTime,
	}
	for true {
		_, succ, ferr, minTarget := umint.CheckStakeKernelHash(&stpl)
		if ferr != nil {
			err = fmt.Errorf("check kernel hash error :%v", ferr)
			return
		}
		if succ {
			comp := umint.IncCompact(umint.BigToCompact(minTarget))
			maximumDiff := umint.CompactToDiff(comp)
			log.Infof("MINT %v %v", time.Unix(stpl.TxTime, 0),
				maximumDiff)
		}
		stpl.TxTime++
		if stpl.TxTime > maxTime {
			break
		}
	}
	return
}
