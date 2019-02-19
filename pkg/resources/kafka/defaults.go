package kafka

const (
	headlessServiceTemplate = "%s-headless"
	brokerConfigTemplate    = "%s-config"
	brokerConfigVolumeMount = "broker-config"
	kafkaDataVolumeMount    = "kafka-data"
	podNamespace            = "POD_NAMESPACE"
)
