package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// moex references:
// - http://iss.moex.com/iss/reference/ - api methods
// - https://www.moex.com/a2193 - common api description
// - https://iss.moex.com/iss/engines/stock/markets/bonds/securities/columns.json columns description
// - http://iss.moex.com/iss/securities/TATN/dividends.json?iss.json=extended - dividend history (for future)

var anyCouponTypesArg = flag.Bool("any-coupon-type", false, "show bonds with all coupon types (by default: fixed only)")
var anyRedemptionTypesArg = flag.Bool("any-redemption-type", false, "show bonds with all redemption types (by default: non amortization only)")

var comissionPercentArg = flag.Float64("comission", 0.0, "comission percent")

var minCouponPercentArg = flag.Float64("min-coupon-percent", 1.0, "minimum allowed coupon percent (skip others)")
var minCleanPricePercentArg = flag.Float64("min-clean-price-percent", 90.0, "minimum allowed clean percent (skip others)")

var minRubSuitablePercentArg = flag.Float64("min-rub-yield", 6, "min rubble yield percent")
var minUsdSuitablePercentArg = flag.Float64("min-usd-yield", 4, "min dollar yield percent")
var minEurSuitablePercentArg = flag.Float64("min-eur-yield", 4, "min euro yield percent")

var minMaturityDateArg = flag.String("min-maturity-date", "", "min maturity date yyyy-mm-dd (by default: today + 1 years)")
var maxMaturityDateArg = flag.String("max-maturity-date", "", "max maturity date yyyy-mm-dd (by default: today + 3 years)")

var threadPoolSizeArg = flag.Int("thread-pool-size", 10, "max number of goroutines for checking coupons and amortization")

var emitentCacheArg = flag.String("emitent-cache", "emitent.cache", "path to output file")

var outputFileArg = flag.String("output", "output.txt", "path to output file")

var emitentBlacklist = flag.String("emitent-blacklist", "emitent.blacklist", "path to file, contains blacklisted companies (to exclude them from result)")
var emitentComments = flag.String("emitent-comments", "emitent.comments", "path to file, contains comments for companies")
var securitiesBlacklist = flag.String("securities-blacklist", "securities.blacklist", "path to file, contains blacklisted security names (to exclude them from result)")

type Emitent struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	INN   string `json:"inn"`
}

func downloadEmitents() (map[string]*Emitent, error) {
	var result = make(map[string]*Emitent)

	var offset = 0
	for {
		var columns = []string{"secid", "type", "emitent_title", "emitent_inn"}

		list := strings.Join(columns, ",")
		url := fmt.Sprintf("https://iss.moex.com/iss/securities.json?engine=stock&market=bonds&iss.meta=off&securities.columns=%v&start=%v", list, offset)
		log.Printf("requesting emitents `%v' ...", url)

		resp, err := http.Get(url)
		if err != nil {
			return nil, fmt.Errorf("GET failed: %v", err)
		}

		data, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("body read failed: %v", err)
		}

		var response struct {
			Securities struct {
				Data [][]string `json:"data"`
			} `json:"securities"`
		}
		if err = json.Unmarshal(data, &response); err != nil {
			return nil, fmt.Errorf("decode `%v' failed: %v", string(data), err)
		}

		for _, v := range response.Securities.Data {
			if len(v) != len(columns) {
				log.Fatalf("unknown format `%v'", string(data))
			}

			result[v[0]] = &Emitent{Type: v[1], Title: v[2], INN: v[3]}
		}
		if len(response.Securities.Data) == 0 {
			break
		}

		offset += len(response.Securities.Data)
	}

	return result, nil
}

type Security struct {
	ID        string `json:"secid"`
	ISIN      string `json:"isin"`
	ShortName string `json:"short_name"`
	SecName   string `json:"sec_name"`

	Coupon struct {
		Percent         float64 `json:"percent"`
		Value           float64 `json:"value"`
		Period          float64 `json:"period"`
		AccruedInterest float64 `json:"accrued_interest"`
		NextCouponDate  string  `json:"next_coupon_date"`

		IsConstant bool `json:"is_constant"`
		IsFixed    bool `json:"is_fixed"`
	} `json:"coupon"`

	CleanPricePercent float64 `json:"clean_price_precent"`
	CleanPrice        float64 `json:"clean_price"`
	DirtyPrice        float64 `json:"dirty_price"`
	Currency          string  `json:"currency"`

	Nominal float64 `json:"nominal"`
	Lot     struct {
		Price     float64 `json:"price"`
		BondCount float64 `json:"bond_count"`
	} `json:"lot"`

	MaturityDate   time.Time `json:"maturity_date"`
	DaysToMaturity float64   `json:"days_to_maturity"`
	OfferDate      string    `json:"offer_date"`

	YieldToMaturity float64 `json:"yield_to_maturity"`

	Amortization bool `json:"amortization"`

	Emitent      *Emitent `json:"emitent"`
	Comment      string   `json:"comment"`
	ListingLevel float64  `json:"listing_level"`

	MarketBoard  string `json:"market_board"`
	RusbondsLink string `json:"rusbonds_link"`
}

