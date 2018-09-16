package bond

import (
	"fmt"
	"time"
)

type Bond struct {
	Type BondType
	Name string
	ISIN string

	Nominal        float64
	CouponInterest float64
	Currency       string

	AccruedInterst    float64
	DirtyPrice        float64
	CleanPrice        float64
	CleanPricePercent float64

	MaturityDate   *time.Time
	YielToMaturity float64
	DaysToMaturity uint32

	OfferDate   *time.Time
	YielToOffer float64
}

func (b *Bond) String() string {
	return fmt.Sprintf(
		"Type:               %s\n"+
			"Emitent:            %s\n"+
			"ISIN:               %s\n"+
			"Nominal:            %.3f\n"+
			"Coupon:             %.3f%%\n"+
			"Currency:           %s\n"+
			"Accurred interest:  %.3f\n"+
			"Clean price:        %.3f (%.3f%%)\n"+
			"Dirty price:        %.3f\n"+
			"Maturity Date:      %s\n"+
			"Days to maturity:   %v\n"+
			"Yield to maturity:  %.3f%%\n",
		b.Type, b.Name, b.ISIN, b.Nominal, b.CouponInterest, b.Currency,
		b.AccruedInterst, b.CleanPrice, b.CleanPricePercent, b.DirtyPrice,
		b.MaturityDate.Format("2006-01-02"), b.DaysToMaturity, b.YielToMaturity)
}

func (b *Bond) Init(comissionPercent float64) {
	if b.CleanPrice == 0.0 {
		b.CleanPrice = b.Nominal * b.CleanPricePercent / 100.0
	} else if b.CleanPricePercent == 0.0 {
		b.CleanPricePercent = b.CleanPrice / b.Nominal * 100.0
	}

	b.DirtyPrice = (b.CleanPrice + b.AccruedInterst) * (1 + comissionPercent/100.0)

	b.YielToMaturity = b.calcYield(comissionPercent, *b.MaturityDate)
	b.DaysToMaturity = uint32(b.MaturityDate.Sub(time.Now()).Hours() / 24.0)
	if b.OfferDate != nil {
		b.YielToOffer = b.calcYield(comissionPercent, *b.OfferDate)
	}
}

func (b *Bond) calcYield(comissionPercent float64, maturityDate time.Time) float64 {
	days := maturityDate.Sub(time.Now()).Hours() / 24

	coupon := b.Nominal * (b.CouponInterest / 100.0) * (days / 365.0)
	if b.Type == TypeCorp {
		coupon *= (1 - 0.13)
	}

	spread := (b.Nominal - b.CleanPrice)
	if spread >= 0.0 {
		// Take taxes
		spread *= (1 - 0.13)
	}

	return (coupon + spread) / b.DirtyPrice * (365.0 / days) * 100.0
}
