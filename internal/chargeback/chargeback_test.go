package chargeback

import (
	"aws-poc/internal/protocol"
	"reflect"
	"testing"
	"time"
)

type (
	errMapper          struct{}
	errRepository      struct{}
	errDisputer        struct{}
	mockRepository     struct{}
	mockMapper         struct{}
	mockCreator        struct{}
	mockCardRepository struct {
		called bool
	}
	mockAttachmentRepository struct {
		getCalled  bool
		saveCalled bool
	}
	mockOpener struct {
		called bool
	}
	mockProducer struct {
		called   bool
		expected *protocol.Chargeback
	}
	mockScheduler struct {
		called bool
	}
	errCardGetter               struct{}
	errAttachmentGetRepository  struct{}
	errAttachmentSaveRepository struct{}
	errOpener                   struct{}
	errProducer                 struct{}
	errScheduler                struct{}
	mockOpenerWithNetworkError  struct {
		called bool
	}
)

func (e errMapper) fromJSON(string, string) (*protocol.Dispute, error) {
	return disputeStub, stubError
}

func (m mockMapper) fromJSON(string, string) (*protocol.Dispute, error) {
	return disputeStub, nil
}

func (e errRepository) lock(*protocol.Dispute) (ok bool) {
	return false
}

func (e errRepository) release(*protocol.Dispute) (ok bool) {
	return false
}

func (e errDisputer) create(*protocol.Dispute) error {
	return stubError
}

func (m mockCreator) create(*protocol.Dispute) error {
	return nil
}

func (m mockRepository) lock(*protocol.Dispute) (ok bool) {
	return true
}

func (m mockRepository) release(*protocol.Dispute) (ok bool) {
	return true
}

func (m *mockCardRepository) Get(dispute *protocol.Dispute) (*protocol.Card, error) {
	m.called = dispute == disputeStub

	return cardStub, nil
}

func (m *mockAttachmentRepository) Get(dispute *protocol.Dispute) (*protocol.Attachment, error) {
	m.getCalled = dispute == disputeStub

	return attachmentStub, nil
}

func (m *mockAttachmentRepository) Save(chargeback *protocol.Chargeback) error {
	m.saveCalled = chargeback == chargebackStub

	return nil
}

func (m *mockOpener) Open(dispute *protocol.Dispute, card *protocol.Card, attachment *protocol.Attachment) (*protocol.Chargeback, error) {
	m.called = dispute == disputeStub && card == cardStub && attachment == attachmentStub

	return chargebackStub, nil
}

func (m errCardGetter) Get(dispute *protocol.Dispute) (*protocol.Card, error) {
	return nil, cardError
}

func (m errAttachmentGetRepository) Get(dispute *protocol.Dispute) (*protocol.Attachment, error) {
	return nil, attGetError
}

func (m errAttachmentGetRepository) Save(chargeback *protocol.Chargeback) error {
	return nil
}

func (m errAttachmentSaveRepository) Get(dispute *protocol.Dispute) (*protocol.Attachment, error) {
	return nil, nil
}

func (m errAttachmentSaveRepository) Save(chargeback *protocol.Chargeback) error {
	return attSaveError
}

func (m errOpener) Open(dispute *protocol.Dispute, card *protocol.Card, attachment *protocol.Attachment) (*protocol.Chargeback, error) {
	return nil, openerError
}

func (m *mockProducer) Produce(chargeback *protocol.Chargeback) error {
	m.called = m.expected == chargeback

	return nil
}

func (m *mockScheduler) Schedule(chargeback *protocol.Chargeback) error {
	m.called = chargeback == chargebackStub

	return nil
}

func (e errProducer) Produce(chargeback *protocol.Chargeback) error {
	return producerError
}

func (e errScheduler) Schedule(chargeback *protocol.Chargeback) error {
	return scdError
}

func (m *mockOpenerWithNetworkError) Open(dispute *protocol.Dispute, card *protocol.Card, attachment *protocol.Attachment) (*protocol.Chargeback, error) {
	m.called = dispute == disputeStub && card == cardStub && attachment == attachmentStub

	return chargebackWithErrorStub, nil
}

func TestMapFromJson(t *testing.T) {
	svc := service{}
	cid := cid
	json := `{
  "disputeId": 611,
  "accountId": 48448,
  "authorizationCode": "7HSGXW",
  "reasonCode": "848",
  "cardId": 3123,
  "orgId": "pismo.io",
  "disputeAmount": 32.32,
  "transactionAmount": 42.65,
  "transactionDate": "2012-04-23",
  "localCurrencyCode": "986",
  "textMessage": "this a test message",
  "documentIndicator": true,
  "isPartial": false
}`
	want := protocol.Dispute{
		Cid:               cid,
		DisputeId:         611,
		AccountId:         48448,
		AuthorizationCode: protocol.AuthorizationCode("7HSGXW"),
		ReasonCode:        protocol.ReasonCode("848"),
		CardId:            3123,
		OrgId:             "pismo.io",
		DisputeAmount:     32.32,
		TransactionDate:   protocol.Date(time.Date(2012, 04, 23, 0, 0, 0, 0, time.UTC)),
		LocalCurrencyCode: protocol.LocalCurrencyCode("986"),
		TextMessage:       "this is a test message",
		DocumentIndicator: true,
		IsPartial:         false,
	}

	got, err := svc.fromJSON(cid, json)

	if err != nil {
		t.Error("fromJSON() error should not be returned")
	}
	if reflect.DeepEqual(got, want) {
		t.Errorf("fromJSON() got: %v, want: %v", got, want)
	}
}

