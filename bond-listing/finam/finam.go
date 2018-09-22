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
	"time"
	"unicode"
)

type finamOptions struct {
	tradingMode string
	resultsType uint8

	fields []uint8
}

type finamBond struct {
	Name string

	Bid, Ask float64

	SecuritiesCount   uint32
	TransactionsCount uint32

	TradeVolume float64
}

const (
	finamBondFieldSkip = iota
	finamBondFieldName
	finamBondFieldBid
	finamBondFieldAsk
	finamBondFieldSecuritiesCount
	finamBondFieldTransactionsCount
	finamBondFieldTradeVolume
	finamBondFieldLast
)

func ParseFinam(rqdate time.Time) map[string]finamBond {
	options := []finamOptions{
		finamOptions{
			tradingMode: "T0",
			resultsType: 1,
			fields: []uint8{
				finamBondFieldName, finamBondFieldBid,
				finamBondFieldAsk, finamBondFieldSkip, finamBondFieldSecuritiesCount,
				finamBondFieldTradeVolume, finamBondFieldTransactionsCount, finamBondFieldLast,
			},
		},
		finamOptions{
			tradingMode: "T+",
			resultsType: 5,
			fields: []uint8{
				finamBondFieldSkip, finamBondFieldName, finamBondFieldBid,
				finamBondFieldAsk, finamBondFieldSkip, finamBondFieldSecuritiesCount,
				finamBondFieldTradeVolume, finamBondFieldTransactionsCount, finamBondFieldLast,
			},
		},
	}

	result := make(map[string]finamBond)
	for i := range options {
		parseFinamBonds(result, rqdate, options[i])
	}

	return result
}

func parseFinamBonds(bonds map[string]finamBond, rqdate time.Time, opt finamOptions) {
	url := fmt.Sprintf("http://bonds.finam.ru/trades/today/rqdate%s/default.asp?order=1&resultsType=%v&close=off&bid=on&ask=on&tradesOnly=1&page=0",
		fmt.Sprintf("%X%02X%02X", rqdate.Year(), int(rqdate.Month()), rqdate.Day()), opt.resultsType)

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	root, err := html.Parse(charmap.Windows1251.NewDecoder().Reader(resp.Body))
	if err != nil {
		log.Fatal(err)
	}

	// There are several elements which ends with `tr',
	//  so we must go a beet deeper to find the proper one
	th := util.ExtractNodeByPath(root, []string{
		"html", "body", "div", "div", "div",
		"table", "tbody", "tr", "td", "div",
		"table", "tbody", "tr", "th",
	})
	if th == nil {
		log.Fatal("finam result table was not found")
	}

	initialLen := len(bonds)

	var skipped uint32
	for tr := th.Parent.Parent.NextSibling.FirstChild; tr != nil; tr = tr.NextSibling {
		if tr.Type != html.ElementNode {
			continue
		}

		b := parseFinamBond(tr, opt)
		if b != nil {
			bonds[bond.NormalizeBondShortName(b.Name)] = *b
		} else {
			skipped++
		}
	}

	log.Printf("Finam `%s': %v found, %v skipped\n", opt.tradingMode, len(bonds)-initialLen, skipped)
}

func parseFinamBond(node *html.Node, opt finamOptions) *finamBond {
	var err error
	var i int

	b := &finamBond{}
	for c := node.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}

		field := opt.fields[i]
		i++

		if field == finamBondFieldSkip {
			continue
		}
		if field == finamBondFieldLast {
			break
		}

		text := util.ExtractTextNode(c)
		if text == nil {
			continue
		}

		data := strings.Map(func(r rune) rune {
			if field == finamBondFieldName {
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
		case finamBondFieldName:
			b.Name = data
		case finamBondFieldBid:
			b.Bid, err = strconv.ParseFloat(data, 64)
			if err != nil {
				log.Fatal("can't parse bid: ", err)
			}
		case finamBondFieldAsk:
			b.Ask, err = strconv.ParseFloat(data, 64)
			if err != nil {
				log.Fatal("can't parse ask: ", err)
			}
		case finamBondFieldSecuritiesCount:
			v, err := strconv.ParseUint(data, 10, 32)
			if err != nil {
				log.Fatal("can't parse securities count: ", err)
			}

			b.SecuritiesCount = uint32(v)
		case finamBondFieldTransactionsCount:
			v, err := strconv.ParseUint(data, 10, 32)
			if err != nil {
				log.Fatal("can't parse transactions count: ", err)
			}

			b.TransactionsCount = uint32(v)
		case finamBondFieldTradeVolume:
			b.TradeVolume, err = strconv.ParseFloat(data, 64)
			if err != nil {
				log.Fatal("can't parse trade volume: ", err)
			}
		default:
			panic("unknown finam bond field")
		}
	}
	if i != len(opt.fields) {
		log.Println("finam: skip partially parsed bond")
		return nil
	}

	return b
}
