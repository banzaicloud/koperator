package kafka

const (
	headlessServiceTemplate = "%s-headless"
	//loadBalancerServiceTemplate = "%s-loadbalancer"
	brokerConfigTemplate    = "%s-config"
	brokerConfigVolumeMount = "broker-config"
	kafkaDataVolumeMount    = "kafka-data"
	podNamespace            = "POD_NAMESPACE"
)
