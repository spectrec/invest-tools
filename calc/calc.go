package main

import (
	"flag"
	"fmt"
	"os"
)

var initialEquity = flag.Float64("initial-equity", 0.0, "specify initial equity")

var monthlyIncome = flag.Float64("monthly-income", 0.0, "specify expected monthly income (REQUIRED)")
var activeInterest = flag.Float64("active-interest", 8.0, "specify expected invest yield")

var monthlyPassiveIncome = flag.Float64("passive-income", 0.0, "specify required passive income (REQUIRED)")
var passiveInterest = flag.Float64("passive-interest", 5.0, "specify expected passive yield")

func main() {
	flag.Parse()

	if *monthlyIncome == 0.0 {
		fmt.Println("invalid usage: `-monthly-income' was not specified")
		os.Exit(1)
	}
	if *monthlyPassiveIncome == 0.0 {
		fmt.Println("invalid usage: `-passive-income' was not specified")
		os.Exit(1)
	}

	currentEquity := *initialEquity
	for year := 0; ; year++ {
		percents := currentEquity * (*activeInterest) / 100.0
		income := *monthlyIncome * 12

		currentEquity += percents + income

		currentPassiveIncome, yearPassiveIncome := calcMonthlyPassiveIncome(currentEquity)
		fmt.Printf("year %2v: percents: %10.2f, income: %12.2f, current equity: %12.2f, passive income: %12.2f (monthly), %12.2f (year)\n",
			year, percents, income, currentEquity, currentPassiveIncome, yearPassiveIncome)

		if currentPassiveIncome >= *monthlyPassiveIncome {
			break
		}
	}
}

func calcMonthlyPassiveIncome(equity float64) (float64, float64) {
	yearIncome := (equity * (*passiveInterest) / 100)
	return yearIncome / 12, yearIncome
}
