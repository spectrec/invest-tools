package rusbonds

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/charmap"

	util "github.com/spectrec/invest-tools/pkg/html"
)

type Bond struct {
	ISIN string

	Redemption string
	Options    string // put, call

	CouponType   string
	CouponFreq   uint32
	CouponPeriod string
}

const searchPattern = `https://www.rusbonds.ru/srch_simple.asp?go=1&nick=%s&emit=0&sec=0&status=&cat=0&per=0&rate=0&ctype=0&pvt=0&grnt=0&conv=0&amm=0&bpog=&epog=&brazm=&erazm=&bvip=&evip=&brep=&erep=&bemis=&eemis=&bstav=&estav=&bcvol=&ecvol=#rslt`

func Search(ISIN string) (*Bond, error) {
	resp, err := http.Get(fmt.Sprintf(searchPattern, ISIN))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	root, err := html.Parse(charmap.Windows1251.NewDecoder().Reader(resp.Body))
	if err != nil {
		return nil, err
	}

	thead := util.ExtractNodeByPath(root, []string{
		"html", "body", "div", "table", "thead",
	})
	if thead == nil {
		// bond was not found (or format has been changed)
		return nil, nil
	}

	var tbody *html.Node
	for tbody = thead.NextSibling; tbody != nil && tbody.Type != html.ElementNode; tbody = tbody.NextSibling {
	}
	if tbody == nil {
		return nil, fmt.Errorf("result table was not found (tbody)")
	}

	href := util.ExtractNodeByPath(tbody, []string{"tr", "td", "a"})
	if href == nil {
		return nil, fmt.Errorf("bad format: can't find <a> tag for next page")
	}

	for _, attr := range href.Attr {
		if attr.Key == "href" {
			return parseBondPage(attr.Val)
		}
	}

	return nil, fmt.Errorf("bad format: can't find `href' attribute within <a> tag for next page")
}

var stripScripts = regexp.MustCompile(`(?s:<!--.+?-->)|(?si:<script.+?</script>)|<[^>]+>`)
var stripEmpty = regexp.MustCompile(`(?s:(?:\s+|&nbsp;))`)

var extractRedemption = regexp.MustCompile(`ПОГАШЕНИЕ\s+-?\s+(\S+)`)
var extractOptions = regexp.MustCompile(`ОФЕРТЫ или ДОСРОЧН.ПОГАШЕНИЕ (.+?) КУПОН`)
var extractCouponType = regexp.MustCompile(`КУПОН\s+-?\s+(\S+)`)
var extractCouponFreq = regexp.MustCompile(`Периодичность выплат в год: (\d+)`)
var extractCouponPeriod = regexp.MustCompile(`Текущий купон [(]всего[)]: (\d+ [(]\d+[)])`)

func parseBondPage(href string) (*Bond, error) {
	resp, err := http.Get(fmt.Sprintf("https://www.rusbonds.ru%s", href))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(charmap.Windows1251.NewDecoder().Reader(resp.Body))
	if err != nil {
		return nil, err
	}

	text := stripEmpty.ReplaceAllLiteralString(stripScripts.ReplaceAllLiteralString(string(data), " "), " ")

	var b Bond
	match := extractRedemption.FindStringSubmatch(text)
	if len(match) > 1 {
		b.Redemption = match[1]
	}

	match = extractOptions.FindStringSubmatch(text)
	if len(match) > 1 {
		b.Options = match[1]
	}

	match = extractCouponType.FindStringSubmatch(text)
	if len(match) > 1 {
		b.CouponType = match[1]
	}

	match = extractCouponFreq.FindStringSubmatch(text)
	if len(match) > 1 {
		n, _ := strconv.ParseUint(match[1], 10, 32)
		b.CouponFreq = uint32(n)
	}

	match = extractCouponPeriod.FindStringSubmatch(text)
	if len(match) > 1 {
		b.CouponPeriod = match[1]
	}

	return &b, nil
}