func (s *Security) String() string {
	data, err := json.MarshalIndent(s, "", "\t")
	if err != nil {
		log.Fatalf("can't encode `%+v' to json: %v", *s, err)
	}

	data = bytes.ReplaceAll(data, []byte("\\"), []byte{})
	data = bytes.ReplaceAll(data, []byte("u0026"), []byte("&"))

	return string(data)
}

func (s *Security) init() {
	const tax = 1 - 0.13

	s.Nominal = s.Lot.Price / s.Lot.BondCount
	s.DaysToMaturity = math.Round(s.MaturityDate.Sub(time.Now()).Hours() / 24)

	s.CleanPrice = s.Nominal * s.CleanPricePercent / 100.0
	s.DirtyPrice = (s.CleanPrice + s.Coupon.AccruedInterest) * (1 + *comissionPercentArg/100.0)

	var spread = 0.0
	if s.Nominal > s.DirtyPrice {
		spread = (s.Nominal - s.DirtyPrice) * tax
	}

	var futureCoupon = s.Nominal * (s.Coupon.Percent / 100.0) * (s.DaysToMaturity / 365.0) * tax
	var accurredInterest = s.Coupon.AccruedInterest * tax // `futureCoupon' doesn't include it

	var income = s.Nominal + spread + accurredInterest + futureCoupon
	var spent = s.DirtyPrice
	s.YieldToMaturity = (income/spent - 1) * (365.0 / s.DaysToMaturity) * 100.0

	s.RusbondsLink = fmt.Sprintf("https://www.rusbonds.ru/srch_simple.asp?go=1&nick=%v", s.ISIN)
}

func downloadSecurities() (map[string]*Security, error) {
	var columns = []string{
		"SECID",
		"ISIN",
		"SHORTNAME",
		"SECNAME",
		"COUPONPERCENT",
		"COUPONVALUE",
		"ACCRUEDINT",
		"NEXTCOUPON",
		"COUPONPERIOD",
		"LOTVALUE",
		"LOTSIZE",
		"OFFERDATE",
		"MATDATE",
		"FACEUNIT",
		"PREVADMITTEDQUOTE",
		"LISTLEVEL",
		"BOARDNAME",
	}

	list := strings.Join(columns, ",")
	url := fmt.Sprintf("https://iss.moex.com/iss/engines/stock/markets/bonds/securities.json?iss.meta=off&iss.only=securities&securities.columns=%v", list)
	log.Printf("downloading securities `%v' ...", url)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET failed: %v", err)
	}

	data, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("body read failed: %v", err)
	}

	var response struct {
		Securities struct {
			Data [][]interface{} `json:"data"`
		} `json:"securities"`
	}
	if err = json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode `%v' failed: %v", string(data), err)
	}

	var result = make(map[string]*Security)
	for _, v := range response.Securities.Data {
		if len(v) != len(columns) {
			log.Fatalf("unknown format `%v'", string(data))
		}

		var secid = v[0].(string)
		var sec = result[secid]
		if sec == nil {
			sec = &Security{ID: secid}
			result[secid] = sec
		}

		sec.ISIN = v[1].(string)
		sec.ShortName = v[2].(string)
		sec.SecName = v[3].(string)

		if v[4] != nil {
			sec.Coupon.Percent = v[4].(float64)
		}
		sec.Coupon.Value = v[5].(float64)
		sec.Coupon.AccruedInterest = v[6].(float64)
		sec.Coupon.NextCouponDate = v[7].(string)
		sec.Coupon.Period = v[8].(float64)

		sec.Lot.Price = v[9].(float64)
		sec.Lot.BondCount = v[10].(float64)

		if v[11] != nil {
			sec.OfferDate = v[11].(string)
		}

		var date = v[12].(string)
		if date == "0000-00-00" {
			// it will be excluded by maturity date
			date = "3999-01-01"
		}
		sec.MaturityDate, err = time.Parse("2006-01-02", date)
		if err != nil {
			return nil, fmt.Errorf("can't decode maturity date `%v': %v", date, err)
		}

		sec.Currency = v[13].(string)
		if v[14] != nil {
			sec.CleanPricePercent = v[14].(float64)
		}

		sec.ListingLevel = v[15].(float64)
		if v[14] != nil {
			// override marken only when price is available to make it consistent
			sec.MarketBoard = v[16].(string)
		}
	}

	return result, nil
}

