package flows

// FlowConfig defines the YAML structure for generalized orchestration flows.
type FlowConfig struct {
	Name              string `yaml:"name"`
	Description       string `yaml:"description"`
	ManagerGuidelines string `yaml:"manager_guidelines"`
	RetryPrompt       string `yaml:"retry_prompt"`
}
