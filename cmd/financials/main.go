package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

type config struct {
	MinAssets  float64 `yaml:"min_assets"`
	MinRevenue float64 `yaml:"min_revenue"`

	MaxLiabilitiesToAssets               float64 `yaml:"max_liabilities_to_assets"`
	MaxDebtToEquity                      float64 `yaml:"max_debt_to_equity"`
	MinOperatingIncomeToInterestExpences float64 `yaml:"min_operating_income_to_interest_expences"`

	MinROE float64 `yaml:"min_roe"`

	ExpectedReturn float64 `yaml:"expected_return"`
}

type exchangeData struct {
	CapitalizationRegular float64 `yaml:"capitalization_regular"`
	StockCountRegular     float64 `yaml:"stock_count_regular"`

	CapitalizationPriv float64 `yaml:"capitalization_priv"`
	StockCountPriv     float64 `yaml:"stock_count_priv"`
}

type financials struct {
	Year int `yaml:"year"`

	Multiplier float64 `yaml:"multiplier"`

	Assets             float64 `yaml:"assets"`
	CashAndEquivalents float64 `yaml:"cash_and_equivalents"`
	Liabilities        float64 `yaml:"liabilities"`
	LongTermDebt       float64 `yaml:"long_term_debt"`
	ShortTermDebt      float64 `yaml:"short_term_debt"`
	Equity             float64 `yaml:"equity"`

	Revenue          float64 `yaml:"revenue"`
	OperatingIncome  float64 `yaml:"operating_income"`
	InterestExpences float64 `yaml:"interest_expences"`
	NetIncome        float64 `yaml:"net_income"`

	Dividents float64 `yaml:"dividends"`

	Exchange exchangeData `yaml:"exchange"`

	IsBank bool `yaml:"is_bank"`

	Comments string `yaml:"comments"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <config> <financials>\n", os.Args[0])
		os.Exit(1)
	}

	data, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't open config `%v': %v\n", os.Args[1], err)
		os.Exit(1)
	}

	var conf = config{}
	if err = yaml.Unmarshal(data, &conf); err != nil {
		fmt.Fprintf(os.Stderr, "can't parse config yaml `%v': %v\n", os.Args[1], err)
		os.Exit(1)
	}

	data, err = ioutil.ReadFile(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "can't open financials data `%v': %v\n", os.Args[2], err)
		os.Exit(1)
	}

	var financials = []financials{}
	if err = yaml.Unmarshal(data, &financials); err != nil {
		fmt.Fprintf(os.Stderr, "can't parse financials yaml `%v': %v\n", os.Args[2], err)
		os.Exit(1)
	}

	analyze(os.Stdout, financials, conf)
}

