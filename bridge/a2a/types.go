package a2a

// AgentCard represents the Agent Card as per A2A specification.
// Only the minimal fields required for S7 are included.
type AgentCard struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	URL          string   `json:"url,omitempty"`
	Version      string   `json:"version,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Skills       []Skill  `json:"skills,omitempty"`
}

type Skill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// DefaultAgentCard returns a basic Agent Card for the zero-bridge.
// The name comes from the agent info provided during initialize.
func DefaultAgentCard(agentName, agentVersion string) AgentCard {
	name := "zero-bridge"
	if agentName != "" {
		name = agentName
	}
	return AgentCard{
		Name:         name,
		Description:  "A2A ingress for zero-bridge",
		Version:      agentVersion,
		Capabilities: []string{"streaming"},
		Skills: []Skill{
			{
				ID:          "chat",
				Name:        "Chat",
				Description: "Send a message to the agent and receive a streaming response",
				Tags:        []string{"chat", "streaming"},
			},
		},
	}
}
