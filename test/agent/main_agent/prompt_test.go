package main_agent_test

import (
	"strings"
	"testing"

	"miss-raspberry-agent/internal/agent/main_agent"
	"miss-raspberry-agent/internal/tools/todo_list"
)

func TestBuildActivationPromptIncludesTodoList(t *testing.T) {
	items := []todo_list.Item{
		{ID: "item-1", Content: "你好", Source: "私聊(用户QQ=123)", TargetType: "private", TargetID: 123, CreatedAt: 1700000000},
		{ID: "item-2", Content: "在吗", Source: "群聊(群号=456,发送者QQ=789)", TargetType: "group", TargetID: 456, CreatedAt: 1700000060},
	}

	prompt := main_agent.BuildActivationPrompt(items)
	for _, want := range []string{"item-1", "item-2", "你好", "在吗", "私聊(用户QQ=123)", "群聊(群号=456,发送者QQ=789)", "private/123", "group/456"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt should contain %q, got:\n%s", want, prompt)
		}
	}
}

func TestBuildActivationPromptEmpty(t *testing.T) {
	prompt := main_agent.BuildActivationPrompt(nil)
	if !strings.Contains(prompt, "空") {
		t.Errorf("empty todo list prompt should mention 空, got:\n%s", prompt)
	}
}

// TestBuildActivationPromptIncludesContext verifies that a producer-supplied Context is rendered
// next to the item so the agent can craft a better reply.
func TestBuildActivationPromptIncludesContext(t *testing.T) {
	items := []todo_list.Item{
		{
			ID:         "item-1",
			Content:    "明天下午的会别忘了",
			Context:    "用户刚出差回来，可能没看到群公告",
			Source:     "API(平台=qq,目标=123)",
			TargetType: "private",
			TargetID:   123,
			CreatedAt:  1700000000,
		},
		{ID: "item-2", Content: "无上下文", Source: "私聊(用户QQ=456)"},
	}

	prompt := main_agent.BuildActivationPrompt(items)
	if !strings.Contains(prompt, "上下文：用户刚出差回来，可能没看到群公告") {
		t.Errorf("prompt should contain the item context, got:\n%s", prompt)
	}
	// The item without context must not get an empty 上下文 label.
	if strings.Count(prompt, "上下文：") != 1 {
		t.Errorf("expected exactly one 上下文 label, got:\n%s", prompt)
	}
}
