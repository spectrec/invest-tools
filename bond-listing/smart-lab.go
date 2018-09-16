package main

import (
	"github.com/spectrec/invest-tools/bond"
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
	bondType   bond.BondType
	couponTax  float64
	url        string
	fields     []string
	bondFields []bondField
}

func parseOptions(t bond.BondType) *bondOptions {
	switch t {
	case bond.TypeCorp:
		return &bondOptions{
			bondType:  t,
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
			bondType: t,
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
			bondType: t,
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
	default:
		return &bondOptions{
			bondType: bond.TypeUndef,
		}
	}
}

func parseBonds(bonds []*bond.Bond, opt *bondOptions) []*bond.Bond {
	resp, err := http.Get(opt.url)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	root, err := html.Parse(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	tbody := extractNodeByPath(root, []string{"html", "body", "div", "div", "table", "tbody"})

	var headerChecked bool
	for tr := tbody.FirstChild; tr != nil; tr = tr.NextSibling {
		if tr.Type != html.ElementNode {
			continue
		}

		if !headerChecked {
			checkHeader(tr, opt)
			headerChecked = true

			continue
		}

		if bond := parseBond(tr, opt); bond != nil {
			bonds = append(bonds, bond)
		}
	}

	return bonds
}

func parseBond(root *html.Node, opt *bondOptions) *bond.Bond {
	var err error
	var i int

	bond := &bond.Bond{Type: opt.bondType}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}

		field := opt.bondFields[i]
		i++

		if field == bondFieldSkip {
			continue
		}

		text := extractTextNode(c)
		if text == nil || text.Data == "" {
			continue
		}

		switch field {
		case bondFieldName:
			bond.Name = text.Data
			bond.ISIN = extractISIN(text.Parent)
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

			if bond.CleanPricePercent < 50 {
				// Assume that something wrong with this bond,
				//  and it is not interesting for us
				return nil
			}
		case bondFieldMaturityDate:
			bond.MaturityDate = extractDate(text.Data)
			if bond.MaturityDate == nil {
				return nil
			}
		case bondFieldOfferDate:
			bond.OfferDate = extractDate(text.Data)
		default:
			panic("unknown bond field")
		}
	}
	if i != len(opt.bondFields) {
		log.Println("skip partially parsed bond")
		return nil
	}
	if bond.CleanPricePercent == 0.0 {
		return nil
	}

	return bond
}

func checkHeader(tr *html.Node, opt *bondOptions) {
	checked := 0
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}

		if opt.fields[checked] == "" {
			// just ignore it
		} else if v := extractTextNode(c); v.Data != opt.fields[checked] {
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

func extractTextNode(node *html.Node) *html.Node {
	if node.Type == html.TextNode {
		return node
	}

	if node.FirstChild == nil {
		return nil
	}

	return extractTextNode(node.FirstChild)
}

func extractChildNode(node *html.Node, name string) *html.Node {
	for node = node.FirstChild; node != nil; node = node.NextSibling {
		if node.Type == html.ElementNode && node.Data == name {
			return node
		}
	}

	log.Fatalf("`%s' node not found", name)

	return nil
}

func extractNodeByPath(root *html.Node, path []string) *html.Node {
	for _, name := range path {
		root = extractChildNode(root, name)
	}

	return root
}

func extractISIN(node *html.Node) string {
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
