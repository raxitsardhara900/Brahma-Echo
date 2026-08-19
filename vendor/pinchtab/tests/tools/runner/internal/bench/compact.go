package bench

const (
	CompactAfterAnthropicMessages = 14
	KeepRecentAnthropicMessages   = 10
	CompactAfterOpenAIItems       = 16
	KeepRecentOpenAIItems         = 10
)

func CompactAnthropicConversation(conversation []interface{}, summary string) []interface{} {
	if len(conversation) <= CompactAfterAnthropicMessages {
		return conversation
	}
	head := conversation[0]
	recentStart := len(conversation) - KeepRecentAnthropicMessages
	if recentStart < 1 {
		recentStart = 1
	}
	recent := conversation[recentStart:]

	result := make([]interface{}, 0, 2+len(recent))
	result = append(result, head)
	result = append(result, map[string]interface{}{
		"role":    "user",
		"content": summary,
	})
	result = append(result, recent...)
	return result
}

func CompactOpenAIConversation(conversation []interface{}, summary string) []interface{} {
	if len(conversation) <= CompactAfterOpenAIItems {
		return conversation
	}
	head := conversation[0]
	recentStart := len(conversation) - KeepRecentOpenAIItems
	if recentStart < 1 {
		recentStart = 1
	}
	recent := conversation[recentStart:]

	result := make([]interface{}, 0, 2+len(recent))
	result = append(result, head)
	result = append(result, map[string]interface{}{
		"role": "user",
		"content": []interface{}{
			map[string]interface{}{"type": "input_text", "text": summary},
		},
	})
	result = append(result, recent...)
	return result
}

func CompactConversation(provider string, conversation []interface{}, summary string) []interface{} {
	if provider == "anthropic" {
		return CompactAnthropicConversation(conversation, summary)
	}
	return CompactOpenAIConversation(conversation, summary)
}
