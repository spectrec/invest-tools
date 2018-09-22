package moex

import (
	"encoding/csv"
	"fmt"
	"golang.org/x/text/encoding/charmap"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	csvDatestamp uint32 = iota
	csvInstrumentID
	csvListSection
	csvRN
	csvSupertype
	csvInstrumentType
	csvInstrumentCategory
	csvTradeCode
	csvISIN
	csvRegistryNumber
	csvRegistryDate
	csvEmitentFullName
	csvInn
	csvNominal
	csvCurrency
	csvIssueAmount
	csvDecisionDate
	csvOksmEdr
	csvOnlyEmitentFullName
	csvRegCountry
	csvQualifiedInvestor
	csvHasProspectus
	csvIsConcessionAgreement
	csvIsMortgageAgent
	csvIncludedDuringCreation
	csvSecurityHasDefault
	csvSecurityHasTechDefault
	csvIncludedWithoutCompliance
	csvRetainedWithoutCompliance
	csvHasRestrictionCirculation
	csvListingLevelHist
	csvObligationProgramRn
	csvCouponPercent
	csvEarlyRepayment
	csvEarlyRedemption
	csvIssBoards
	csvOtherSecurities
	csvDisclosurePartPage
	csvDisclosureRfInfoPage
	csvLast
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

func DownloadAndParse(debug bool) (map[string]*Bond, error) {
	result := make(map[string]*Bond)
	headerChecked := false

	resp, err := http.Get("https://www.moex.com/ru/listing/securities-list-csv.aspx?type=1")
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	r := csv.NewReader(charmap.Windows1251.NewDecoder().Reader(resp.Body))
	for {
		parsed, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}

			return result, err
		}

		if !headerChecked {
			headerChecked = true
			if len(parsed) < len(expectedOrder) {
				return result, fmt.Errorf("header changed, too short: ", parsed)
			}

			for i := range expectedOrder {
				if parsed[i] == expectedOrder[i] {
					continue
				}

				return result, fmt.Errorf("order changed: expected `%s', got `%s'", expectedOrder[i], parsed[i])
			}
		}

		// Exclude bad strings
		if len(parsed) < int(csvLast) {
			if debug {
				fmt.Println("skip bad csv: ", parsed)
			}

			continue
		}
		if parsed[csvISIN] == "" {
			continue
		}
		if parsed[csvSupertype] != "Облигации" {
			continue
		}

		// We are not qualified investors yet
		if parsed[csvQualifiedInvestor] == "+" {
			continue
		}

		// Skip bad emitents
		if parsed[csvSecurityHasDefault] == "+" || parsed[csvSecurityHasTechDefault] == "+" {
			continue
		}

		i := strings.Index(parsed[csvCouponPercent], "%")
		if i == -1 {
			continue
		}

		percentString := strings.Replace(parsed[csvCouponPercent][:i], ",", ".", 1)
		percent, err := strconv.ParseFloat(percentString, 64)
		if err != nil {
			return result, fmt.Errorf("can't parse coupon percent (moex): ", err)
		}

		nominalString := strings.Replace(parsed[csvNominal], ",", ".", 1)
		nominal, err := strconv.ParseFloat(nominalString, 64)
		if err != nil {
			return result, fmt.Errorf("can't parse nominal (moex): ", err)
		}

		b := Bond{
			Name:           parsed[csvEmitentFullName],
			Nominal:        nominal,
			CouponInterest: percent,
			Currency:       parsed[csvCurrency],
		}

		result[parsed[csvISIN]] = &b
		if parsed[csvISIN] != parsed[csvTradeCode] {
			result[parsed[csvTradeCode]] = &b
		}
	}

	return result, nil
}
