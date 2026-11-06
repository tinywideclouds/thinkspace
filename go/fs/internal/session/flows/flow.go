package flows

// FlowStrategy defines how the user wants to process candidate branches.
type FlowStrategy string

const (
	StrategySkip   FlowStrategy = "skip"
	StrategyManual FlowStrategy = "manual"
	StrategyReview FlowStrategy = "review"
	StrategyRefine FlowStrategy = "refine"
)

// BranchResult encapsulates the final state of a candidate branch produced by a flow.
type BranchResult struct {
	CandidateID string
	Passed      bool
	Trace       string
	RawPayload  string
	Instruction string
}

// FlowAction defines the toolkit of capabilities the FlowEngine knows how to execute.
type FlowAction string

const (
	ActionSpawnAgents     FlowAction = "spawn_agents"
	ActionRequestStrategy FlowAction = "request_strategy"
	ActionSynthesize      FlowAction = "synthesize"
	ActionManualReview    FlowAction = "manual_review"
	ActionCommit          FlowAction = "commit"
	ActionFinish          FlowAction = "finish"
	ActionError           FlowAction = "error"
)

// FlowState is passed to the Flow so it can evaluate what has happened so far.
type FlowState struct {
	InitialTasks []SubAgentTask
	Candidates   []BranchResult
	UserStrategy FlowStrategy
	TargetBranch string
}

// FlowDirective is the command the Flow returns to direct the FlowEngine.
type FlowDirective struct {
	Action       FlowAction
	Tasks        []SubAgentTask
	TargetBranch string
	Error        error
}

// SubAgentTask defines a single isolated work unit for a sub-agent.
type SubAgentTask struct {
	AgentID       string
	ContextDigest string
	Instruction   string
	TargetFiles   []string
	RetryPrompt   string
}

// FlowContext provides stable metadata to the FlowEngine.
type FlowContext struct {
	FlowID         string
	SpaceID        string
	BaseAgentRules string
	FlowSummary    string
}

// Flow encapsulates the domain logic for a specific orchestration pattern.
type Flow interface {
	Name() string
	MaxSteps() int
	Next(state FlowState) FlowDirective
}
