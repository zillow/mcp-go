package mcp

// PromptOption is a function that configures a Prompt.
// It provides a flexible way to set various properties of a Prompt using the functional options pattern.
type PromptOption func(*Prompt)

// ArgumentOption is a function that configures a PromptArgument.
// It allows for flexible configuration of prompt arguments using the functional options pattern.
type ArgumentOption func(*PromptArgument)

//
// Core Prompt Functions
//

// NewPrompt creates a new Prompt with the given name and options.
// The prompt will be configured based on the provided options.
// Options are applied in order, allowing for flexible prompt configuration.
func NewPrompt(name string, opts ...PromptOption) Prompt {
	prompt := Prompt{
		Name: name,
	}

	for _, opt := range opts {
		opt(&prompt)
	}

	return prompt
}

// WithPromptDescription adds a description to the Prompt.
// The description should provide a clear, human-readable explanation of what the prompt does.
func WithPromptDescription(description string) PromptOption {
	return func(p *Prompt) {
		p.Description = description
	}
}

// WithArgument adds an argument to the prompt's argument list.
// The argument will be configured based on the provided options.
func WithArgument(name string, opts ...ArgumentOption) PromptOption {
	return func(p *Prompt) {
		arg := PromptArgument{
			Name: name,
		}

		for _, opt := range opts {
			opt(&arg)
		}

		if p.Arguments == nil {
			p.Arguments = make([]PromptArgument, 0)
		}
		p.Arguments = append(p.Arguments, arg)
	}
}

//
// Argument Options
//

// ArgumentDescription adds a description to a prompt argument.
// The description should explain the purpose and expected values of the argument.
func ArgumentDescription(desc string) ArgumentOption {
	return func(arg *PromptArgument) {
		arg.Description = desc
	}
}

// RequiredArgument marks an argument as required in the prompt.
// Required arguments must be provided when getting the prompt.
func RequiredArgument() ArgumentOption {
	return func(arg *PromptArgument) {
		arg.Required = true
	}
}
