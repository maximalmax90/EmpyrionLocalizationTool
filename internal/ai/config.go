package ai

type Task string

const (
	TaskTranslate Task = "translate"
	TaskGlossary  Task = "glossary"
	TaskRefine    Task = "refine"
)

type TaskConfig struct {
	Model string `json:"model,omitempty"`

	ReasoningEffort string `json:"reasoning_effort,omitempty"`

	Temperature *float64 `json:"temperature,omitempty"`

	SystemPrompt string `json:"system_prompt,omitempty"`
}

type Config struct {
	URL string `json:"url"`

	Token string `json:"token,omitempty"`

	Default   TaskConfig  `json:"default"`
	Translate *TaskConfig `json:"translate,omitempty"`
	Glossary  *TaskConfig `json:"glossary,omitempty"`
	Refine    *TaskConfig `json:"refine,omitempty"`
}

func (c Config) Resolve(task Task) TaskConfig {
	out := c.Default

	var override *TaskConfig
	switch task {
	case TaskTranslate:
		override = c.Translate
	case TaskGlossary:
		override = c.Glossary
	case TaskRefine:
		override = c.Refine
	}
	if override == nil {
		return out
	}

	if override.Model != "" {
		out.Model = override.Model
	}
	if override.ReasoningEffort != "" {
		out.ReasoningEffort = override.ReasoningEffort
	}
	if override.Temperature != nil {
		out.Temperature = override.Temperature
	}
	if override.SystemPrompt != "" {
		out.SystemPrompt = override.SystemPrompt
	}
	return out
}
