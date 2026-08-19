package bench

import (
	"testing"
)

func TestCompactAnthropicNoOp(t *testing.T) {
	conv := make([]interface{}, 8)
	for i := range conv {
		conv[i] = map[string]interface{}{"role": "user", "content": "msg"}
	}
	result := CompactAnthropicConversation(conv, "summary")
	if len(result) != 8 {
		t.Errorf("got %d; want 8 (no compaction)", len(result))
	}
}

func TestCompactAnthropicTriggered(t *testing.T) {
	// len=17 triggers compaction (> CompactAfterAnthropicMessages=14)
	conv := make([]interface{}, 17)
	conv[0] = map[string]interface{}{"role": "user", "content": "initial"}
	for i := 1; i < 17; i++ {
		conv[i] = map[string]interface{}{"role": "assistant", "content": i}
	}

	result := CompactAnthropicConversation(conv, "summary text")

	// head + summary + 10 recent = 12
	if len(result) != 12 {
		t.Errorf("got %d; want 12 (head + summary + 10 recent)", len(result))
	}

	// Summary is at position 1
	summary := result[1].(map[string]interface{})
	if summary["role"] != "user" {
		t.Errorf("summary role = %v; want user", summary["role"])
	}
	content, _ := summary["content"].(string)
	if content != "summary text" {
		t.Errorf("summary content = %q; want 'summary text'", content)
	}
}

func TestCompactAnthropicBasic(t *testing.T) {
	// Build a conversation that triggers compaction
	conv := []interface{}{
		map[string]interface{}{"role": "user", "content": "initial"},
	}
	for i := 0; i < 16; i++ {
		conv = append(conv, map[string]interface{}{"role": "assistant", "content": i})
	}

	result := CompactAnthropicConversation(conv, "progress")

	// Should have head + summary + 10 recent
	if len(result) != 12 {
		t.Errorf("got %d; want 12", len(result))
	}

	// Head preserved
	head := result[0].(map[string]interface{})
	if head["content"] != "initial" {
		t.Errorf("head content = %v; want 'initial'", head["content"])
	}

	// Summary at position 1
	summary := result[1].(map[string]interface{})
	if summary["content"] != "progress" {
		t.Errorf("summary content = %v; want 'progress'", summary["content"])
	}
}

func TestCompactOpenAINoOp(t *testing.T) {
	conv := make([]interface{}, 14)
	for i := range conv {
		conv[i] = map[string]interface{}{"type": "message"}
	}
	result := CompactOpenAIConversation(conv, "summary")
	if len(result) != 14 {
		t.Errorf("got %d; want 14 (no compaction)", len(result))
	}
}

func TestCompactOpenAITriggered(t *testing.T) {
	conv := make([]interface{}, 20)
	conv[0] = map[string]interface{}{"role": "user", "content": "initial"}
	for i := 1; i < 20; i++ {
		conv[i] = map[string]interface{}{"type": "function_call", "id": i}
	}

	result := CompactOpenAIConversation(conv, "summary text")

	if len(result) != 12 {
		t.Errorf("got %d; want 12 (head + summary + 10 recent)", len(result))
	}

	summary := result[1].(map[string]interface{})
	content := summary["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("summary content length = %d; want 1", len(content))
	}
	part := content[0].(map[string]interface{})
	if part["text"] != "summary text" {
		t.Errorf("summary text = %v", part["text"])
	}
}

func TestCompactConversationRouting(t *testing.T) {
	conv := make([]interface{}, 20)
	conv[0] = map[string]interface{}{"role": "user", "content": "initial"}
	for i := 1; i < 20; i++ {
		conv[i] = map[string]interface{}{"role": "assistant"}
	}

	anthropic := CompactConversation("anthropic", conv, "summary")
	// head + summary + 10 recent = 12
	if len(anthropic) != 12 {
		t.Errorf("anthropic compaction = %d; want 12", len(anthropic))
	}

	openai := CompactConversation("openai", conv, "summary")
	if len(openai) != 12 {
		t.Errorf("openai compaction = %d; want 12", len(openai))
	}
}