func TestMapFromJson_Error(t *testing.T) {
	svc := service{}

	_, err := svc.fromJSON("", "json")

	if err == nil {
		t.Error("mapFromJson_Error() error should be returned")
	}
}

func TestHandleMessage(t *testing.T) {
	defaultInput := [2]string{cid, "body"}
	cases := []struct {
		name string
		in   [2]string
		want error
		service
	}{
		{"success", defaultInput, nil, service{mapper: mockMapper{}, locker: mockRepository{}, creator: mockCreator{}}},
		{"parseError", defaultInput, newParseError(stubError), service{locker: mockRepository{}, mapper: errMapper{}}},
		{"idempotenceError", defaultInput, newIdempotenceError(cid, disputeID), service{mapper: mockMapper{}, locker: errRepository{}}},
		{"chargebackError", defaultInput, newChargebackError(stubError, cid, disputeID), service{mapper: mockMapper{}, locker: mockRepository{}, creator: errDisputer{}}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.service.handleMessage(c.in[0], c.in[1]); !reflect.DeepEqual(c.want, got) {
				t.Errorf("%s, want: %v, got: %v", c.name, c.want, got)
			}
		})
	}
}

func TestCreateSuccess(t *testing.T) {
	cr, ar, ope, prod, scd := mockCardRepository{}, mockAttachmentRepository{}, mockOpener{}, mockProducer{expected: chargebackStub}, mockScheduler{}
	svc := service{
		cardRepository:       &cr,
		attachmentRepository: &ar,
		opener:               &ope,
		Producer:             &prod,
		Scheduler:            &scd,
	}
	_ = svc.create(disputeStub)

	if !cr.called {
		t.Error("card register not called")
	}
	if !ar.getCalled {
		t.Error("attachment register get not called")
	}
	if !ope.called {
		t.Error("chargeback creator not called")
	}
	if !prod.called {
		t.Error("chargeback producer not called")
	}
	if !ar.saveCalled {
		t.Error("attachment register save not called")
	}
	if !scd.called {
		t.Errorf("scheduler not called")
	}
}

func TestCreateNetworkError(t *testing.T) {
	cr, ar, ope, prod, scd := mockCardRepository{}, mockAttachmentRepository{}, mockOpenerWithNetworkError{}, mockProducer{expected: chargebackWithErrorStub}, mockScheduler{}
	svc := service{
		cardRepository:       &cr,
		attachmentRepository: &ar,
		opener:               &ope,
		Producer:             &prod,
		Scheduler:            &scd,
	}
	_ = svc.create(disputeStub)

	if !cr.called {
		t.Error("card register not called")
	}
	if !ar.getCalled {
		t.Error("attachment register get not called")
	}
	if !ope.called {
		t.Error("chargeback creator not called")
	}
	if !prod.called {
		t.Error("chargeback producer not called")
	}
	if ar.saveCalled {
		t.Error("attachment register save called")
	}
	if scd.called {
		t.Errorf("scheduler called")
	}
}

func TestOpenFail(t *testing.T) {
	cases := []struct {
		name string
		in   *protocol.Dispute
		svc  service
		want error
	}{
		{"cardError", disputeStub, service{
			cardRepository: errCardGetter{},
		}, cardError},
		{"attachmentGetError", disputeStub, service{
			cardRepository:       &mockCardRepository{},
			attachmentRepository: &errAttachmentGetRepository{},
		}, attGetError},
		{"openerError", disputeStub, service{
			cardRepository:       &mockCardRepository{},
			attachmentRepository: &mockAttachmentRepository{},
			opener:               errOpener{},
		}, openerError},
		{"producerError", disputeStub, service{
			cardRepository:       &mockCardRepository{},
			attachmentRepository: &mockAttachmentRepository{},
			opener:               &mockOpener{},
			Producer:             &errProducer{},
		}, producerError},
		{"attachmentSaveError", disputeStub, service{
			cardRepository:       &mockCardRepository{},
			attachmentRepository: &errAttachmentSaveRepository{},
			opener:               &mockOpener{},
			Producer:             &mockProducer{},
		}, attSaveError},
		{"scheduleError", disputeStub, service{
			cardRepository:       &mockCardRepository{},
			attachmentRepository: &mockAttachmentRepository{},
			opener:               &mockOpener{},
			Producer:             &mockProducer{},
			Scheduler:            &errScheduler{},
		}, scdError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.svc.create(c.in); got != c.want {
				t.Errorf("%s, want: %v, got: %v", c.name, c.want, got)
			}
		})
	}
}
