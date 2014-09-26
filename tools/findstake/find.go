package main

import (
	"flag"
	"fmt"
	"github.com/mably/btcchain"
	"github.com/mably/btcdb"
	"github.com/mably/btcnet"
	"github.com/mably/btcwire"
	"os"
	"strconv"
	"strings"
	"time"
)

const ()

var (
	testnet     bool
	diff        float64
	days        uint
	startString string
)

const (
	nProtocolV03SwitchTime     int64 = 1363800000
	nProtocolV03TestSwitchTime int64 = 1359781000
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s: DB_PATH TX:IDX\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.BoolVar(&testnet, "testnet", false, "use testnet params")
	flag.Float64Var(&diff, "diff", -0.8, "display success on diff [default: use 80% of current diff]")
	flag.UintVar(&days, "days", 7, "number of days to check")
	flag.StringVar(&startString, "from", "now", "date from which scan [f.e. 2014-09-12]")
	flag.Parse()
}

func main() {
	var dbPath string
	// db path
	if len(flag.Args()) < 1 {
		fmt.Println("DB_PATH not specified")
		flag.Usage()
		return
	}
	dbPath = flag.Arg(0)
	dirInfo, err := os.Stat(dbPath)
	if err != nil {
		fmt.Printf("%s: %v\n", dbPath, err)
		return
	}
	if !dirInfo.IsDir() {
		fmt.Printf("directory expected: %s\n", dbPath)
		return
	}
	// tx:idx
	if len(flag.Args()) < 2 {
		fmt.Println("TX:IDX not specified")
		flag.Usage()
		return
	}
	tmp := flag.Arg(1)
	sa := strings.Split(tmp, ":")
	if len(sa) != 2 {
		fmt.Printf("invalid format of TX:IDX    - %s\n", tmp)
		return
	}
	txSha, err := btcwire.NewShaHashFromStr(sa[0])
	if len(sa[0]) < 64 || err != nil {
		fmt.Printf("invalid TX    - %s\n", sa[0])
		return
	}
	outputIdx, err := strconv.Atoi(sa[1])
	if err != nil {
		fmt.Printf("invalid IDX    - %s\n", sa[1])
		return
	}
	outPoint := btcwire.NewOutPoint(txSha, uint32(outputIdx))

	// -from
	start := time.Now()
	if startString != "now" {
		start, err = time.Parse("2006-01-02", startString)
		if err != nil {
			fmt.Printf("invalid -from: %v\n", err)
			return
		}
	}
	end := start.Add(time.Hour * time.Duration(24*days))

	// done, now fire
	fmt.Printf(
		`db:     %v
tx:      %v
idx:     %v
testnet: %v
start:   %v
end:     %v
`, dbPath, txSha, outputIdx, testnet, start, end)
	var params *btcnet.Params
	if testnet {
		params = &btcnet.TestNet3Params
	} else {
		params = &btcnet.MainNetParams
	}
	db, err := btcdb.OpenDB("leveldb", dbPath)
	if err != nil {
		fmt.Printf("opening db: %v\n", err)
		return
	}
	_, bestChainHeight, _ := db.NewestSha()
	fmt.Printf("height:  %v\n", bestChainHeight)
	c := btcchain.New(db, params, nil)
	err = c.GenerateInitialIndex()
	if err != nil {
		fmt.Printf("generate initial idx: %v\n", err)
		return
	}

	err = findStake(outPoint, db, c, params, start.Unix(), end.Unix(), float32(diff))
	if err != nil {
		fmt.Printf("error while searching: %v\n", err)
	}
}
