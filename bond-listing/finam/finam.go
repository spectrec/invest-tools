package finam

import (
	"fmt"
	"github.com/spectrec/invest-tools/bond"
	util "github.com/spectrec/invest-tools/bond-listing/html"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type Bond struct {
	Name string

	Bid, Ask float64

	SecuritiesCount   uint32
	TransactionsCount uint32
	TradeVolume       float64
}

const (
	fieldSkip = iota
	fieldName
	fieldBid
	fieldAsk
	fieldSecuritiesCount
	fieldTransactionsCount
	fieldTradeVolume
	fieldLast
)

type options struct {
	tradingMode string
	resultsType uint8

	fields []uint8

	debug bool
}

func DownloadAndParse(rqdate time.Time, debug bool) (map[string]Bond, error) {
	options := []options{
		options{
			tradingMode: "T0",
			resultsType: 1,
			fields: []uint8{
				fieldName, fieldBid,
				fieldAsk, fieldSkip, fieldSecuritiesCount,
				fieldTradeVolume, fieldTransactionsCount, fieldLast,
			},
			debug: debug,
		},
		options{
			tradingMode: "T+",
			resultsType: 5,
			fields: []uint8{
				fieldSkip, fieldName, fieldBid,
				fieldAsk, fieldSkip, fieldSecuritiesCount,
				fieldTradeVolume, fieldTransactionsCount, fieldLast,
			},
			debug: debug,
		},
	}

	// Collect results from the last week about transactions
	const days = 3

	wg := sync.WaitGroup{}
	wg.Add(days)

	results := make([]map[string]Bond, days)
	for d := 0; d < days; d++ {
		go func(d int, wg *sync.WaitGroup) {
			defer wg.Done()

			log.Printf("Dowlading finam bonds, day %v of %v ...", d, days)

			date := rqdate.AddDate(0, 0, -d)

			res := make(map[string]Bond)
			for i := range options {
				if err := parseFinamBonds(res, date, &options[i]); err != nil {
					log.Fatal("finam failed: ", err)
				}
			}

			results[d] = res

			log.Printf("Dowlading finam bonds, day %v of %v done", d, days)
		}(d, &wg)
	}
	wg.Wait()

	result := make(map[string]Bond)
	for d := 0; d < days; d++ {
		for name, b := range results[d] {
			v, exist := result[name]
			if !exist {
				// Store all new values as is
				result[name] = b
				continue
			}

			// Take only information about transactions if value
			// already exist
			v.SecuritiesCount += b.SecuritiesCount
			v.TransactionsCount += b.TransactionsCount
			v.TradeVolume += b.TradeVolume

			result[name] = v
		}
	}

	return result, nil
}

func parseFinamBonds(bonds map[string]Bond, rqdate time.Time, opt *options) error {
	url := fmt.Sprintf("http://bonds.finam.ru/trades/today/rqdate%s/default.asp?order=1&resultsType=%v&close=off&bid=on&ask=on&tradesOnly=1&page=0",
		fmt.Sprintf("%X%02X%02X", rqdate.Year(), int(rqdate.Month()), rqdate.Day()), opt.resultsType)

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	root, err := html.Parse(charmap.Windows1251.NewDecoder().Reader(resp.Body))
	if err != nil {
		return err
	}

	// There are several elements which ends with `tr',
	//  so we must go a beet deeper to find the proper one
	th := util.ExtractNodeByPath(root, []string{
		"html", "body", "div", "div", "div",
		"table", "tbody", "tr", "td", "div",
		"table", "tbody", "tr", "th",
	})
	if th == nil {
		return fmt.Errorf("result table was not found")
	}

	initialLen := len(bonds)

	var skipped uint32
	for tr := th.Parent.Parent.NextSibling.FirstChild; tr != nil; tr = tr.NextSibling {
		if tr.Type != html.ElementNode {
			continue
		}

		b, err := parseFinamBond(tr, opt)
		if err != nil {
			return err
		}

		if b == nil {
			skipped++
			continue
		}

		bonds[bond.NormalizeBondShortName(b.Name)] = *b
	}

	if opt.debug {
		fmt.Printf("Finam `%s': %v found, %v skipped\n", opt.tradingMode, len(bonds)-initialLen, skipped)
	}

	return nil
}

func parseFinamBond(node *html.Node, opt *options) (*Bond, error) {
	var err error
	var i int

	b := &Bond{}
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if i == len(opt.fields) {
			// All required fields have been checked
			break
		}

		if c.Type != html.ElementNode {
			continue
		}

		field := opt.fields[i]
		i++

		if field == fieldSkip {
			continue
		}
		if field == fieldLast {
			break
		}

		text := util.ExtractTextNode(c)
		if text == nil {
			continue
		}

		data := strings.Map(func(r rune) rune {
			if field == fieldName {
				return r
			}

			if unicode.IsSpace(r) {
				return -1
			} else if r == ',' {
				return '.'
			}

			return r
		}, text.Data)
		if data == "" {
			continue
		}

		switch field {
		case fieldName:
			b.Name = data
		case fieldBid:
			b.Bid, err = strconv.ParseFloat(data, 64)
			if err != nil {
				return nil, fmt.Errorf("can't parse bid: ", err)
			}
		case fieldAsk:
			b.Ask, err = strconv.ParseFloat(data, 64)
			if err != nil {
				return nil, fmt.Errorf("can't parse ask: ", err)
			}
		case fieldSecuritiesCount:
			v, err := strconv.ParseUint(data, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("can't parse securities count: ", err)
			}

			b.SecuritiesCount = uint32(v)
		case fieldTransactionsCount:
			v, err := strconv.ParseUint(data, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("can't parse transactions count: ", err)
			}

			b.TransactionsCount = uint32(v)
		case fieldTradeVolume:
			b.TradeVolume, err = strconv.ParseFloat(data, 64)
			if err != nil {
				return nil, fmt.Errorf("can't parse trade volume: ", err)
			}
		default:
			panic("unknown finam bond field")
		}
	}
	if i != len(opt.fields) {
		log.Println("finam: skip partially parsed bond")
		return nil, nil
	}

	return b, nil
}
