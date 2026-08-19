package bench

import "time"

type ToolCall struct {
	ID             string
	Command        string
	TimeoutSeconds int
}

type ToolExecutionResult struct {
	ID      string
	IsError bool
	Content string
}

type UsageCounters struct {
	RequestCount             int
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int
}

type Runner interface {
	Provider() string
	Source() string
	Model() string
	ToolDefinitions() interface{}
	InitialConversation(userPrompt string) []interface{}
	Send(systemPrompt string, conversation []interface{}) (interface{}, error)
	ExtractToolCalls(response interface{}, defaultTimeout time.Duration) []ToolCall
	AppendToolResults(conversation []interface{}, response interface{}, results []ToolExecutionResult) []interface{}
	ExtractFinalText(response interface{}) string
	Usage() UsageCounters
}
