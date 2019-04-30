package v1alpha1

type BrokerState string

const (
	Running BrokerState = "Running"
	Error   BrokerState = "Error"
)