func (s *Security) downloadBondization() error {
	var amortizationColumns = []string{"amortdate", "valueprc", "value"}
	var couponsColumns = []string{"coupondate", "valueprc", "value"}
	var offerColumns = []string{"offerdate", "offerdatestart", "offerdateend", "offertype"}

	url := fmt.Sprintf("http://iss.moex.com/iss/securities/%v/bondization.json?limit=unlimited&iss.meta=off&amortizations.columns=%v&coupons.columns=%v&offers.columns=%v",
		s.ID, strings.Join(amortizationColumns, ","), strings.Join(couponsColumns, ","), strings.Join(offerColumns, ","))
	log.Printf("downloading bondization `%v' ...", url)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("GET failed: %v", err)
	}

	data, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return fmt.Errorf("body read failed: %v", err)
	}

	var response struct {
		Amortizations struct {
			Data [][]interface{} `json:"data"`
		} `json:"amortizations"`
		Coupons struct {
			Data [][]interface{} `json:"data"`
		} `json:"coupons"`
		Offers struct {
			Data [][]interface{} `json:"data"`
		} `json:"offers"`
	}
	if err = json.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("decode `%v' failed: %v", string(data), err)
	}

	if len(response.Amortizations.Data) > 1 {
		s.Amortization = true
	}

	s.Coupon.IsFixed = true
	s.Coupon.IsConstant = true
	for i := 0; i < len(response.Coupons.Data); i++ {
		if response.Coupons.Data[i][1] == nil {
			s.Coupon.IsConstant = false
			s.Coupon.IsFixed = false
			break
		}

		if i > 0 && response.Coupons.Data[i-1][1] != response.Coupons.Data[i][1] {
			s.Coupon.IsConstant = false
		}
	}

	return nil
}

