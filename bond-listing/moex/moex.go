package moex

import (
	"encoding/csv"
	"golang.org/x/text/encoding/charmap"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

const (
	moexCsvDatestamp uint32 = iota
	moexCsvInstrumentID
	moexCsvListSection
	moexCsvRN
	moexCsvSupertype
	moexCsvInstrumentType
	moexCsvInstrumentCategory
	moexCsvTradeCode
	moexCsvISIN
	moexCsvRegistryNumber
	moexCsvRegistryDate
	moexCsvEmitentFullName
	moexCsvInn
	moexCsvNominal
	moexCsvCurrency
	moexCsvIssueAmount
	moexCsvDecisionDate
	moexCsvOksmEdr
	moexCsvOnlyEmitentFullName
	moexCsvRegCountry
	moexCsvQualifiedInvestor
	moexCsvHasProspectus
	moexCsvIsConcessionAgreement
	moexCsvIsMortgageAgent
	moexCsvIncludedDuringCreation
	moexCsvSecurityHasDefault
	moexCsvSecurityHasTechDefault
	moexCsvIncludedWithoutCompliance
	moexCsvRetainedWithoutCompliance
	moexCsvHasRestrictionCirculation
	moexCsvListingLevelHist
	moexCsvObligationProgramRn
	moexCsvCouponPercent
	moexCsvEarlyRepayment
	moexCsvEarlyRedemption
	moexCsvIssBoards
	moexCsvOtherSecurities
	moexCsvDisclosurePartPage
	moexCsvDisclosureRfInfoPage
	moexCsvLast
)

var expectedOrder = []string{
	"DATESTAMP", "INSTRUMENT_ID", "LIST_SECTION", "RN", "SUPERTYPE",
	"INSTRUMENT_TYPE", "INSTRUMENT_CATEGORY", "TRADE_CODE", "ISIN",
	"REGISTRY_NUMBER", "REGISTRY_DATE", "EMITENT_FULL_NAME", "INN",
	"NOMINAL", "CURRENCY", "ISSUE_AMOUNT", "DECISION_DATE",
	"OKSM_EDR", "ONLY_EMITENT_FULL_NAME", "REG_COUNTRY", "QUALIFIED_INVESTOR",
	"HAS_PROSPECTUS", "IS_CONCESSION_AGREEMENT", "IS_MORTGAGE_AGENT", "INCLUDED_DURING_CREATION",
	"SECURITY_HAS_DEFAULT", "SECURITY_HAS_TECH_DEFAULT", "INCLUDED_WITHOUT_COMPLIANCE", "RETAINED_WITHOUT_COMPLIANCE",
	"HAS_RESTRICTION_CIRCULATION", "LISTING_LEVEL_HIST", "OBLIGATION_PROGRAM_RN", "COUPON_PERCENT",
	"EARLY_REPAYMENT", "EARLY_REDEMPTION", "ISS_BOARDS", "OTHER_SECURITIES",
	"DISCLOSURE_PART_PAGE", "DISCLOSURE_RF_INFO_PAGE",
}

type Bond struct {
	Name           string
	Nominal        float64
	CouponInterest float64
	Currency       string
}

func DownloadAndParse() map[string]Bond {
	result := make(map[string]Bond)
	headerChecked := false

	resp, err := http.Get("https://www.moex.com/ru/listing/securities-list-csv.aspx?type=1")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	r := csv.NewReader(charmap.Windows1251.NewDecoder().Reader(resp.Body))
	for {
		parsed, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}

			log.Fatal(err)
		}

		if !headerChecked {
			headerChecked = true
			if len(parsed) < len(expectedOrder) {
				log.Fatal("header changed, too short: ", parsed)
			}

			for i := range expectedOrder {
				if parsed[i] == expectedOrder[i] {
					continue
				}

				log.Fatalf("moex order changed: expected `%s', got `%s'",
					expectedOrder[i], parsed[i])
			}
		}

		// Exclude bad strings
		if len(parsed) < int(moexCsvLast) {
			log.Println("skip bad csv: ", parsed)
			continue
		}
		if parsed[moexCsvISIN] == "" {
			continue
		}
		if parsed[moexCsvSupertype] != "Облигации" {
			continue
		}

		// We are not qualified investors yet
		if parsed[moexCsvQualifiedInvestor] == "+" {
			continue
		}

		// Skip bad emitents
		if parsed[moexCsvSecurityHasDefault] == "+" || parsed[moexCsvSecurityHasTechDefault] == "+" {
			continue
		}

		i := strings.Index(parsed[moexCsvCouponPercent], "%")
		if i == -1 {
			continue
		}

		percentString := strings.Replace(parsed[moexCsvCouponPercent][:i], ",", ".", 1)
		percent, err := strconv.ParseFloat(percentString, 64)
		if err != nil {
			log.Fatal("can't parse coupon percent (moex): ", err)
		}

		nominalString := strings.Replace(parsed[moexCsvNominal], ",", ".", 1)
		nominal, err := strconv.ParseFloat(nominalString, 64)
		if err != nil {
			log.Fatal("can't parse nominam (moex): ", err)
		}

		result[parsed[moexCsvISIN]] = Bond{
			Name:           parsed[moexCsvEmitentFullName],
			Nominal:        nominal,
			CouponInterest: percent,
			Currency:       parsed[moexCsvCurrency],
		}

		if parsed[moexCsvISIN] != parsed[moexCsvTradeCode] {
			result[parsed[moexCsvTradeCode]] = Bond{
				Name:           parsed[moexCsvEmitentFullName],
				Nominal:        nominal,
				CouponInterest: percent,
				Currency:       parsed[moexCsvCurrency],
			}
		}
	}

	return result
}
