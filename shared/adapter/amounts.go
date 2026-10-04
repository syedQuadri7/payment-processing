package adapter

type stripeDataObject struct {
	Amount           int64               `json:"amount"`
	ID               string              `json:"id"`
	Currency         string              `json:"currency"`
	Status           string              `json:"status"`
	Metadata         map[string]string   `json:"metadata"`
	LastPaymentError *stripePaymentError `json:"last_payment_error"`
	LatestCharge     string              `json:"latest_charge"`
}

type stripeDisputeEvent struct {
	Amount   int64  `json:"amount"`
	ID       string `json:"id"`
	Currency string `json:"currency"`
	Reason   string `json:"reason"`
	Status   string `json:"status"`
}

type adyenAmount struct {
	Value    int64  `json:"value"`
	Currency string `json:"currency"`
}
