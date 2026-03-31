package workflow

type WorkflowStreamEvent struct {
	Type    string        `json:"type"`
	Node    string        `json:"node,omitempty"`
	Delta   string        `json:"delta,omitempty"`
	Reason  string        `json:"reason,omitempty"`
	Payload interface{}   `json:"payload,omitempty"`
	Plan    *WorkflowPlan `json:"plan,omitempty"`
}

type NodeStreamer struct {
	emit func(event WorkflowStreamEvent)
}

func NewNodeStreamer(emit func(event WorkflowStreamEvent)) *NodeStreamer {
	return &NodeStreamer{emit: emit}
}

func (s *NodeStreamer) EmitPlan(plan WorkflowPlan) {
	if s == nil || s.emit == nil {
		return
	}
	p := plan
	s.emit(WorkflowStreamEvent{Type: "plan", Plan: &p})
}

func (s *NodeStreamer) Start(node string) {
	if s == nil || s.emit == nil {
		return
	}
	s.emit(WorkflowStreamEvent{Type: "node_start", Node: node})
}

func (s *NodeStreamer) Delta(node string, delta string) {
	if s == nil || s.emit == nil || delta == "" {
		return
	}
	s.emit(WorkflowStreamEvent{Type: "node_delta", Node: node, Delta: delta})
}

func (s *NodeStreamer) Complete(node string, payload interface{}) {
	if s == nil || s.emit == nil {
		return
	}
	s.emit(WorkflowStreamEvent{Type: "node_complete", Node: node, Payload: payload})
}

func (s *NodeStreamer) Skip(node string, reason string) {
	if s == nil || s.emit == nil {
		return
	}
	s.emit(WorkflowStreamEvent{Type: "node_skip", Node: node, Reason: reason})
}
