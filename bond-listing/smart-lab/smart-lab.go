package smartlab

import (
	"github.com/spectrec/invest-tools/bond"
	util "github.com/spectrec/invest-tools/bond-listing/html"
	"golang.org/x/net/html"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type bondField uint8

const (
	bondFieldSkip = iota
	bondFieldName
	bondFieldMaturityDate
	bondFieldOfferDate
	bondFieldAccruedInterest
	bondFieldCleanPrice
)

type bondOptions struct {
	BondType   bond.BondType
	couponTax  float64
	url        string
	fields     []string
	bondFields []bondField
}

func ParseOptions(t bond.BondType) *bondOptions {
	switch t {
	case bond.TypeCorp:
		return &bondOptions{
			BondType:  t,
			couponTax: 13.0,
			url:       "https://smart-lab.ru/q/bonds/",
			fields: []string{
				"№", "Время", "Имя", "",
				"Погашение", "Лет до", "Доходн", "Год.куп.",
				"Куп.дох.", "Цена", "Объем, млн руб", "Купон, руб",
				"Частота,", "НКД, руб", "Дюр-я, лет", "Дата купона", "Оферта",
			},
			bondFields: []bondField{
				bondFieldSkip, bondFieldSkip, bondFieldName, bondFieldSkip,
				bondFieldMaturityDate, bondFieldSkip, bondFieldSkip, bondFieldSkip,
				bondFieldSkip, bondFieldCleanPrice, bondFieldSkip, bondFieldSkip,
				bondFieldSkip, bondFieldAccruedInterest, bondFieldSkip, bondFieldSkip, bondFieldOfferDate,
			},
		}
	case bond.TypeGov:
		return &bondOptions{
			BondType: t,
			url:      "https://smart-lab.ru/q/ofz/",
			fields: []string{
				"№", "Время", "Имя", "",
				"Погашение", "Лет до", "Доходн", "!", "Год.куп.",
				"Куп.дох.", "Цена", "Объем,", "Купон, руб",
				"Частота,", "НКД, руб", "Дюр-я, лет", "Дата купона",
			},
			bondFields: []bondField{
				bondFieldSkip, bondFieldSkip, bondFieldName, bondFieldSkip,
				bondFieldMaturityDate, bondFieldSkip, bondFieldSkip, bondFieldSkip, bondFieldSkip,
				bondFieldSkip, bondFieldCleanPrice, bondFieldSkip, bondFieldSkip,
				bondFieldSkip, bondFieldAccruedInterest, bondFieldSkip, bondFieldSkip,
			},
		}
	case bond.TypeMun:
		return &bondOptions{
			BondType: t,
			url:      "https://smart-lab.ru/q/subfed/",
			fields: []string{
				"№", "Время", "Имя", "",
				"Погашение", "Лет до", "Доходн", "Год.куп.",
				"Куп.дох.", "Цена", "Объем, млн руб", "Купон, руб",
				"Частота,", "НКД, руб", "Дюр-я, лет", "Дата купона", "Оферта",
			},
			bondFields: []bondField{
				bondFieldSkip, bondFieldSkip, bondFieldName, bondFieldSkip,
				bondFieldMaturityDate, bondFieldSkip, bondFieldSkip, bondFieldSkip,
				bondFieldSkip, bondFieldCleanPrice, bondFieldSkip, bondFieldSkip,
				bondFieldSkip, bondFieldAccruedInterest, bondFieldSkip, bondFieldSkip, bondFieldOfferDate,
			},
		}
	case bond.TypeEuro:
		return &bondOptions{
			BondType: t,
			url:      "https://smart-lab.ru/q/eurobonds/",
			fields: []string{
				"№", "Время", "Имя", "",
				"Погашение", "Лет до", "Доходн", "Год.куп.дох.",
				"Куп.дох.", "Цена", "Объем, тыс. $", "Купон, $",
				"Частота,", "НКД, $", "Дата купона", "Оферта",
			},
			bondFields: []bondField{
				bondFieldSkip, bondFieldSkip, bondFieldName, bondFieldSkip,
				bondFieldMaturityDate, bondFieldSkip, bondFieldSkip, bondFieldSkip,
				bondFieldSkip, bondFieldCleanPrice, bondFieldSkip, bondFieldSkip,
				bondFieldSkip, bondFieldAccruedInterest, bondFieldSkip, bondFieldOfferDate,
			},
		}
	default:
		return &bondOptions{
			BondType: bond.TypeUndef,
		}
	}
}

func ParseBonds(bonds []*bond.Bond, opt *bondOptions) []*bond.Bond {
	resp, err := http.Get(opt.url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	root, err := html.Parse(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	tbody := util.ExtractNodeByPath(root, []string{"html", "body", "div", "div", "table", "tbody"})

	var headerChecked bool
	var found, skipped uint32
	for tr := tbody.FirstChild; tr != nil; tr = tr.NextSibling {
		if tr.Type != html.ElementNode {
			continue
		}

		if !headerChecked {
			checkSmartLabHeader(tr, opt)
			headerChecked = true

			continue
		}

		bond := parseSmartLabBond(tr, opt)
		if bond != nil {
			bonds = append(bonds, bond)
			found++
		} else {
			skipped++
		}
	}

	log.Printf("`%s' bonds: %v found, %v skipped\n", opt.BondType, found, skipped)

	return bonds
}

func parseSmartLabBond(root *html.Node, opt *bondOptions) *bond.Bond {
	var err error
	var i int

	bond := &bond.Bond{Type: opt.BondType}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}

		field := opt.bondFields[i]
		i++

		if field == bondFieldSkip {
			continue
		}

		text := util.ExtractTextNode(c)
		if text == nil || text.Data == "" {
			continue
		}

		switch field {
		case bondFieldName:
			bond.ShortName = text.Data
			bond.ISIN = extractSmartLabISIN(text.Parent)
		case bondFieldAccruedInterest:
			bond.AccruedInterst, err = strconv.ParseFloat(text.Data, 64)
			if err != nil {
				log.Fatal("can't parse accrued interest: ", err)
			}
		case bondFieldCleanPrice:
			bond.CleanPricePercent, err = strconv.ParseFloat(text.Data, 64)
			if err != nil {
				log.Fatal("can't parse clean price: ", err)
			}
		case bondFieldMaturityDate:
			bond.MaturityDate = extractDate(text.Data)
			if bond.MaturityDate == nil {
				// log.Printf("skip `%s', maturity date not found\n", bond.ISIN)
				return nil
			}
		case bondFieldOfferDate:
			bond.OfferDate = extractDate(text.Data)
		default:
			panic("unknown bond field")
		}
	}
	if i != len(opt.bondFields) {
		// log.Println("skip partially parsed bond")
		return nil
	}

	return bond
}

func checkSmartLabHeader(tr *html.Node, opt *bondOptions) {
	checked := 0
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}

		if opt.fields[checked] == "" {
			// just ignore it
		} else if v := util.ExtractTextNode(c); v.Data != opt.fields[checked] {
			log.Fatalf("header format changed, expected `%s', got `%s'\n",
				opt.fields[checked], v.Data)
		}

		checked++
	}

	if checked != len(opt.fields) {
		log.Fatalf("header format changed, expected %v fields, checked %v fields\n",
			len(opt.fields), checked)
	}
}

func extractSmartLabISIN(node *html.Node) string {
	if node.Type != html.ElementNode || node.Data != "a" {
		log.Fatal("can't extract ISIN: unexpected node: ", node.Type, node.Data)
	}

	for i := range node.Attr {
		if node.Attr[i].Key != "href" {
			continue
		}

		// must be `/q/bonds/RU000A0JU880/'
		href := node.Attr[i].Val
		if !strings.HasPrefix(node.Attr[i].Val, "") {
			log.Fatal("can't extract ISIN: unknown link:", href)
		}

		return href[len("/q/bonds/") : len(href)-1]
	}

	log.Fatal("can't extract ISIN: href not found")
	return ""
}

func extractDate(date string) *time.Time {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil
	}

	return &t
}
