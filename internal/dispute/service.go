package dispute

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type (
	date time.Time

	Entity struct {
		CorrelationID       string
		DisputeID           int
		AccountID           int
		AuthorizationCode   string
		ReasonCode          string
		CardID              string
		Tenant              string
		DisputeAmount       float64
		TransactionDate     date
		LocalCurrencyCode   string
		TextMessage         string
		DocumentIndicator   bool
		IsPartialChargeback bool
	}

	locker interface {
		lock(Entity) (ok bool)
		unlock(Entity)
	}

	mapper interface {
		fromJSON(string, string) (Entity, error)
	}

	disputer interface {
		open(Entity) error
	}

	service struct {
		locker
		mapper
		disputer
	}
)

// ID return DisputeID::CorrelationID
func (e Entity) ID() string {
	return fmt.Sprintf("%v::%s", e.DisputeID, e.CorrelationID)
}

// UnmarshalJSON receive a date in []bytes and parse it in the pattern YYYY-MM-DD
func (d *date) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	s := strings.Trim(string(data), `"`)
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}

	*d = date(t)

	return nil
}

func (s service) open(_ Entity) error {
	return nil
}

func (s service) handleMessage(cid, body string) error {
	d, err := s.mapper.fromJSON(cid, body)
	if err != nil {
		return newParseError(err)
	}

	if ok := s.locker.lock(d); !ok {
		return newIdempotenceError(cid, d.DisputeID)
	}

	if err := s.disputer.open(d); err != nil {
		defer s.locker.unlock(d)
		return newChargebackError(err, cid, d.DisputeID)
	}

	return nil
}

func (s service) fromJSON(cid, j string) (Entity, error) {
	var d Entity
	err := json.Unmarshal([]byte(j), &d)
	if err != nil {
		return Entity{}, err
	}
	d.CorrelationID = cid
	return d, nil
}
