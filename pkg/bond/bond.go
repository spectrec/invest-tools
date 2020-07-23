package bond

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

type Bond struct {
	Type      BondType
	ShortName string
	Name      string
	ISIN      string

	Nominal        float64
	CouponInterest float64
	Currency       string
	CouponType     string
	CouponFreq     uint32
	CouponPeriod   string

	AccruedInterst    float64
	DirtyPrice        float64
	CleanPrice        float64
	CleanPricePercent float64

	MaturityDate   *time.Time
	YielToMaturity float64
	DaysToMaturity uint32

	Options    string
	Redemption string

	SecuritiesCount   uint32
	TransactionsCount uint32
	TradeVolume       float64
}

const (
	CouponTypeFixed            = `Постоянный`
	RedemptionTypeAmortization = `Амортизация`
)

func (b *Bond) String() string {
	fields := []string{
		fmt.Sprintf("Type:               %s", b.Type),
		fmt.Sprintf("ISIN:               %s", b.ISIN),
		fmt.Sprintf("Emitent:            %s (%s)", b.ShortName, b.Name),
		fmt.Sprintf("Nominal:            %.3f", b.Nominal),
		fmt.Sprintf("Coupon:             %.3f%%", b.CouponInterest),
		fmt.Sprintf("CouponType:         %s", b.CouponType),
		fmt.Sprintf("CouponFreq:         %v (per year)", b.CouponFreq),
		fmt.Sprintf("CouponPeriod:       %s", b.CouponPeriod),
		fmt.Sprintf("Currency:           %s", b.Currency),
		fmt.Sprintf("Accurred interest:  %.3f", b.AccruedInterst),
		fmt.Sprintf("Clean price:        %.3f (%.3f%%)", b.CleanPrice, b.CleanPricePercent),
		fmt.Sprintf("Dirty price:        %.3f", b.DirtyPrice),
		fmt.Sprintf("Maturity Date:      %s", b.MaturityDate.Format("2006-01-02")),
		fmt.Sprintf("Days to maturity:   %v", b.DaysToMaturity),
		fmt.Sprintf("Yield to maturity:  %.3f%%", b.YielToMaturity),
	}

	if b.Redemption != "" {
		fields = append(fields, fmt.Sprintf("Redemption:         %s", b.Redemption))
	}

	if b.Options != "" {
		fields = append(fields, fmt.Sprintf("Options:            %s", b.Options))
	}

	var liquid string
	if b.TransactionsCount != 0 {
		liquid = fmt.Sprintf("Liquid:             yes (securities/transactions/volume: %v/%v/%.3f)",
			b.SecuritiesCount, b.TransactionsCount, b.TradeVolume)
	} else {
		liquid = "Liquid:             no"
	}
	fields = append(fields, liquid, "")

	return strings.Join(fields, "\n")

}

func (b *Bond) Init(comissionPercent float64) {
	if b.CleanPricePercent != 0.0 {
		b.CleanPrice = b.Nominal * b.CleanPricePercent / 100.0
	} else {
		b.CleanPricePercent = b.CleanPrice / b.Nominal * 100.0
	}

	b.DirtyPrice = (b.CleanPrice + b.AccruedInterst) * (1 + comissionPercent/100.0)

	b.YielToMaturity = b.calcYield(comissionPercent, *b.MaturityDate)
	b.DaysToMaturity = uint32(b.MaturityDate.Sub(time.Now()).Hours() / 24.0)
}

func (b *Bond) calcYield(comissionPercent float64, maturityDate time.Time) float64 {
	const tax = 1 - 0.13

	spread := 0.0
	if b.Nominal > b.CleanPrice {
		spread = (b.Nominal - b.CleanPrice) * tax
	}

	days := maturityDate.Sub(time.Now()).Hours() / 24
	futureCoupon := b.Nominal * (b.CouponInterest / 100.0) * (days / 365.0) * tax
	accurredInterest := b.AccruedInterst * tax // `futureCoupon' doesn't include it

	income := b.Nominal + spread + accurredInterest + futureCoupon
	spent := b.DirtyPrice

	return (income/spent - 1) * (365.0 / days) * 100.0
}

func NormalizeBondShortName(name string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}

		switch r {
		case '-':
			return -1
		case '/':
			return -1
		case '.':
			return -1
		default:
			return unicode.ToUpper(r)
		}
	}, name)
}
