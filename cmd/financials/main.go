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

func convert(data []financials) {
	for i := range data {
		const balanceNormalizer = 1e6

		data[i].Exchange.CapitalizationPriv = data[i].Exchange.CapitalizationPriv / balanceNormalizer
		data[i].Exchange.CapitalizationRegular = data[i].Exchange.CapitalizationRegular / balanceNormalizer

		data[i].Assets = data[i].Assets * data[i].Multiplier / balanceNormalizer
		data[i].CashAndEquivalents = data[i].CashAndEquivalents * data[i].Multiplier / balanceNormalizer
		data[i].Liabilities = data[i].Liabilities * data[i].Multiplier / balanceNormalizer
		data[i].LongTermDebt = data[i].LongTermDebt * data[i].Multiplier / balanceNormalizer
		data[i].ShortTermDebt = data[i].ShortTermDebt * data[i].Multiplier / balanceNormalizer
		data[i].Equity = data[i].Equity * data[i].Multiplier / balanceNormalizer

		data[i].Revenue = data[i].Revenue * data[i].Multiplier / balanceNormalizer
		data[i].OperatingIncome = data[i].OperatingIncome * data[i].Multiplier / balanceNormalizer
		data[i].InterestExpences = data[i].InterestExpences * data[i].Multiplier / balanceNormalizer
		data[i].NetIncome = data[i].NetIncome * data[i].Multiplier / balanceNormalizer

		data[i].Dividents = data[i].Dividents * data[i].Multiplier / balanceNormalizer
	}
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

	convert(financials)

	analyze(os.Stdout, financials, conf)
}

type printOptions struct {
	rowNameFmt string

	dataFmt              string
	dataChangePercentFmt string

	showDataChangePercent bool
}

func printRow(w io.Writer, rowName string, opts printOptions, n int, getValue func(i int) float64) {
	fmt.Fprintf(w, opts.rowNameFmt, rowName)

	for i := 0; i < n; i++ {
		v := getValue(i)

		fmt.Fprintf(w, opts.dataFmt, v)
		if opts.showDataChangePercent {
			if i == 0 {
				fmt.Fprintf(w, opts.dataChangePercentFmt, 0.0)
			} else {
				prev := getValue(i - 1)
				fmt.Fprintf(w, opts.dataChangePercentFmt, (v-prev)/prev*100.0)
			}
		}
	}
	fmt.Fprintf(w, "\n")
}

func calcROE(data []financials) (roe []float64, avg float64) {
	for i := range data {
		var v float64

		if i != 0 {
			v = data[i].NetIncome / data[i-1].Equity * 100.0
			avg += v
		}

		roe = append(roe, v)
	}
	avg /= float64(len(data) - 1)

	return roe, avg
}