func main() {
	var wg sync.WaitGroup
	var err error

	flag.Parse()

	var excludeEmitent = make([]string, 0)
	if *emitentBlacklist != "" {
		f, err := os.Open(*emitentBlacklist)
		if err != nil {
			log.Fatalf("can't open file `%v': %v", *emitentBlacklist, err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) == 0 || strings.HasPrefix(line, "#") {
				continue
			}

			excludeEmitent = append(excludeEmitent, line)
		}
		if err = scanner.Err(); err != nil {
			log.Fatalf("emitent blacklist scan failed: %v", err)
		}
	}

	var emitentComment = make(map[string]string)
	if *emitentComments != "" {
		f, err := os.Open(*emitentComments)
		if err != nil {
			log.Fatalf("can't open file `%v': %v", *emitentComments, err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) == 0 || strings.HasPrefix(line, "#") {
				continue
			}

			parts := strings.Split(line, " -> ")
			if len(parts) != 2 {
				log.Fatalf("bad comment format `%v' (expected: `emitent' -> `comment'", line)
			}

			emitentComment[parts[0]] = parts[1]
		}
		if err = scanner.Err(); err != nil {
			log.Fatalf("emitent comments scan failed: %v", err)
		}
	}

	var excludeSecurities = make([]string, 0)
	if *securitiesBlacklist != "" {
		f, err := os.Open(*securitiesBlacklist)
		if err != nil {
			log.Fatalf("can't open file `%v': %v", *securitiesBlacklist, err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) == 0 || strings.HasPrefix(line, "#") {
				continue
			}

			excludeSecurities = append(excludeSecurities, line)
		}
		if err = scanner.Err(); err != nil {
			log.Fatalf("securities blacklist scan failed: %v", err)
		}
	}

	var minMaturityDate = time.Now().AddDate(1, 0, 0) // skip 1 years from now
	if *minMaturityDateArg != "" {
		date, err := time.Parse("2006-01-02", *minMaturityDateArg)
		if err != nil {
			log.Fatal("can't parse maturity date: ", err)
		}

		minMaturityDate = date
	}

	var maxMaturityDate = time.Now().AddDate(3, 0, 0) // skip 3 years from now
	if *maxMaturityDateArg != "" {
		date, err := time.Parse("2006-01-02", *maxMaturityDateArg)
		if err != nil {
			log.Fatal("can't parse max maturity date: ", err)
		}

		maxMaturityDate = date
	}

	wg.Add(2)

	var secid2emitent map[string]*Emitent
	go func() {
		defer wg.Done()

		if *emitentCacheArg != "" {
			data, err := ioutil.ReadFile(*emitentCacheArg)
			if err == nil {
				if err = json.Unmarshal(data, &secid2emitent); err == nil {
					return
				}

				log.Printf("can't decode emitents cache `%v' (will be requested again): %v", *emitentCacheArg, err)
			} else if !os.IsNotExist(err) {
				log.Fatalf("can't read emitents cache `%v': %v", *emitentCacheArg, err)
			}
		}

		secid2emitent, err = downloadEmitents()
		if err != nil {
			log.Fatalf("can't download emitents: %v", err)
		}

		if *emitentCacheArg != "" {
			data, err := json.Marshal(&secid2emitent)
			if err != nil {
				log.Fatalf("can't encode emitents cache: %v", err)
			}
			if err = ioutil.WriteFile(*emitentCacheArg, data, 0644); err != nil {
				log.Fatalf("can't store emitents cache into `%v': %v", *emitentCacheArg, err)
			}
		}
	}()

	var securities map[string]*Security
	go func() {
		defer wg.Done()

		securities, err = downloadSecurities()
		if err != nil {
			log.Fatalf("can't download securities: %v", err)
		}
	}()

	wg.Wait()

	var blacklisted, skipLowPrice, skipLowCouponPercent, skipLowYield, skipMaturityDate, skipCouponType, skipAmortization int
	for secid, v := range securities {
		var skip bool

		v.init()

		if e := secid2emitent[secid]; e != nil {
			v.Emitent = e

			for _, exclude := range excludeEmitent {
				if strings.Contains(v.Emitent.Title, exclude) {
					skip = true
					break
				}
			}

			v.Comment = emitentComment[e.Title]
		} else {
			log.Printf("emitent for `%v' not found", secid)
		}

		if skip == false {
			for _, exclude := range excludeSecurities {
				if strings.Contains(v.ISIN, exclude) || strings.Contains(v.ShortName, exclude) || strings.Contains(v.SecName, exclude) {
					skip = true
					break
				}
			}
		}
		if skip == true {
			delete(securities, secid)
			blacklisted++

			continue
		}

		if v.CleanPricePercent < *minCleanPricePercentArg {
			delete(securities, secid)
			skipLowPrice++

			continue
		}
		if v.Coupon.Percent < *minCouponPercentArg {
			delete(securities, secid)
			skipLowCouponPercent++

			continue
		}

		if minMaturityDate.After(v.MaturityDate) || maxMaturityDate.Before(v.MaturityDate) {
			skipMaturityDate++
			delete(securities, secid)

			continue
		}
	}

	var ch = make(chan *Security, *threadPoolSizeArg)
	wg.Add(*threadPoolSizeArg)
	for i := 0; i < *threadPoolSizeArg; i++ {
		go func() {
			defer wg.Done()

			for {
				var sec = <-ch
				if sec == nil {
					return
				}

				if err := sec.downloadBondization(); err != nil {
					log.Printf("can't donwnload coupon/amortization/offers info for `%v': %v", sec, err)
				}
			}
		}()
	}
	for _, v := range securities {
		ch <- v
	}
	close(ch)

	wg.Wait()

	var bonds []*Security
	for _, v := range securities {
		if v.Coupon.IsFixed == false && *anyCouponTypesArg == false {
			skipCouponType++
			continue
		}

		if v.Amortization && *anyRedemptionTypesArg == false {
			skipAmortization++
			continue
		}

		// skip only contants coupons because of low yield, because yield for other bond types could be incorrect
		var minYieldPercent float64
		switch v.Currency {
		case "SUR":
			minYieldPercent = *minRubSuitablePercentArg
		case "USD":
			minYieldPercent = *minUsdSuitablePercentArg
		case "EUR":
			minYieldPercent = *minEurSuitablePercentArg
		}
		if v.Coupon.IsFixed && v.YieldToMaturity < minYieldPercent {
			skipLowYield++
			continue
		}

		bonds = append(bonds, v)
	}

	log.Printf("\nskip stat:\n")
	log.Printf("\tblacklisted: %v\n", blacklisted)
	log.Printf("\tlow price: %v\n", skipLowPrice)
	log.Printf("\tlow coupon: %v\n", skipLowCouponPercent)
	log.Printf("\tlow yield: %v\n", skipLowYield)
	log.Printf("\tclose/far maturity date: %v\n", skipMaturityDate)
	log.Printf("\tnon fixed coupon: %v\n", skipCouponType)
	log.Printf("\tamortization: %v\n\n", skipAmortization)

	log.Printf("Sorting `%v' results ...", len(bonds))
	sort.Slice(bonds, func(i, j int) bool {
		return bonds[i].YieldToMaturity > bonds[j].YieldToMaturity
	})

	log.Println("Storing results ...")
	file, err := os.OpenFile(*outputFileArg, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	for i, b := range bonds {
		_, err = fmt.Fprintf(file, "%v: %v\n\n", i, b)
		if err != nil {
			log.Fatal("can't store results into `%v'", *outputFileArg, err)
		}
	}

	log.Printf("Results stored into `%s'", *outputFileArg)
}
