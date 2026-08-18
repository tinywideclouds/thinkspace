package llm

import "google.golang.org/genai"

func GetWorkspaceTools() []*genai.Tool {
	return []*genai.Tool{
		{
			FunctionDeclarations: []*genai.FunctionDeclaration{
				{
					Name:        "propose_change",
					Description: "Propose a modification to a file. For small edits, use 'patch'. For new files, use 'new_content'.",
					Parameters: &genai.Schema{
						Type: genai.TypeObject,
						Properties: map[string]*genai.Schema{
							"file_path":   {Type: genai.TypeString},
							"patch":       {Type: genai.TypeString},
							"new_content": {Type: genai.TypeString},
							"reasoning":   {Type: genai.TypeString},
						},
						Required: []string{"file_path", "reasoning"},
					},
				},
			},
		},
	}
}
