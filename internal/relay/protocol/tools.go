package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
)

func openAIToolsToGemini(tools []map[string]interface{}) []map[string]interface{} {
	if len(tools) == 0 {
		return nil
	}
	declarations := make([]interface{}, 0, len(tools))
	for _, tool := range tools {
		function, _ := tool["function"].(map[string]interface{})
		if function == nil {
			function = tool
		}
		name, _ := function["name"].(string)
		if name == "" {
			continue
		}
		decl := map[string]interface{}{"name": name}
		if description, _ := function["description"].(string); description != "" {
			decl["description"] = description
		}
		if parameters, ok := function["parameters"]; ok && parameters != nil {
			decl["parameters"] = parameters
		} else {
			decl["parameters"] = emptyObjectSchema()
		}
		declarations = append(declarations, decl)
	}
	if len(declarations) == 0 {
		return nil
	}
	return []map[string]interface{}{{"functionDeclarations": declarations}}
}

func geminiToolsToOpenAI(tools []map[string]interface{}) []map[string]interface{} {
	if len(tools) == 0 {
		return nil
	}
	converted := make([]map[string]interface{}, 0)
	for _, tool := range tools {
		decls, ok := asInterfaceSlice(tool["functionDeclarations"])
		if !ok {
			decls, _ = asInterfaceSlice(tool["function_declarations"])
		}
		for _, value := range decls {
			decl, ok := value.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := decl["name"].(string)
			if name == "" {
				continue
			}
			function := map[string]interface{}{"name": name}
			if description, _ := decl["description"].(string); description != "" {
				function["description"] = description
			}
			if parameters, ok := decl["parameters"]; ok && parameters != nil {
				function["parameters"] = parameters
			} else {
				function["parameters"] = emptyObjectSchema()
			}
			converted = append(converted, map[string]interface{}{"type": "function", "function": function})
		}
	}
	return converted
}

func openAIToolChoiceToGemini(choice interface{}) interface{} {
	if choice == nil {
		return nil
	}
	config := map[string]interface{}{}
	switch value := choice.(type) {
	case string:
		switch strings.ToLower(value) {
		case "none":
			config["mode"] = "NONE"
		case "required":
			config["mode"] = "ANY"
		default:
			config["mode"] = "AUTO"
		}
	case map[string]interface{}:
		if function, ok := value["function"].(map[string]interface{}); ok {
			if name, _ := function["name"].(string); name != "" {
				config["mode"] = "ANY"
				config["allowedFunctionNames"] = []string{name}
			}
		}
	}
	if len(config) == 0 {
		return nil
	}
	return map[string]interface{}{"functionCallingConfig": config}
}

func geminiToolChoiceToOpenAI(toolConfig interface{}) interface{} {
	config, ok := toolConfig.(map[string]interface{})
	if !ok {
		return nil
	}
	fcc, _ := config["functionCallingConfig"].(map[string]interface{})
	if fcc == nil {
		fcc, _ = config["function_calling_config"].(map[string]interface{})
	}
	if fcc == nil {
		return nil
	}
	mode, _ := fcc["mode"].(string)
	switch strings.ToUpper(mode) {
	case "NONE":
		return "none"
	case "ANY":
		if names, ok := stringSlice(fcc["allowedFunctionNames"]); ok && len(names) > 0 {
			return map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": names[0]}}
		}
		if names, ok := stringSlice(fcc["allowed_function_names"]); ok && len(names) > 0 {
			return map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": names[0]}}
		}
		return "required"
	default:
		return "auto"
	}
}

func geminiFunctionCallToOpenAI(value interface{}) map[string]interface{} {
	call, ok := value.(map[string]interface{})
	if !ok || call == nil {
		return nil
	}
	name, _ := call["name"].(string)
	if name == "" {
		return nil
	}
	id, _ := call["id"].(string)
	if id == "" {
		id = generatedID("call")
	}
	return map[string]interface{}{
		"id":   id,
		"type": "function",
		"function": map[string]interface{}{
			"name":      name,
			"arguments": jsonString(call["args"]),
		},
	}
}

func openAIToolCallToGeminiFunctionCall(toolCall map[string]interface{}) map[string]interface{} {
	function, _ := toolCall["function"].(map[string]interface{})
	if function == nil {
		return nil
	}
	name, _ := function["name"].(string)
	if name == "" {
		return nil
	}
	call := map[string]interface{}{"name": name, "args": parseJSONValue(function["arguments"])}
	if id, _ := toolCall["id"].(string); id != "" {
		call["id"] = id
	}
	return call
}

