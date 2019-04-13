package smartlab

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/spectrec/invest-tools/pkg/bond"
	util "github.com/spectrec/invest-tools/pkg/html"
)

const (
	fieldSkip = iota
	fieldName
	fieldMaturityDate
	fieldOfferDate
	fieldAccruedInterest
	fieldCleanPrice
)

type options struct {
	bondType bond.BondType

	couponTax float64

	url        string
	fieldNames []string
	fields     []uint8

	debug bool
}

func getOptions(name string, debug bool) *options {
	t := bond.Type(name)

	switch t {
	case bond.TypeCorp:
		return &options{
			bondType:  t,
			couponTax: 13.0,
			url:       "https://smart-lab.ru/q/bonds/",
			fieldNames: []string{
				"№", "Время", "Имя", "",
				"Размещение", "Погашение", "Лет до", "Доходн", "Год.куп.",
				"Куп.дох.", "Цена", "Объем, млн руб", "Купон, руб",
				"Частота,", "НКД, руб", "Дюр-я, лет", "Дата купона", "Оферта",
			},
			fields: []uint8{
				fieldSkip, fieldSkip, fieldName, fieldSkip,
				fieldSkip, fieldMaturityDate, fieldSkip, fieldSkip, fieldSkip,
				fieldSkip, fieldCleanPrice, fieldSkip, fieldSkip,
				fieldSkip, fieldAccruedInterest, fieldSkip, fieldSkip, fieldOfferDate,
			},
			debug: debug,
		}
	case bond.TypeGov:
		return &options{
			bondType: t,
			url:      "https://smart-lab.ru/q/ofz/",
			fieldNames: []string{
				"№", "Время", "Имя", "",
				"Погашение", "Лет до", "Доходн", "!", "Год.куп.",
				"Куп.дох.", "Цена", "Объем,", "Купон, руб",
				"Частота,", "НКД, руб", "Дюр-я, лет", "Дата купона",
			},
			fields: []uint8{
				fieldSkip, fieldSkip, fieldName, fieldSkip,
				fieldMaturityDate, fieldSkip, fieldSkip, fieldSkip, fieldSkip,
				fieldSkip, fieldCleanPrice, fieldSkip, fieldSkip,
				fieldSkip, fieldAccruedInterest, fieldSkip, fieldSkip,
			},
			debug: debug,
		}
	case bond.TypeMun:
		return &options{
			bondType: t,
			url:      "https://smart-lab.ru/q/subfed/",
			fieldNames: []string{
				"№", "Время", "Имя", "",
				"Погашение", "Лет до", "Доходн", "Год.куп.",
				"Куп.дох.", "Цена", "Объем, млн руб", "Купон, руб",
				"Частота,", "НКД, руб", "Дюр-я, лет", "Дата купона", "Оферта",
			},
			fields: []uint8{
				fieldSkip, fieldSkip, fieldName, fieldSkip,
				fieldMaturityDate, fieldSkip, fieldSkip, fieldSkip,
				fieldSkip, fieldCleanPrice, fieldSkip, fieldSkip,
				fieldSkip, fieldAccruedInterest, fieldSkip, fieldSkip, fieldOfferDate,
			},
			debug: debug,
		}
	case bond.TypeEuro:
		return &options{
			bondType: t,
			url:      "https://smart-lab.ru/q/eurobonds/",
			fieldNames: []string{
				"№", "Время", "Имя", "",
				"Погашение", "Лет до", "Доходн", "Год.куп.дох.",
				"Куп.дох.", "Цена", "Объем, тыс. $", "Купон, $",
				"Частота,", "НКД, $", "Дата купона", "Оферта",
			},
			fields: []uint8{
				fieldSkip, fieldSkip, fieldName, fieldSkip,
				fieldMaturityDate, fieldSkip, fieldSkip, fieldSkip,
				fieldSkip, fieldCleanPrice, fieldSkip, fieldSkip,
				fieldSkip, fieldAccruedInterest, fieldSkip, fieldOfferDate,
			},
			debug: debug,
		}
	default:
		return nil
	}
}

