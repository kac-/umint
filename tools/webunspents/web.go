package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/conformal/goleveldb/leveldb"
	"github.com/kac-/umint/utxo"
	"github.com/mably/btcnet"
	"github.com/mably/btcutil"
	"github.com/mably/btcwire"
	"net/http"
	"strconv"
	"strings"
)

var (
	dbPath string
	listen string
	params = &btcnet.MainNetParams
)

func init() {
	flag.StringVar(&dbPath, "db", "", "unpent database path")
	flag.StringVar(&listen, "s", ":9999", "listen on [ip]:port")
	flag.Parse()
}

func main() {
	if dbPath == "" {
		fmt.Println("ERR: db path required")
		flag.Usage()
		return
	}
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		fmt.Printf("ERR: open db(%v): %v", dbPath, err)
	}
	defer db.Close()
	height, time, err := utxo.FetchHeight(db)
	if err != nil {
		fmt.Printf("ERR: fetch height(%v): %v\n", dbPath, err)
		return
	}
	fmt.Printf("db path: %v height: %v time: %v\n", dbPath, height, time)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		r.Body.Close()
		p := r.URL.Path[1:]
		if len(p) < 26 {
			return
		}
		if len(p) < 64 {
			addrStr := p
			skip := uint(0)
			skipStr, hasSkip := r.URL.Query()["skip"]
			//fmt.Fprintf(w, "skip: %#v %#v\n", skipStr, hasSkip)
			if hasSkip {
				s, err := strconv.Atoi(skipStr[0])
				if err != nil || s < 0 {
					fmt.Fprintf(w, "ERR: invalid skip arg: '%v'", skipStr[0])
					return
				}
				skip = uint(s)
			}
			decoded, err := btcutil.DecodeAddress(addrStr, params)
			if err != nil {
				fmt.Fprintf(w, "ERR: invalid address(%v): %v\n", addrStr, err)
				return
			}
			addr, ok := decoded.(*btcutil.AddressPubKeyHash)
			if !ok {
				fmt.Fprintf(w, "ERR: pub key hash address expected: %v\n", addrStr)
				return
			}
			points, complete, err := utxo.FetchOutPoints(db, addr, 100, skip)
			if err != nil {
				fmt.Fprintf(w, "ERR: fetch outPoints(%v): %v\n", addr, err)
				return
			}
			for _, point := range points {
				by := point.Hash.Bytes()
				for i := 0; i < 16; i++ {
					by[i], by[31-i] = by[31-i], by[i]
				}
				fmt.Fprintf(w, "%x:%d\n", by, point.Index)
			}
			fmt.Fprintln(w, complete)
		} else {
			sa := strings.Split(p, ":")
			if len(sa) != 2 {
				fmt.Fprintf(w, "ERR: invalid format of TX:IDX - %s\n", p)
				return
			}
			txSha, err := btcwire.NewShaHashFromStr(sa[0])
			if len(sa[0]) < 64 || err != nil {
				fmt.Fprintf(w, "ERR: invalid TX    - %s\n", sa[0])
				return
			}
			outputIdx, err := strconv.Atoi(sa[1])
			if err != nil {
				fmt.Fprintf(w, "ERR: invalid IDX    - %s\n", sa[1])
				return
			}
			outPoint := btcwire.NewOutPoint(txSha, uint32(outputIdx))
			u, err := utxo.FetchUTXO(db, outPoint)
			if err != nil {
				if strings.HasSuffix(err.Error(), "leveldb: not found") {
					fmt.Fprintln(w, "ERR: not found")
				} else {
					fmt.Printf("ERR: fetch utxo: %v\n", err)
					fmt.Fprintln(w, "ERR: internal")
				}
				return
			}
			by, err := json.Marshal(u)
			if err != nil {
				fmt.Fprintln(w, "ERR: internal")
				return
			}
			fmt.Fprintln(w, string(by))
		}
	})
	if err = http.ListenAndServe(listen, nil); err != nil {
		fmt.Printf("ERR: listen and serve(%v): %v\n", listen, err)
	}
}
