package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spectrec/invest-tools/internal/stock/finam"
	"github.com/spectrec/invest-tools/internal/stock/moex"
	smartlab "github.com/spectrec/invest-tools/internal/stock/smart-lab"
	"github.com/spectrec/invest-tools/pkg/bond"
)

var bondTypesArg = flag.String("types", "gov,mun,corp,euro", "required bond types (corp,gov,mun)")

var comissionPercentArg = flag.Float64("comission", 0.1, "comission percent")
var minCleanPricePercentArg = flag.Float64("min-clean-price-percent", 50.0, "minimum allowed clean percent (skip others)")

var minTransactionsCount = flag.Uint("min-txn-count", 50, "minimum suitable transactions count (filter out non liquid bonds)")

var minRubSuitablePercentArg = flag.Float64("min-rub-yield", 8, "min rubble yield percent")
var minUsdSuitablePercentArg = flag.Float64("min-usd-yield", 4, "min dollar yield percent")
var minEurSuitablePercentArg = flag.Float64("min-eur-yield", 4, "min euro yield percent")

var maturityDateArg = flag.String("maturity-date", "", "max maturity date yyyy-mm-dd (by default: today + 5 years)")
var statisticDateArg = flag.String("stat-date", "", "trade statistic date yyyy-mm-dd (by default: yestarday when `now' is before 6 p.m; otherwise today)")

var outputFileArg = flag.String("output", "output.txt", "path to output file")
var moexResults = flag.String("moex-cache", "", "path to file, downloaded from 'https://www.moex.com/ru/listing/securities-list-csv.aspx?type=1' (needed when moex failed)")
var companyBlacklist = flag.String("blacklist", "", "path to file, contains blacklisted companies (to exclude them from result)")

var debugArg = flag.Bool("debug", false, "enable debug output")

func main() {
	var finamBonds map[string]finam.Bond
	var listing map[string]*moex.Bond
	var bonds []*bond.Bond
	var wg sync.WaitGroup
	var err error

	flag.Parse()

	var excludeCompany = make([]string, 0)
	if *companyBlacklist != "" {
		f, err := os.Open(*companyBlacklist)
		if err != nil {
			log.Fatalf("can't open file `%v': %v", *companyBlacklist, err)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			excludeCompany = append(excludeCompany, scanner.Text())
		}
		if err = scanner.Err(); err != nil {
			log.Fatalf("blacklist scan failed: %v", err)
		}
	}

	var maturityDate time.Time
	if *maturityDateArg != "" {
		maturityDate, err = time.Parse("2006-01-02", *maturityDateArg)
		if err != nil {
			log.Fatal("can't parse maturity date: ", err)
		}
	} else {
		// Skip 5 years from now
		maturityDate = time.Now().AddDate(5, 0, 0)
	}

	var statisticDate time.Time
	if *statisticDateArg != "" {
		statisticDate, err = time.Parse("2006-01-02", *statisticDateArg)
		if err != nil {
			log.Fatal("can't parse statistic date: ", err)
		}
	} else if time.Now().Hour() < 18 {
		// Take previuos date, becase exchange still works
		statisticDate = time.Now().Add(-time.Hour * 24)
	} else {
		// We can take `now' because exchange is closed
		statisticDate = time.Now()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		for _, name := range strings.Split(*bondTypesArg, ",") {
			var err error

			log.Printf("Donwloading `%s' bonds list ...", name)
			bonds, err = smartlab.DownloadAndParse(name, bonds, *debugArg)
			log.Printf("Donwloading `%s' bonds finished ...", name)

			if err != nil {
				log.Fatal("smart-lab failed: ", err)
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		var err error

		log.Println("Donwloading moex listings ...")
		listing, err = moex.DownloadAndParse(*moexResults, *debugArg)
		log.Println("Donwloading moex finished ...")

		if err != nil {
			log.Fatal("moex failed: ", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		var err error

		log.Printf("Donwloading finam bonds list ...")
		finamBonds, err = finam.DownloadAndParse(statisticDate, *debugArg)
		log.Printf("Donwloading finam bonds finished ...")

		if err != nil {
			log.Fatal("finam failed: ", err)
		}
	}()

	wg.Wait()
	log.Println("Merging lists ...")

	var skippedMoex, skippedPrice, notFoundFinam uint32
	for i, b := range bonds {
		v, exist := listing[b.ISIN]
		if !exist {
			skippedMoex++
			bonds[i] = nil

			continue
		}

		b.Currency = v.Currency
		b.CouponInterest = v.CouponInterest
		b.Nominal = v.Nominal
		b.Name = v.Name

		fb, exist := finamBonds[bond.NormalizeBondShortName(b.ShortName)]
		if exist {
			b.SecuritiesCount = fb.SecuritiesCount
			b.TransactionsCount = fb.TransactionsCount
			b.TradeVolume = fb.TradeVolume

			// Use the most pessimistic case
			b.CleanPricePercent = fb.Ask
		} else {
			notFoundFinam++
		}
		if b.TransactionsCount < uint32(*minTransactionsCount) {
			bonds[i] = nil
			continue
		}

		if b.CleanPricePercent < *minCleanPricePercentArg {
			skippedPrice++
			bonds[i] = nil

			continue
		}

		b.Init(*comissionPercentArg)

		var minYieldPercent float64
		switch b.Currency {
		case "Рубль":
			minYieldPercent = *minRubSuitablePercentArg
		case "Доллар США":
			minYieldPercent = *minUsdSuitablePercentArg
		case "Евро":
			minYieldPercent = *minEurSuitablePercentArg
		}

		if maturityDate.Before(*b.MaturityDate) || b.YielToMaturity < minYieldPercent {
			bonds[i] = nil
		}
	}
	log.Printf("Merge stat: moex not found: %v; finam not found: %v; too low clean price: %v",
		skippedMoex, notFoundFinam, skippedPrice)

	log.Println("Sorting results ...")
	sort.Slice(bonds, func(i, j int) bool {
		if bonds[i] == nil && bonds[j] == nil {
			return false // they are equal
		}

		if bonds[i] == nil {
			return false // move them to the end
		}
		if bonds[j] == nil {
			return true
		}

		return bonds[i].YielToMaturity > bonds[j].YielToMaturity
	})

	log.Println("Storing results ...")
	file, err := os.OpenFile(*outputFileArg, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	var blacklisted uint32
	for i, b := range bonds {
		if b == nil {
			break
		}

		var skip bool
		for _, exclude := range excludeCompany {
			if strings.Contains(b.Name, exclude) {
				skip = true
				break
			}
		}
		if skip == true {
			blacklisted++
			continue
		}

		_, err = fmt.Fprintf(file, "%v\n%v\n", i, b)
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("Results stored into `%s', skipped: %v (blacklist)", *outputFileArg, blacklisted)
}