func DownloadAndParse(name string, bonds []*bond.Bond, debug bool) ([]*bond.Bond, error) {
	opt := getOptions(name, debug)
	if opt == nil {
		return bonds, nil
	}

	resp, err := http.Get(opt.url)
	if err != nil {
		return bonds, err
	}
	defer resp.Body.Close()

	root, err := html.Parse(resp.Body)
	if err != nil {
		return bonds, err
	}

	tbody := util.ExtractNodeByPath(root, []string{"html", "body", "div", "div", "table", "tbody"})
	if tbody == nil {
		return bonds, fmt.Errorf("bad html")
	}

	var headerChecked bool
	var found, skipped uint32
	for tr := tbody.FirstChild; tr != nil; tr = tr.NextSibling {
		if tr.Type != html.ElementNode {
			continue
		}

		if !headerChecked {
			if err := checkHeader(tr, opt); err != nil {
				return bonds, err
			}

			headerChecked = true
			continue
		}

		bond, err := parseBond(tr, opt)
		if err != nil {
			return bonds, err
		}

		if bond == nil {
			skipped++
			continue
		}

		bonds = append(bonds, bond)
		found++
	}

	if opt.debug {
		fmt.Printf("`%s' bonds: %v found, %v skipped\n", opt.bondType, found, skipped)
	}

	return bonds, nil
}

func parseBond(root *html.Node, opt *options) (*bond.Bond, error) {
	var err error
	var i int

	bond := &bond.Bond{Type: opt.bondType}
	for c := root.FirstChild; c != nil; c = c.NextSibling {
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

		text := util.ExtractTextNode(c)
		if text == nil || text.Data == "" {
			continue
		}

		switch field {
		case fieldName:
			bond.ShortName = text.Data

			bond.ISIN, err = extractISIN(text.Parent)
			if err != nil {
				return nil, err
			}
		case fieldAccruedInterest:
			bond.AccruedInterst, err = strconv.ParseFloat(text.Data, 64)
			if err != nil {
				return nil, fmt.Errorf("can't parse accrued interest: ", err)
			}
		case fieldCleanPrice:
			bond.CleanPricePercent, err = strconv.ParseFloat(text.Data, 64)
			if err != nil {
				return nil, fmt.Errorf("can't parse clean price: ", err)
			}
		case fieldMaturityDate:
			v, err := time.Parse("2006-01-02", text.Data)
			if err != nil {
				if opt.debug {
					fmt.Printf("skip `%s', maturity date not found\n", bond.ISIN)
				}

				return nil, nil
			}

			bond.MaturityDate = &v
		case fieldOfferDate:
			// It is not interested for now
		default:
			panic("unknown bond field")
		}
	}
	if i != len(opt.fields) {
		if opt.debug {
			fmt.Println("skip partially parsed bond")
		}

		return nil, nil
	}

	return bond, nil
}

func checkHeader(tr *html.Node, opt *options) error {
	checked := 0
	for c := tr.FirstChild; c != nil; c = c.NextSibling {
		if checked == len(opt.fieldNames) {
			// All required fields have been checked
			break
		}

		if c.Type != html.ElementNode {
			continue
		}

		if opt.fieldNames[checked] == "" {
			// just ignore it
		} else if v := util.ExtractTextNode(c); v.Data != opt.fieldNames[checked] {
			return fmt.Errorf("header format changed, expected `%s', got `%s'\n",
				opt.fieldNames[checked], v.Data)
		}

		checked++
	}

	if checked != len(opt.fieldNames) {
		return fmt.Errorf("header format changed, expected %v fields, checked %v fields\n",
			len(opt.fieldNames), checked)
	}

	return nil
}

func extractISIN(node *html.Node) (string, error) {
	if node.Type != html.ElementNode || node.Data != "a" {
		return "", fmt.Errorf("can't extract ISIN: unexpected node: ", node.Type, node.Data)
	}

	for i := range node.Attr {
		if node.Attr[i].Key != "href" {
			continue
		}

		// must be `/q/bonds/RU000A0JU880/'
		href := node.Attr[i].Val
		if !strings.HasPrefix(node.Attr[i].Val, "") {
			return "", fmt.Errorf("can't extract ISIN: unknown link:", href)
		}

		return href[len("/q/bonds/") : len(href)-1], nil
	}

	return "", fmt.Errorf("can't extract ISIN: href not found")
}
