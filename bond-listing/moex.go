package main

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
)

type moexBond struct {
	Name           string
	Nominal        float64
	CouponInterest float64
	Currency       string
}

func moexDownloadAndParse() map[string]moexBond {
	result := make(map[string]moexBond)

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

		// Exclude bad strings
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

		result[parsed[moexCsvISIN]] = moexBond{
			Name:           parsed[moexCsvEmitentFullName],
			Nominal:        nominal,
			CouponInterest: percent,
			Currency:       parsed[moexCsvCurrency],
		}

		if parsed[moexCsvISIN] != parsed[moexCsvTradeCode] {
			result[parsed[moexCsvTradeCode]] = moexBond{
				Name:           parsed[moexCsvEmitentFullName],
				Nominal:        nominal,
				CouponInterest: percent,
				Currency:       parsed[moexCsvCurrency],
			}
		}
	}

	return result
}
