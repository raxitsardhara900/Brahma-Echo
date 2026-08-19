package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
)

var schemaArgTypesOnce = sync.OnceValue(func() map[string]map[string]string {
	types := make(map[string]map[string]string)
	for _, tool := range allTools() {
		types[tool.Name] = typedArgsOf(tool)
	}
	return types
})

// typedArgsOf reads the argument types out of the tool's own schema, so the
// declaration that documents an argument is the one that validates it. A second
// hand-written list of numeric arguments would drift the first time a tool gains
// one.
func typedArgsOf(tool mcp.Tool) map[string]string {
	raw, err := json.Marshal(tool.InputSchema)
	if err != nil {
		return nil
	}
	var schema struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil
	}

	types := make(map[string]string, len(schema.Properties))
	for name, property := range schema.Properties {
		switch property.Type {
		case "number", "integer", "boolean":
			types[name] = property.Type
		}
	}
	return types
}

// validateTypedArgs rejects an argument the caller did pass in a shape the
// accessors cannot read. Dropping it instead is what let a malformed deltaY
// degrade into a scroll with no magnitude — or, with direction set, into a
// confidently wrong scroll the other way, which the caller cannot detect.
func validateTypedArgs(toolName string, args map[string]any) error {
	types := schemaArgTypesOnce()[toolName]
	if len(types) == 0 || len(args) == 0 {
		return nil
	}

	bad := make([]string, 0, len(args))
	for name, declared := range types {
		value, present := args[name]
		if !present || value == nil {
			continue
		}
		if raw, isString := value.(string); isString && strings.TrimSpace(raw) == "" {
			continue
		}
		if typedArgIsReadable(declared, value) {
			continue
		}
		bad = append(bad, fmt.Sprintf("%s expects a %s, got %s", name, declared, describeArgValue(value)))
	}
	if len(bad) == 0 {
		return nil
	}

	sort.Strings(bad)
	return fmt.Errorf("%s: %s", toolName, strings.Join(bad, "; "))
}

func typedArgIsReadable(declared string, value any) bool {
	switch declared {
	case "number", "integer":
		if _, ok := value.(float64); ok {
			return true
		}
		raw, ok := value.(string)
		if !ok {
			return false
		}
		_, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		return err == nil
	case "boolean":
		if _, ok := value.(bool); ok {
			return true
		}
		raw, ok := value.(string)
		if !ok {
			return false
		}
		_, err := strconv.ParseBool(strings.TrimSpace(raw))
		return err == nil
	}
	return true
}

// describeArgValue echoes what arrived so the model can correct itself in one
// turn, quoting strings so a numeric-looking string is distinguishable from a
// number.
func describeArgValue(value any) string {
	if raw, ok := value.(string); ok {
		return strconv.Quote(raw)
	}
	return fmt.Sprintf("%v", value)
}

// withTypedArgChecks rejects malformed arguments before the handler runs, so an
// unreadable value never reaches upstream and every accessor's (_, false) again
// means only "absent" — which is what all of their call sites already assume.
func withTypedArgChecks(name string, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, r mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if err := validateTypedArgs(name, r.GetArguments()); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return h(ctx, r)
	}
}
