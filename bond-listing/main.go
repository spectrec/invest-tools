package main

import (
	"flag"
	"fmt"
	"github.com/spectrec/invest-tools/bond"
	"github.com/spectrec/invest-tools/bond-listing/finam"
	"github.com/spectrec/invest-tools/bond-listing/moex"
	"github.com/spectrec/invest-tools/bond-listing/smart-lab"
	"log"
	"os"
	"sort"
	"strings"
	"time"
)

var bondTypesArg = flag.String("types", "gov,mun,corp,euro", "required bond types (corp,gov,mun)")

var comissionPercentArg = flag.Float64("comission", 0.1, "comission percent")
var minCleanPricePercentArg = flag.Float64("min-clean-price-percent", 50.0, "minimum allowed clean percent (skip others)")

var minRubSuitablePercentArg = flag.Float64("min-rub-yield", 8, "min rubble yield percent")
var minUsdSuitablePercentArg = flag.Float64("min-usd-yield", 4, "min rubble yield percent")
var minEurSuitablePercentArg = flag.Float64("min-eur-yield", 4, "min rubble yield percent")

var maturityDateArg = flag.String("maturity-date", "", "max maturity date yyyy-mm-dd (by default: today + 3 years)")
var statisticDateArg = flag.String("stat-date", "", "trade statistic date yyyy-mm-dd (by default: yestarday when `now' is before 6 p.m; otherwise today)")

var outputFileArg = flag.String("output", "output.txt", "path to output file")

var debugArg = flag.Bool("debug", false, "enable debug output")

func main() {
	flag.Parse()

	var maturityDate time.Time
	if *maturityDateArg != "" {
		t, err := time.Parse("2006-01-02", *maturityDateArg)
		if err != nil {
			log.Fatal("can't parse maturity date: ", err)
		}

		maturityDate = t
	} else {
		// Skip 3 years from now
		maturityDate = time.Now().AddDate(3, 0, 0)
	}

	var statisticDate time.Time
	if *statisticDateArg != "" {
		t, err := time.Parse("2006-01-02", *statisticDateArg)
		if err != nil {
			log.Fatal("can't parse statistic date: ", err)
		}

		statisticDate = t
	} else if time.Now().Hour() < 18 {
		// Take previuos date, becase exchange still works
		statisticDate = time.Now().Add(-time.Hour * 24)
	} else {
		// We can take `now' because exchange is closed
		statisticDate = time.Now()
	}

	bonds := make([]*bond.Bond, 0, 0)
	for _, name := range strings.Split(*bondTypesArg, ",") {
		o := smartlab.ParseOptions(bond.Type(name))

		if o.BondType == bond.TypeUndef {
			continue
		}

		log.Printf("Donwloading `%s' bonds list ...\n", name)
		bonds = smartlab.ParseBonds(bonds, o)
		log.Println("Donwload completed")
	}

	log.Println("Donwloading moex listings ...")
	listing := moex.DownloadAndParse()

	log.Printf("Donwloading finam bonds list ...\n")
	finamBonds := finam.ParseFinam(statisticDate)

	log.Println("Merging lists ...")

	var skippedMoex, skippedPrice uint32
	for i, b := range bonds {
		v, exist := listing[b.ISIN]
		if !exist {
			skippedMoex++
			bonds[i] = nil

			continue
		}

		if bonds[i].CleanPricePercent < *minCleanPricePercentArg {
			skippedPrice++
			bonds[i] = nil

			continue
		}

		b.Currency = v.Currency
		b.CouponInterest = v.CouponInterest
		b.Nominal = v.Nominal
		b.Name = v.Name

		_, found := finamBonds[bond.NormalizeBondShortName(b.ShortName)]
		if found {
			// TODO: fill info
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
	log.Printf("Merge stat: moex not found: %v, too low clean price: %v\n", skippedMoex, skippedPrice)

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

	for i, b := range bonds {
		if b == nil {
			break
		}

		_, err = fmt.Fprintf(file, "%v\n%v\n", i, b)
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("Results stored into `%s'\n", *outputFileArg)
}