func analyze(w io.Writer, data []financials, conf config) {
	if len(data) == 0 {
		return
	}

	const columnNameFmt = "%- 40s"
	const columnDataFmt = "% 15.1f"
	const columnDataChangePercent = " (%+ 4.0f%%)"

	opts := printOptions{rowNameFmt: columnNameFmt, dataFmt: "% 21.0f"}
	printRow(w, "Год", opts, len(data), func(i int) float64 { return float64(data[i].Year) })
	fmt.Fprintf(w, "\n")

	opts = printOptions{
		rowNameFmt:            columnNameFmt,
		dataFmt:               columnDataFmt,
		dataChangePercentFmt:  columnDataChangePercent,
		showDataChangePercent: true,
	}
	printRow(w, "Активы (млн)", opts, len(data), func(i int) float64 { return data[i].Assets })
	printRow(w, "Обязательства (млн)", opts, len(data), func(i int) float64 { return data[i].Liabilities })
	printRow(w, "Собственный капитал (млн)", opts, len(data), func(i int) float64 { return data[i].Equity })
	printRow(w, "Собственный капитал per share", opts, len(data), func(i int) float64 {
		return data[i].Equity * 1e6 / float64(data[i].Exchange.StockCountPriv+data[i].Exchange.StockCountRegular)
	})
	fmt.Fprintf(w, "\n")

	printRow(w, "Выручка (млн)", opts, len(data), func(i int) float64 { return data[i].Revenue })
	printRow(w, "Операционная прибыль (млн)", opts, len(data), func(i int) float64 { return data[i].OperatingIncome })
	printRow(w, "Чистая прибыль (млн)", opts, len(data), func(i int) float64 { return data[i].NetIncome })
	fmt.Fprintf(w, "\n")

	printRow(w, "Выручка per share", opts, len(data), func(i int) float64 {
		return data[i].Revenue * 1e6 / float64(data[i].Exchange.StockCountPriv+data[i].Exchange.StockCountRegular)
	})
	printRow(w, "Операционная прибыль per share", opts, len(data), func(i int) float64 {
		return data[i].OperatingIncome * 1e6 / float64(data[i].Exchange.StockCountPriv+data[i].Exchange.StockCountRegular)
	})
	printRow(w, "Чистая прибыль per share", opts, len(data), func(i int) float64 {
		return data[i].NetIncome * 1e6 / float64(data[i].Exchange.StockCountPriv+data[i].Exchange.StockCountRegular)
	})
	fmt.Fprintf(w, "\n")

	printRow(w, "Выплаченные дивиденды (млн)", opts, len(data), func(i int) float64 { return data[i].Dividents })
	printRow(w, "Дивиденды per share", opts, len(data), func(i int) float64 {
		return data[i].Dividents * 1e6 / (data[i].Exchange.StockCountPriv + data[i].Exchange.StockCountRegular)
	})

	opts.dataFmt = "% 21.2f"
	opts.showDataChangePercent = false

	printRow(w, "Дивидендная доходность (ап)", opts, len(data), func(i int) float64 {
		count := data[i].Exchange.StockCountPriv
		if count == 0.0 {
			return 0.0
		}

		price := data[i].Exchange.CapitalizationPriv / count
		dividendsPerShare := data[i].Dividents / (data[i].Exchange.StockCountPriv + data[i].Exchange.StockCountRegular)

		return dividendsPerShare / price * 100
	})
	printRow(w, "Дивидендная доходность (аo)", opts, len(data), func(i int) float64 {
		count := data[i].Exchange.StockCountRegular
		if count == 0.0 {
			return 0.0
		}

		price := data[i].Exchange.CapitalizationRegular / count
		dividendsPerShare := data[i].Dividents / (data[i].Exchange.StockCountPriv + data[i].Exchange.StockCountRegular)

		return dividendsPerShare / price * 100
	})

	var tooHighPayoutRatio = false
	printRow(w, "Payout ratio", opts, len(data), func(i int) float64 {
		payoutRatio := data[i].Dividents / data[i].NetIncome * 100.0
		if payoutRatio > 100.0 {
			tooHighPayoutRatio = true
		}
		return payoutRatio
	})
	fmt.Fprintf(w, "\n")

	printRow(w, "Закредитованность (Обязательства/Активы)", opts, len(data), func(i int) float64 { return data[i].Liabilities / data[i].Assets })

	roe, roe_avg := calcROE(data)
	printRow(w, "ROE", opts, len(data), func(i int) float64 { return roe[i] })
	printRow(w, "ROS", opts, len(data), func(i int) float64 { return data[i].NetIncome / data[i].Revenue })
	printRow(w, "Выручка/Активы", opts, len(data), func(i int) float64 {
		if i == 0 {
			return 0.0
		}

		return data[i].Revenue / data[i].Assets
	})

	printRow(w, "E/P", opts, len(data), func(i int) float64 {
		capitalization := data[i].Exchange.CapitalizationPriv + data[i].Exchange.CapitalizationRegular
		return data[i].NetIncome / capitalization * 100.0
	})
	fmt.Fprintf(w, "\n")

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
	fmt.Fprintf(w, "\t\tАктивы: %.1f млн (required min: %v)\n", lastYear.Assets, conf.MinAssets)
	fmt.Fprintf(w, "\t\tВыручка: %.1f млн (required min: %v)\n", lastYear.Revenue, conf.MinRevenue)

	fmt.Fprintf(w, "\tУстойчивость компании:\n")
	fmt.Fprintf(w, "\t\tЗакредитованность: %.2f, bank: %v (required max: %v)\n",
		lastYear.Liabilities/lastYear.Assets, lastYear.IsBank, conf.MaxLiabilitiesToAssets)
	fmt.Fprintf(w, "\t\tДолг/Капитал: %.2f, bank: %v (required max: %v)\n",
		(lastYear.ShortTermDebt+lastYear.LongTermDebt)/lastYear.Equity, lastYear.IsBank, conf.MaxDebtToEquity)
	fmt.Fprintf(w, "\t\tОперационная прибыль/Проценты к уплате: %.2f, bank: %v (required min: %v)\n",
		lastYear.OperatingIncome/lastYear.InterestExpences, lastYear.IsBank, conf.MinOperatingIncomeToInterestExpences)

	fmt.Fprintf(w, "\tЭффективность и стоимость:\n")
	{
		fmt.Fprintf(w, "\t\tROE (avg): %.2f (required min: %v)\n", roe_avg, conf.MinROE)

		capitalization := lastYear.Exchange.CapitalizationPriv + lastYear.Exchange.CapitalizationRegular
		fmt.Fprintf(w, "\t\tТекущая доходность (E/P): %.2f%%\n", lastYear.NetIncome/capitalization*100)

		// equity*roe == expected net income
		expectedCapitalization := lastYear.Equity * roe_avg / conf.ExpectedReturn
		k := expectedCapitalization / capitalization * 1e6
		fmt.Fprintf(w, "\t\tОценка по Арсагере (r=%.2f): ao=%.2f ап=%.2f\n", conf.ExpectedReturn,
			lastYear.Exchange.CapitalizationRegular/lastYear.Exchange.StockCountRegular*k,
			lastYear.Exchange.CapitalizationPriv/lastYear.Exchange.StockCountPriv*k)
	}
}
