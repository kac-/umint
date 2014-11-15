package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/conformal/goleveldb/leveldb"
	"github.com/kac-/umint/utxo"
	"github.com/mably/btcnet"
	"github.com/mably/btcutil"
	"github.com/mably/btcwire"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s: [ADDR|TX:IDX]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Float64Var(&diff, "diff", 10.0, "display success on diff ")
	flag.UintVar(&days, "days", 7, "number of days to check")
	flag.StringVar(&startString, "from", "now", "date from which scan [i.e. 2014-09-12]")
	flag.Parse()
}

func main() {
	var (
		err            error
		params         = &btcnet.MainNetParams
		addrOrOutPoint string
		addr           *btcutil.AddressPubKeyHash
		outPoint       *btcwire.OutPoint
	)

	configSeelog()
	defer log.Flush()

	url := "http://kac-pub.s3.amazonaws.com/post/cryptos/peercoin//unspent-141k.tar.gz"

	appHome := btcutil.AppDataDir("ppc-umint", false)
	if err := os.MkdirAll(appHome, 0777); err != nil {
		log.Errorf("create app home(%v): %v\n", appHome, err)
		return
	}
	dbDestinationDir := filepath.Join(appHome, "unspent_db")
	topHeight, topTime, err := utxo.FetchHeightFile(dbDestinationDir)
	if err != nil || topHeight != 142000 {
		var dbTempDir string
		dbTempDir, topHeight, topTime, err = DownloadDB(url)
		defer os.RemoveAll(dbTempDir)
		if err != nil {
			log.Errorf("downloading database failed: %v\n", err)
			return
		}
		err = os.RemoveAll(dbDestinationDir)
		if err != nil {
			log.Errorf("remove database dir(%v): %v\n", dbDestinationDir, err)
			return
		}
		err = os.Rename(dbTempDir, dbDestinationDir)
		if err != nil {
			if strings.HasSuffix(err.Error(), ": invalid cross-device link") {
				// destination is on different partition, we need to copy directory
				err = CopyFile(dbTempDir, dbDestinationDir)
				if err != nil {
					log.Errorf("copying db from %v to %v failed: %v", dbTempDir, dbDestinationDir, err)
					// cleanup
					os.RemoveAll(dbDestinationDir)
					return
				}
			} else {
				log.Errorf("rename/move %v to %v: %v\n", dbTempDir, dbDestinationDir, err)
				return
			}
		}
	}
	log.Infof("got db: %v blocks (%v)", topHeight, topTime.Format("2006-01-02 15:04:05"))

	// db path
	if len(flag.Args()) < 1 {
		fmt.Println("arg required")
		flag.Usage()
		return
	}
	addrOrOutPoint = flag.Arg(0)
	if len(addrOrOutPoint) > 64 { //TXID:IDX
		sa := strings.Split(addrOrOutPoint, ":")
		if len(sa) != 2 {
			fmt.Printf("invalid format of TX:IDX    - %s\n", addrOrOutPoint)
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
		outPoint = btcwire.NewOutPoint(txSha, uint32(outputIdx))
	} else { // ADDR
		decoded, err := btcutil.DecodeAddress(addrOrOutPoint, params)
		if err != nil {
			fmt.Printf("invalid address(%v): %v\n", addrOrOutPoint, err)
			return
		}
		var ok bool
		addr, ok = decoded.(*btcutil.AddressPubKeyHash)
		if !ok {
			fmt.Printf("pub key hash address expected: %v\n", addrOrOutPoint)
			return
		}
	}

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
	if outPoint != nil {
		log.Infof(`params:
tx:      %v
idx:     %v
start:   %v
end:     %v
diff:    %v
`, outPoint.Hash, outPoint.Index, start, end, diff)
	} else {
		log.Infof(`params:
addr:    %v
start:   %v
end:     %v
diff:    %v
`, addr.EncodeAddress(), start, end, diff)
	}

	db, err := leveldb.OpenFile(dbDestinationDir, nil)
	if err != nil {
		fmt.Printf("opening db: %v\n", err)
		return
	}
	if addr != nil {
		ups, _, err := utxo.FetchCoins(db, addr)
		if err != nil {
			log.Criticalf("fetching coins for %v: %v", addr.EncodeAddress(), err)
			return
		}
		for i := range ups {
			err = findStake(ups[i], db, params, start.Unix(), end.Unix(), float32(diff))
			if err != nil {
				log.Errorf("error while searching: %v", err)
			}
		}
	} else {
		err = findStake(outPoint, db, params, start.Unix(), end.Unix(), float32(diff))
		if err != nil {
			log.Errorf("error while searching: %v", err)
		}
	}
	_ = addr
}

func DownloadDB(url string) (dbTempDir string, topHeight uint32, topTime time.Time, err error) {
	var file *os.File
	prefix := filepath.Join(os.TempDir(), fmt.Sprintf("db-download-%v-", time.Now().Unix()))
	filename := url[strings.LastIndex(url, "/")+1:]
	if !strings.HasSuffix(filename, "tar.gz") {
		err = fmt.Errorf("insupported db archive: %v", filename)
		return
	}
	downloadTo := prefix + filename
	dbTempDir = prefix + "db"

	defer os.Remove(downloadTo)
	func() { // download, wrap to close files w/ defer
		var resp *http.Response

		log.Infof("downloading %v to %v", url, downloadTo)
		file, err = os.Create(downloadTo)
		if err != nil {
			err = fmt.Errorf("create download destination file(%v): %v", downloadTo, err)
			return
		}
		defer file.Close()

		resp, err = http.Get(url)
		if err != nil {
			err = fmt.Errorf("db archive http request(%v): %v", url, err)
			return
		}
		defer resp.Body.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			err = fmt.Errorf("copy data from %v to %v: %v", url, downloadTo, err)
			return
		}
	}()

	// unpack
	var (
		th *tar.Header
		gr *gzip.Reader
	)
	log.Infof("unpacking %v to %v", downloadTo, dbTempDir)

	err = os.MkdirAll(dbTempDir, 0777)
	if err != nil {
		err = fmt.Errorf("create temp db dir(%v): %v", dbTempDir, err)
		return
	}
	file, err = os.Open(downloadTo)
	if err != nil {
		err = fmt.Errorf("open db archive(%v): %v", downloadTo, err)
		return
	}
	defer file.Close()
	gr, err = gzip.NewReader(file)
	if err != nil {
		err = fmt.Errorf("open gzip reader(%v): %v", downloadTo, err)
		return
	}
	tr := tar.NewReader(gr)
	for th, err = tr.Next(); err == nil; {
		fi := th.FileInfo()
		if !fi.IsDir() { // there are only files
			func() { // wrap to close files w/ defer
				fn := filepath.Join(dbTempDir, fi.Name())
				file, err = os.Create(fn)
				if err != nil {
					err = fmt.Errorf("create archived file(%v): %v", fn, err)
					return
				}
				defer file.Close()
				_, err = io.Copy(file, tr)
				if err != nil {
					err = fmt.Errorf("copy tar data to file(%v): %v", fn, err)
					return
				}
			}()
		}
		th, err = tr.Next()
	}
	if err != io.EOF {
		err = fmt.Errorf("archive error(%v): %v", downloadTo, err)
		return
	}

	// test db
	topHeight, topTime, err = utxo.FetchHeightFile(dbTempDir)

	return
}

func CopyFile(src string, dst string) error {
	srcLen := len(src)
	err := filepath.Walk(src, func(path string, f os.FileInfo, err error) error {
		dpath := dst + path[srcLen:]
		//fmt.Printf("%s (%v) -> %v\n", path, f.Name(), dpath)
		if f.IsDir() {
			err = os.MkdirAll(dpath, 0777)
			if err != nil {
				return fmt.Errorf("mkdir(%v): %v", dpath, err)
			}
		} else {
			in, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("open file(%v): %v", path, err)
			}
			defer in.Close()
			out, err := os.Create(dpath)
			if err != nil {
				return fmt.Errorf("create file(%v): %v", dpath, err)
			}
			defer out.Close()
			_, err = io.Copy(out, in)
			if err != nil {
				return fmt.Errorf("copy content from %v to %v: %v", path, dpath, err)
			}
		}
		return nil
	})
	return err
}

func configSeelog() {
	l, _ := log.LoggerFromConfigAsString(`
<seelog>
	<outputs formatid="main">
		<console />
	</outputs>
	<formats>
		<format id="main" format="[%Level] %Msg%n"/>
	</formats>
</seelog>
`)
	log.ReplaceLogger(l)
}
