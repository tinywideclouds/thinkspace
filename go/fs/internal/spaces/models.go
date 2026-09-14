package spaces

// ModelCategory defines the role of an LLM in the orchestration process.
type ModelCategory string

const (
	ModelCategoryManager ModelCategory = "manager"
	ModelCategoryWorker  ModelCategory = "worker"
)

type ThinkSpaceRoles struct {
	Manager string `yaml:"manager"`
	Worker  string `yaml:"worker"`
}