func geminiFunctionResponseToChatMessage(value interface{}) (ChatMessage, bool) {
	resp, ok := value.(map[string]interface{})
	if !ok || resp == nil {
		return ChatMessage{}, false
	}
	name, _ := resp["name"].(string)
	id, _ := resp["id"].(string)
	content := ""
	if response, ok := resp["response"]; ok {
		content = jsonString(response)
	}
	if content == "" || content == "{}" {
		content = name
	}
	return ChatMessage{Role: "tool", ToolCallID: id, Content: content}, true
}

func chatToolMessageToGeminiFunctionResponse(message ChatMessage) map[string]interface{} {
	response := parseJSONValue(message.Content)
	if response == nil {
		response = map[string]interface{}{"content": message.Content}
	}
	functionResponse := map[string]interface{}{"response": response}
	if message.ToolCallID != "" {
		functionResponse["id"] = message.ToolCallID
	}
	return functionResponse
}

func normalizeOpenAIToolCallDelta(delta map[string]interface{}) map[string]interface{} {
	if delta == nil {
		return nil
	}
	out := map[string]interface{}{}
	if index, ok := delta["index"]; ok {
		out["index"] = index
	}
	if id, _ := delta["id"].(string); id != "" {
		out["id"] = id
	}
	if typ, _ := delta["type"].(string); typ != "" {
		out["type"] = typ
	} else {
		out["type"] = "function"
	}
	if function, ok := delta["function"].(map[string]interface{}); ok {
		fn := map[string]interface{}{}
		if name, _ := function["name"].(string); name != "" {
			fn["name"] = name
		}
		if arguments, _ := function["arguments"].(string); arguments != "" {
			fn["arguments"] = arguments
		}
		if len(fn) > 0 {
			out["function"] = fn
		}
	}
	return out
}

func emptyObjectSchema() map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
}

func asInterfaceSlice(value interface{}) ([]interface{}, bool) {
	switch items := value.(type) {
	case []interface{}:
		return items, true
	case []map[string]interface{}:
		out := make([]interface{}, 0, len(items))
		for _, item := range items {
			out = append(out, item)
		}
		return out, true
	default:
		return nil, false
	}
}

func stringSlice(value interface{}) ([]string, bool) {
	switch items := value.(type) {
	case []string:
		return items, true
	case []interface{}:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if text, ok := item.(string); ok && text != "" {
				out = append(out, text)
			}
		}
		return out, true
	default:
		return nil, false
	}
}

func mapFromStruct(value interface{}) map[string]interface{} {
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func mergeToolCallDelta(existing map[string]interface{}, delta map[string]interface{}) map[string]interface{} {
	if existing == nil {
		existing = map[string]interface{}{"type": "function", "function": map[string]interface{}{}}
	}
	if id, _ := delta["id"].(string); id != "" {
		existing["id"] = id
	}
	if typ, _ := delta["type"].(string); typ != "" {
		existing["type"] = typ
	}
	fn, _ := existing["function"].(map[string]interface{})
	if fn == nil {
		fn = map[string]interface{}{}
		existing["function"] = fn
	}
	if deltaFn, _ := delta["function"].(map[string]interface{}); deltaFn != nil {
		if name, _ := deltaFn["name"].(string); name != "" {
			fn["name"] = name
		}
		if args, _ := deltaFn["arguments"].(string); args != "" {
			old, _ := fn["arguments"].(string)
			fn["arguments"] = old + args
		}
	}
	return existing
}

func toolCallID(value map[string]interface{}) string {
	id, _ := value["id"].(string)
	return id
}

func toolCallNameAndArguments(value map[string]interface{}) (string, string) {
	function, _ := value["function"].(map[string]interface{})
	name, _ := function["name"].(string)
	arguments, _ := function["arguments"].(string)
	return name, arguments
}

func ensureToolCallID(id string) string {
	if strings.TrimSpace(id) == "" {
		return generatedID("call")
	}
	return id
}

func formatToolNameFallback(id string) string {
	if id == "" {
		return "function"
	}
	return fmt.Sprintf("function_%s", strings.ReplaceAll(id, "-", "_"))
}
