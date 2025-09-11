package i18nx

import (
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"strings"
)

func MessageFormat(t string, messages []*i18n.Message) any {
	switch strings.ToLower(t) {
	case "json":
		var msg []map[string]string
		for _, message := range messages {
			msg = append(msg, messageJsonFormat(message))
		}
		return msg
	case "toml":
		var msg = make(map[string]any)
		for _, message := range messages {
			k, m := messageTomlFormat(message)
			msg[k] = m
		}
		return msg
	}
	return nil
}

func messageJsonFormat(message *i18n.Message) map[string]string {
	return messageFormat(message)
}

func messageTomlFormat(message *i18n.Message) (string, map[string]string) {
	msg := messageFormat(message)
	key := msg["id"]
	delete(msg, "id")
	return key, msg
}

func messageFormat(message *i18n.Message) map[string]string {
	msg := make(map[string]string, 0)
	msg["id"] = message.ID
	if message.Description != "" {
		msg["description"] = message.Description
	}

	if message.Few != "" {
		msg["few"] = message.Few
	}
	if message.One != "" {
		msg["one"] = message.One
	}
	if message.Two != "" {
		msg["two"] = message.Two
	}
	if message.Few != "" {
		msg["few"] = message.Few
	}
	if message.Many != "" {
		msg["many"] = message.Many
	}
	if message.Other != "" {
		msg["other"] = message.Other
	}
	return msg
}
