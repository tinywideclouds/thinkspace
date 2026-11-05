package tools

type Registry struct {
	tools map[ToolName]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[ToolName]Tool),
	}
}

func (r *Registry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

func (r *Registry) Get(name string) (Tool, bool) {
	tool, exists := r.tools[ToolName(name)]
	return tool, exists
}
