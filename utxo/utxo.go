package utxo

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/conformal/goleveldb/leveldb"
	"github.com/mably/btcutil"
	"github.com/mably/btcwire"
	"time"
)

const (
	DB_UTXO byte = iota
	DB_ADDR
	DB_HEIGHT
	DB_MAX
)

type UTXO struct {
	BlockTime     uint32
	StakeModifier uint64
	OffsetInBlock uint32
	Time          uint32
	Value         uint64
	PkScript      []byte
}

func SerializeOutPoint(outPoint *btcwire.OutPoint) []byte {
	buf := make([]byte, 1+32+4)
	buf[0] = DB_UTXO
	copy(buf[1:], outPoint.Hash[:])
	binary.LittleEndian.PutUint32(buf[1+32:], outPoint.Index)
	return buf
}

func DeserializeOutPoint(buf []byte) *btcwire.OutPoint {
	outPoint := btcwire.OutPoint{}
	copy(outPoint.Hash[:], buf[1:1+32])
	outPoint.Index = binary.LittleEndian.Uint32(buf[1+32:])
	return &outPoint
}

func DeserializeUTXO(buf []byte) *UTXO {
	u := UTXO{}
	u.BlockTime = binary.LittleEndian.Uint32(buf[0:])
	u.StakeModifier = binary.LittleEndian.Uint64(buf[4:])
	u.OffsetInBlock = binary.LittleEndian.Uint32(buf[12:])
	u.Time = binary.LittleEndian.Uint32(buf[16:])
	u.Value = binary.LittleEndian.Uint64(buf[20:])
	u.PkScript = make([]byte, len(buf)-28)
	copy(u.PkScript, buf[28:])
	return &u
}

func SerializeUTXO(utxo *UTXO) []byte {
	scriptLen := len(utxo.PkScript)
	buf := make([]byte,
		4+ //blockFromTime uint32
			8+ //stakeModifier uint64
			4+ //txOffset uint32
			4+ //txTime uint32
			8+ //value uint64
			scriptLen)
	binary.LittleEndian.PutUint32(buf[0:], utxo.BlockTime)
	binary.LittleEndian.PutUint64(buf[4:], utxo.StakeModifier)
	binary.LittleEndian.PutUint32(buf[12:], utxo.OffsetInBlock)
	binary.LittleEndian.PutUint32(buf[16:], utxo.Time)
	binary.LittleEndian.PutUint64(buf[20:], utxo.Value)
	copy(buf[28:], utxo.PkScript)
	return buf
}

func FetchOutPoints(db *leveldb.DB, addr *btcutil.AddressPubKeyHash, count, skip uint) (outPoints []*btcwire.OutPoint, complete bool, err error) {
	key := make([]byte, 1+20)
	key[0] = DB_ADDR
	copy(key[1:], addr.ScriptAddress())
	iter := db.NewIterator(nil, nil)
	position := uint(0)
	for ok := iter.Seek(key); ok; ok, position = iter.Next(), position+1 {
		if !bytes.Equal(iter.Key()[:1+20], key) {
			complete = true
			break
		}
		if position >= skip {
			outPoints = append(outPoints, DeserializeOutPoint(iter.Value()))
			if count > 0 && uint(len(outPoints)) == count {
				break
			}
		}
	}
	iter.Release()
	err = iter.Error()
	if err != nil {
		err = fmt.Errorf("iterator error: %v", err)
	}
	return
}

func FetchCoins(db *leveldb.DB, addr *btcutil.AddressPubKeyHash) ([]*btcwire.OutPoint, []*UTXO, error) {
	var err error
	key := make([]byte, 1+20)
	key[0] = DB_ADDR
	copy(key[1:], addr.ScriptAddress())
	iter := db.NewIterator(nil, nil)
	var outPoints [][]byte
	for ok := iter.Seek(key); ok; ok = iter.Next() {
		if !bytes.Equal(iter.Key()[:1+20], key) {
			break
		}
		val := make([]byte, len(iter.Value()))
		copy(val, iter.Value())
		outPoints = append(outPoints, val)

		//fmt.Println(time.Unix(int64(binary.LittleEndian.Uint32(iter.Key()[1+20:1+24])),0))
		//fmt.Println(iter.Key()[1+20:1+24])
	}
	iter.Release()
	err = iter.Error()
	if err != nil {
		return nil, nil, fmt.Errorf("iterating over address entries: %v", err)
	}
	outs := make([]*btcwire.OutPoint, len(outPoints))
	utxos := make([]*UTXO, len(outPoints))
	for i, outPoint := range outPoints {
		value, err := db.Get(outPoint, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("error getting utxo: %v", err)
		}
		outs[i] = DeserializeOutPoint(outPoint)
		utxos[i] = DeserializeUTXO(value)
	}
	return outs, utxos, nil
}

func FetchUTXO(db *leveldb.DB, outPoint *btcwire.OutPoint) (*UTXO, error) {
	value, err := db.Get(SerializeOutPoint(outPoint), nil)
	if err != nil {
		return nil, fmt.Errorf("fetching utxo(%v): %v", outPoint, err)
	}
	return DeserializeUTXO(value), nil
}

func FetchHeight(db *leveldb.DB) (topHeight uint32, topTime time.Time, err error) {
	value, err := db.Get([]byte{DB_HEIGHT}, nil)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("db error: %v", err)
	}
	if len(value) < 8 {
		return 0, time.Time{}, fmt.Errorf("invalid 'height' record length: %v", len(value))
	}
	return binary.LittleEndian.Uint32(value[0:4]),
		time.Unix(int64(binary.LittleEndian.Uint32(value[4:8])), 0), nil
}

func FetchHeightFile(dbDir string) (topHeight uint32, topTime time.Time, err error) {
	var db *leveldb.DB
	db, err = leveldb.OpenFile(dbDir, nil)
	if err != nil {
		err = fmt.Errorf("open unspent db(%v): %v", dbDir, err)
		return
	}
	defer db.Close()
	topHeight, topTime, err = FetchHeight(db)
	if err != nil {
		err = fmt.Errorf("fetch height from db(%v): %v", dbDir, err)
		return
	}
	return
}
