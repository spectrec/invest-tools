package main

import (
	"flag"
	"fmt"
	"github.com/spectrec/invest-tools/bond"
	"log"
	"os"
	"sort"
	"strings"
	"time"
)

var bondTypesArg = flag.String("types", "gov,mun,corp", "required bond types (corp,gov,mun)")

var comissionPercentArg = flag.Float64("comission", 0.1, "comission percent")

var minRubSuitablePercentArg = flag.Float64("min-rub-yield", 8, "min rubble yield percent")
var minUsdSuitablePercentArg = flag.Float64("min-usd-yield", 4, "min rubble yield percent")
var minEurSuitablePercentArg = flag.Float64("min-eur-yield", 4, "min rubble yield percent")

var maturityDateArg = flag.String("maturity-date", "", "max maturity date yyyy-mm-dd (by default: today + 3 years)")

var outputFileArg = flag.String("output", "output.txt", "path to output file")

func main() {
	flag.Parse()

	var maturityDate time.Time
	if *maturityDateArg != "" {
		t := extractDate(*maturityDateArg)
		if t == nil {
			log.Fatal("can't parse maturity date")
		}

		maturityDate = *t
	} else {
		// Skip 3 years from now
		maturityDate = time.Now().AddDate(3, 0, 0)
	}

	bonds := make([]*bond.Bond, 0, 0)
	for _, name := range strings.Split(*bondTypesArg, ",") {
		o := parseOptions(bond.Type(name))

		if o.bondType == bond.TypeUndef {
			continue
		}

		log.Printf("Donwloading `%s' bonds list ...\n", name)
		bonds = parseBonds(bonds, o)
		log.Println("Donwload completed")
	}

	log.Println("Donwloading moex listings ...")
	listing := moexDownloadAndParse()

	log.Println("Merging lists ...")
	for i, b := range bonds {
		v, exist := listing[b.ISIN]
		if !exist {
			bonds[i] = nil
			continue
		}

		b.Currency = v.Currency
		b.CouponInterest = v.CouponInterest
		b.Nominal = v.Nominal
		b.Name = v.Name

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

	for _, b := range bonds {
		if b == nil {
			break
		}

		_, err = fmt.Fprintln(file, b)
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("Results stored into `%s'\n", *outputFileArg)
}