func analyze(w io.Writer, data []financials, conf config) {
	const columnNameFmt = "%- 40s"
	const columnDataFmt = "% 15.2f"

	if len(data) == 0 {
		return
	}

	fmt.Fprintf(w, columnNameFmt, "Год")
	for _, r := range data {
		fmt.Fprintf(w, "% 15d", r.Year)
	}
	fmt.Fprintf(w, "\n\n")

	const balanceNormalizer = 1e9
	fmt.Fprintf(w, columnNameFmt, "Активы (млрд)")
	for _, r := range data {
		fmt.Fprintf(w, columnDataFmt, r.Assets*r.Multiplier/balanceNormalizer)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, columnNameFmt, "Обязательства (млрд)")
	for _, r := range data {
		fmt.Fprintf(w, columnDataFmt, r.Liabilities*r.Multiplier/balanceNormalizer)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, columnNameFmt, "Собственный капитал (млрд)")
	for _, r := range data {
		fmt.Fprintf(w, columnDataFmt, r.Equity*r.Multiplier/balanceNormalizer)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, columnNameFmt, "Капитализация (млрд)")
	for _, r := range data {
		fmt.Fprintf(w, columnDataFmt, (r.Exchange.CapitalizationPriv+r.Exchange.CapitalizationRegular)/balanceNormalizer)
	}
	fmt.Fprintf(w, "\n\n")

	const resultsNormalizer = 1e6
	fmt.Fprintf(w, columnNameFmt, "Выручка (млн)")
	for _, r := range data {
		fmt.Fprintf(w, columnDataFmt, r.Revenue*r.Multiplier/resultsNormalizer)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, columnNameFmt, "Операционная прибыль (млн)")
	for _, r := range data {
		fmt.Fprintf(w, columnDataFmt, r.OperatingIncome*r.Multiplier/resultsNormalizer)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, columnNameFmt, "Процентные расходы (млн)")
	for _, r := range data {
		fmt.Fprintf(w, columnDataFmt, r.InterestExpences*r.Multiplier/resultsNormalizer)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, columnNameFmt, "Чистая прибыль (млн)")
	for _, r := range data {
		fmt.Fprintf(w, columnDataFmt, r.NetIncome*r.Multiplier/resultsNormalizer)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, columnNameFmt, "Выплаченные дивиденды (млн)")
	for _, r := range data {
		fmt.Fprintf(w, columnDataFmt, r.Dividents*r.Multiplier/resultsNormalizer)
	}
	fmt.Fprintf(w, "\n\n")

	fmt.Fprintf(w, columnNameFmt, "Закредитованность (Обязательства/Активы)")
	for _, r := range data {
		fmt.Fprintf(w, columnDataFmt, r.Liabilities/r.Assets)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, columnNameFmt, "ROE")
	var roe_avg float64
	for i := range data {
		if i == 0 {
			fmt.Fprintf(w, columnDataFmt, 0.0)
			continue
		}

		roe := data[i].NetIncome / data[i-1].Equity * 100.0
		roe_avg += roe

		fmt.Fprintf(w, columnDataFmt, roe)
	}
	roe_avg /= float64(len(data) - 1)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, columnNameFmt, "ROS")
	for _, r := range data {
		fmt.Fprintf(w, columnDataFmt, r.NetIncome/r.Revenue*100.0)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, columnNameFmt, "Выручка/Активы")
	for i := range data {
		if i == 0 {
			fmt.Fprintf(w, columnDataFmt, 0.0)
			continue
		}

		fmt.Fprintf(w, columnDataFmt, data[i].Revenue/data[i-1].Assets)
	}
	fmt.Fprintf(w, "\n")

	var tooHighPayoutRatio = false
	fmt.Fprintf(w, columnNameFmt, "Payout ratio")
	for _, r := range data {
		payoutRatio := r.Dividents / r.NetIncome * 100.0
		fmt.Fprintf(w, columnDataFmt, payoutRatio)

		if payoutRatio > 100.0 {
			tooHighPayoutRatio = true
		}
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, columnNameFmt, "E/P")
	for _, r := range data {
		capitalization := r.Exchange.CapitalizationPriv + r.Exchange.CapitalizationRegular
		fmt.Fprintf(w, columnDataFmt, r.NetIncome*r.Multiplier/capitalization*100.0)
	}
	fmt.Fprintf(w, "\n\n")

	for _, r := range data {
		if len(r.Comments) != 0 {
			fmt.Fprintf(w, columnNameFmt+"%s\n", fmt.Sprintf("Комментарий к %v", r.Year), r.Comments)
		}
	}
	fmt.Fprintf(w, "\n")

	// summary
	fmt.Fprintf(w, "%s\n", "Заключение:")

	var lastYear = data[len(data)-1]
	fmt.Fprintf(w, "\tРазмер компании:\n")
	{
		if lastYear.Assets*lastYear.Multiplier >= conf.MinAssets {
			fmt.Fprintf(w, "\t\tАктивы: ок\n")
		} else {
			fmt.Fprintf(w, "\t\tАктивы: не ок\n")
		}

		if lastYear.Revenue*lastYear.Multiplier >= conf.MinRevenue {
			fmt.Fprintf(w, "\t\tВыручка: ок\n")
		} else {
			fmt.Fprintf(w, "\t\tВыручка: не ок\n")
		}
	}

	fmt.Fprintf(w, "\tУстойчивость компании:\n")
	{
		var liabilitiesToAssets = lastYear.Liabilities / lastYear.Assets
		if liabilitiesToAssets <= conf.MaxLiabilitiesToAssets {
			fmt.Fprintf(w, "\t\tЗакредитованность: ок (%.2f)\n", liabilitiesToAssets)
		} else {
			if lastYear.IsBank {
				fmt.Fprintf(w, "\t\tЗакредитованность: не ок (%.2f), но это банк, поэтому можно пропустить\n", liabilitiesToAssets)
			} else {
				fmt.Fprintf(w, "\t\tЗакредитованность: не ок (%.2f)\n", liabilitiesToAssets)
			}
		}

		var debtToEquity = (lastYear.ShortTermDebt + lastYear.LongTermDebt) / lastYear.Equity
		if debtToEquity > conf.MaxDebtToEquity {
			fmt.Fprintf(w, "\t\tДолг/Капитал: не ок (%.2f)\n", debtToEquity)
		} else {
			fmt.Fprintf(w, "\t\tДолг/Капитал: ок (%.2f)\n", debtToEquity)
		}

		var opResToInterestEx = lastYear.OperatingIncome / lastYear.InterestExpences
		if opResToInterestEx < conf.MinOperatingIncomeToInterestExpences {
			fmt.Fprintf(w, "\t\tОперационная прибыль/Проценты к уплате: не ок (%.2f)\n", opResToInterestEx)
		} else {
			fmt.Fprintf(w, "\t\tОперационная прибыль/Проценты к уплате: ок (%.2f)\n", opResToInterestEx)
		}

		if tooHighPayoutRatio {
			fmt.Fprintf(w, "\t\tPayout ratio > 100%: не ок\n")
		}
	}

	fmt.Fprintf(w, "\tЭффективность и стоимость:\n")
	{
		if roe_avg >= conf.MinROE {
			fmt.Fprintf(w, "\t\tROE (avg): ок (%.2f)\n", roe_avg)
		} else {
			fmt.Fprintf(w, "\t\tROE (avg): не ок (%.2f)\n", roe_avg)
		}

		capitalization := lastYear.Exchange.CapitalizationPriv + lastYear.Exchange.CapitalizationRegular
		fmt.Fprintf(w, "\t\tТекущая доходность (E/P): %.2f%%\n", lastYear.NetIncome*lastYear.Multiplier/capitalization*100)

		// equity*roe == expected net income
		expectedCapitalization := lastYear.Equity * lastYear.Multiplier * roe_avg / conf.ExpectedReturn
		k := expectedCapitalization / capitalization
		fmt.Fprintf(w, "\t\tОценка по Арсагере (r=%.2f): ao=%.2f ап=%.2f\n", conf.ExpectedReturn,
			lastYear.Exchange.CapitalizationRegular/lastYear.Exchange.StockCountRegular*k,
			lastYear.Exchange.CapitalizationPriv/lastYear.Exchange.StockCountPriv*k)
	}
}
