package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/spectrec/invest-tools/pkg/bond"
)

var maturityDate = flag.String("maturity-date", "", "when you want to sale a bond (yyyy-mm-dd)")
var comissionPercent = flag.Float64("comission", 0.1, "average comission percent (broker, exchange, etc)")

var cleanPrice = flag.Float64("clean-price", 1000.0, "buy price of the one bond (not including comission and accrued coupon)")
var cleanPricePercent = flag.Float64("clean-price-percent", 0.0, "buy price of the one bond in percents (not including comission and accrued coupon)")
var nominal = flag.Float64("nominal", 1000.0, "nominal price of the one bond")

var accruedCouponIncome = flag.Float64("accrued-coupon", 0.0, "accrued coupon income")
var couponInterest = flag.Float64("coupon-interest", 0.0, "coupon interest percent (0-100)")

var bondType = flag.String("type", "corp", "bonds type")

var count = flag.Uint("count", 1, "bonds count")

func main() {
	flag.Parse()

	if *maturityDate == "" {
		log.Fatalln("missed `-maturity-date' options")
	}

	b := &bond.Bond{
		Type:    bond.Type(*bondType),
		Nominal: *nominal,

		CouponInterest: *couponInterest,
		AccruedInterst: *accruedCouponIncome,

		CleanPrice:        *cleanPrice,
		CleanPricePercent: *cleanPricePercent,

		MaturityDate: parseDate(*maturityDate),
	}

	b.Init(*comissionPercent)
	fmt.Printf("%vCost total: %.3f\n", b, b.DirtyPrice*float64(*count))
}

func parseDate(date string) *time.Time {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		log.Fatal("can't parse date ", err)
	}

	return &t
}
