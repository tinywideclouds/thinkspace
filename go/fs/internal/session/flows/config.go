package flows

type FlowConfig struct {
	Name              string `yaml:"name"`
	Description       string `yaml:"description"`
	ManagerGuidelines string `yaml:"manager_guidelines"`
	RetryPrompt       string `yaml:"retry_prompt"`
	ReviewPrompt      string `yaml:"review_prompt"`
	RefinePrompt      string `yaml:"refine_prompt"`
}
