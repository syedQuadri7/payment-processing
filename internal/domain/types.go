package domain

// Provider represents a payment provider
type Provider string

const (
	ProviderStripe Provider = "STRIPE"
	ProviderAdyen  Provider = "ADYEN"
	ProviderPayPal Provider = "PAYPAL"
)

// PaymentMethodType represents the type of payment method
type PaymentMethodType string

const (
	PaymentMethodCard   PaymentMethodType = "CARD"
	PaymentMethodACH    PaymentMethodType = "ACH"
	PaymentMethodWallet PaymentMethodType = "WALLET"
)

// PaymentMethodStatus represents the status of a payment method
type PaymentMethodStatus string

const (
	PaymentMethodStatusActive  PaymentMethodStatus = "ACTIVE"
	PaymentMethodStatusExpired PaymentMethodStatus = "EXPIRED"
	PaymentMethodStatusDeleted PaymentMethodStatus = "DELETED"
)

// PaymentIntentStatus represents the status of a payment intent
type PaymentIntentStatus string

const (
	PaymentIntentStatusCreated        PaymentIntentStatus = "CREATED"
	PaymentIntentStatusRequiresMethod PaymentIntentStatus = "REQUIRES_METHOD"
	PaymentIntentStatusRequiresAuth   PaymentIntentStatus = "REQUIRES_AUTH"
	PaymentIntentStatusAuthorized     PaymentIntentStatus = "AUTHORIZED"
	PaymentIntentStatusCaptured       PaymentIntentStatus = "CAPTURED"
	PaymentIntentStatusFailed         PaymentIntentStatus = "FAILED"
	PaymentIntentStatusCancelled      PaymentIntentStatus = "CANCELLED"
)

// CaptureMethod represents how the payment should be captured
type CaptureMethod string

const (
	CaptureMethodAutomatic CaptureMethod = "AUTOMATIC"
	CaptureMethodManual    CaptureMethod = "MANUAL"
)

// HoldStatus represents the status of an authorization hold
type HoldStatus string

const (
	HoldStatusActive   HoldStatus = "ACTIVE"
	HoldStatusCaptured HoldStatus = "CAPTURED"
	HoldStatusVoided   HoldStatus = "VOIDED"
	HoldStatusExpired  HoldStatus = "EXPIRED"
)

// AttemptStatus represents the status of a payment attempt
type AttemptStatus string

const (
	AttemptStatusPending    AttemptStatus = "PENDING"
	AttemptStatusProcessing AttemptStatus = "PROCESSING"
	AttemptStatusSucceeded  AttemptStatus = "SUCCEEDED"
	AttemptStatusFailed     AttemptStatus = "FAILED"
)

// DeclineType categorizes decline reasons
type DeclineType string

const (
	DeclineTypeSoft      DeclineType = "SOFT"
	DeclineTypeHard      DeclineType = "HARD"
	DeclineTypeFraud     DeclineType = "FRAUD"
	DeclineTypeTemporary DeclineType = "TEMPORARY"
)
