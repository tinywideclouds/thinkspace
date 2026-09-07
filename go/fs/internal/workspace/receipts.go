package workspace

import (
	"encoding/xml"
	"time"
)

// FlowReceipt is a static, domain-agnostic snapshot of an orchestration flow.
type FlowReceipt struct {
	XMLName   xml.Name  `xml:"FlowReceipt"`
	FlowID    string    `xml:"FlowID,attr"`
	TaskID    string    `xml:"TaskID,attr"`
	Timestamp time.Time `xml:"Timestamp,attr"`

	// The overarching objective of the flow
	Task string `xml:"Task"`

	// The individual isolated attempts
	Agents []AgentRecord `xml:"Agents>Agent"`

	// The final outcome or LLM evaluation of the entire flow
	Summary string `xml:"Summary"`
}

// AgentRecord captures the raw I/O of a single sub-agent execution.
type AgentRecord struct {
	AgentID           string `xml:"AgentID,attr"`
	Passed            bool   `xml:"Passed,attr"`
	Instruction       string `xml:"Instruction"`
	RawPayload        string `xml:"RawPayload"`
	VerificationTrace string `xml:"VerificationTrace"`
	StateDelta        string `xml:"StateDelta"`
}
