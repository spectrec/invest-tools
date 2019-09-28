package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"time"
)

type price struct {
	value float64
	date  time.Time
}

var argStartDate = flag.String("start-date", "", "specify start date (dd.mm.yyyy)")
var argEndDate = flag.String("end-date", "", "specify end date (dd.mm.yyyy)")

var argInitialSum = flag.Float64("initial-sum", 0, "specify initial invest sum")

var argAdditionalSum = flag.Float64("additional-sum", 0, "specify additional invest sum")
var argAdditionalInterval = flag.Uint("additional-interval", 30, "specify interval before additional investments (days)")

var argETF = flag.Bool("etf", false, "don't allow partial assets")

var argDebug = flag.Bool("debug", false, "enable debug output")

func main() {
	flag.Parse()

	if len(flag.Args()) != 1 {
		log.Fatalf("Usage: %v [-debug] [-start-date dd.mm.yyyy] [-end-date dd.mm.yyyy] [-initial-sum X] [-additional-sum Y] [-additional-interval Z (days)] [-etf true/false] <fund price csv-file (dd.mm.yyyy,price)", os.Args[0])
	}

	rec, err := parseCSV(flag.Arg(0))
	if err != nil {
		log.Fatalf("failed to parse csv file: %v", err)
	}

	var startDate time.Time
	if *argStartDate != "" {
		startDate, err = time.Parse("02.01.2006", *argStartDate)
		if err != nil {
			log.Fatalf("bad `start-date' specified `%v': %v", *argStartDate, err)
		}
	}

	var endDate time.Time
	if *argEndDate != "" {
		endDate, err = time.Parse("02.01.2006", *argEndDate)
		if err != nil {
			log.Fatalf("bad `end-date' specified `%v': %v", *argEndDate, err)
		}
	} else {
		endDate = time.Now()
	}

	if startDate.After(endDate) {
		log.Fatal("`-start-date' (%v) must be lower than `-end-date' (%v)", startDate, endDate)
	}

	// keep only interesting for us records
	var records []price
	for _, r := range rec {
		if r.date.Before(startDate) {
			continue
		}
		if r.date.After(endDate) {
			break
		}

		if len(records) == 0 {
			records = append(records, r)
		} else {
			last := records[len(records)-1]

			nextDate := last.date.AddDate(0, 0, int(*argAdditionalInterval))
			if nextDate.Before(r.date) {
				records = append(records, r)
			}
		}
	}
	if len(records) <= 1 {
		log.Fatal("can't calculate yield, need more records")
	}

	var daysSpent uint
	var cash = *argInitialSum
	var assetsCount, moneySpent float64
	for i, r := range records {
		if *argDebug {
			log.Printf("%v: process price %v, cash: %.2f, spent total: %.2f, assets count: %.1f", r.date, r.value, cash, moneySpent, assetsCount)
		}

		if i != 0 {
			cash += *argAdditionalSum
			daysSpent += *argAdditionalInterval
		}

		// try to buy something
		if *argETF == false || cash > r.value {
			if *argETF {
				// we can buy only whole etf asset
				var n = math.Floor(cash / r.value)
				assetsCount += n

				moneySpent += n * r.value
				cash -= n * r.value
			} else {
				assetsCount += cash / r.value

				moneySpent += cash
				cash = 0
			}
		}
	}

	var lastPrice = records[len(records)-1]
	var returnTotal = assetsCount * lastPrice.value

	var yieldTotalPercent float64
	if moneySpent == 0.0 {
		yieldTotalPercent = 0.0
	} else {
		yieldTotalPercent = (returnTotal - moneySpent) / moneySpent * 100.0
	}

	var yieldTotalPercentNormalized = yieldTotalPercent * 365.0 / float64(daysSpent)

	fmt.Printf("Result stat:\n")
	fmt.Printf(" Days spent:        %v\n", daysSpent)
	fmt.Printf(" Unused cash:       %.2f\n", cash)
	fmt.Printf(" Money spent:       %.2f\n", moneySpent)
	fmt.Printf(" Money return:      %.2f\n", returnTotal)
	fmt.Printf(" Yield absolute:    %.2f%%\n", yieldTotalPercent)
	fmt.Printf(" Yield normalized:  %.2f%%\n", yieldTotalPercentNormalized)
}

type collection []price

func (c collection) Len() int {
	return len(c)
}

func (c collection) Less(i, j int) bool {
	return c[i].date.Before(c[j].date)
}

func (c collection) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

// expected format: `dd.mm.yyyy, price'
func parseCSV(path string) ([]price, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("can't open file `%v': %v", path, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	lines, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse csv file: %v", err)
	}

	records := make([]price, 0, len(lines))
	for i, line := range lines {
		if len(line) != 2 {
			return nil, fmt.Errorf("line `%v': too low parameters (%+v)", i, line)
		}

		date, err := time.Parse("02.01.2006", line[0])
		if err != nil {
			return nil, fmt.Errorf("line `%v': broken date `%v': %v", i, line[0], err)
		}

		v, err := strconv.ParseFloat(line[1], 64)
		if err != nil {
			return nil, fmt.Errorf("line `%v': broken price `%v': %v", i, line[1], err)
		}

		records = append(records, price{value: v, date: date})
	}

	c := collection(records)
	sort.Sort(c)

	return records, nil
}
