package agent

import "genFu/internal/message"

func lastUserMessage(messages []message.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == message.RoleUser {
			return messages[i].Content
		}
	}
	return ""
}
