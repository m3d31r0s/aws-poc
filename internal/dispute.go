package internal

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type (
	// Date is an type to parse a string in the format YYYY-MM-DD and parse to time.Time
	Date time.Time

	dispute struct {
		CorrelationID       string
		DisputeID           int
		AccountID           int
		AuthorizationCode   string
		ReasonCode          string
		CardID              string
		Tenant              string
		DisputeAmount       float64
		TransactionDate     Date
		LocalCurrencyCode   string
		TextMessage         string
		DocumentIndicator   bool
		IsPartialChargeback bool
	}

	register interface {
		lock(dispute) (ok bool)
		unlock(dispute)
	}

	mapper interface {
		fromJSON(string, string) (dispute, error)
	}

	disputer interface {
		open(dispute) error
	}

	service struct {
		register
		mapper
		disputer
	}
)

// UnmarshalJSON receive a date in []bytes and parse it in the pattern YYYY-MM-DD
func (d *Date) UnmarshalJSON(data []byte) error {
	if string(data) == "null" { //TODO: cover this flow
		return nil
	} //TODO: cover it with unit tests

	s := strings.Trim(string(data), `"`)
	t, err := time.Parse("2006-01-02", s)
	if err != nil { //TODO: cover this flow
		return err
	} //TODO: cover it with unit tests

	*d = Date(t)

	return nil
}

func (s service) open(_ dispute) error {
	return nil
}

func (s service) handleMessage(cid, body string) error {
	d, err := s.mapper.fromJSON(cid, body)
	if err != nil { // TODO: change errors to custom errors aiming to type assertions in tests improve handleMessage tests
		return fmt.Errorf("parser error: %s", err.Error())
	}

	if ok := s.register.lock(d); !ok {
		return fmt.Errorf("idempotence error: cid(%v), disputeId(%v)", cid, d.DisputeID)
	}

	if err := s.disputer.open(d); err != nil {
		defer s.register.unlock(d)
		return fmt.Errorf("parser error: %s", err.Error())
	}

	return nil
}

func (s service) fromJSON(cid, j string) (dispute, error) {
	var d dispute
	err := json.Unmarshal([]byte(j), &d)
	if err != nil {
		return dispute{}, err
	}
	d.CorrelationID = cid
	return d, nil
}
