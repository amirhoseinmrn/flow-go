package insecure

// AttackerOrchestrator represents the stateful interface that implements a certain type of attack, e.g., wintermute attack.
type AttackerOrchestrator interface {
	// HandleEgressEvent implements logic of processing the outgoing events received from a corrupted node.
	// Corrupted nodes relay all their outgoing events to the orchestrator instead of dispatching them to the network.
	//
	// Note: as a design assumption, this method is invoked sequentially by the AttackerNetwork to pass the
	// events of corrupted nodes. Hence, no extra concurrency-safe consideration is needed.
	HandleEgressEvent(*EgressEvent) error

	Register(AttackerNetwork)

	// HandleIngressEvent implements the logic of processing an incoming event to a corrupted node.
	// Note: as a design assumption, this method is invoked sequentially by the AttackerNetwork to pass the
	// events of corrupted nodes. Hence, no extra concurrency-safe consideration is needed.
	HandleIngressEvent(*IngressEvent) error
}
