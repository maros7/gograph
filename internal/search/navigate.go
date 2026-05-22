package search

import (
	"strings"

	"github.com/ozgurcd/gograph/internal/graph"
)

// NavigateIntent represents the kind of information the caller wants.
type NavigateIntent string

const (
	IntentFields     NavigateIntent = "fields"      // struct fields
	IntentSource     NavigateIntent = "source"      // source code
	IntentMarshal    NavigateIntent = "marshal"     // JSON/gojay marshal implementation
	IntentUnmarshal  NavigateIntent = "unmarshal"   // JSON/gojay unmarshal implementation
	IntentCallers    NavigateIntent = "callers"     // who calls this
	IntentCallees    NavigateIntent = "callees"     // what this calls
	IntentOverview   NavigateIntent = "overview"    // brief: kind + fields + key methods
)

// NavigateResult is a compact, intent-focused response.
type NavigateResult struct {
	Symbol  string `json:"symbol"`
	Kind    string `json:"kind"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Doc     string `json:"doc,omitempty"`
	Source  string `json:"source,omitempty"`
	Fields  []FieldInfo `json:"fields,omitempty"`
	Methods []string    `json:"methods,omitempty"`
}

// FieldInfo is a minimal field representation.
type FieldInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
	JSON string `json:"json,omitempty"` // JSON key extracted from tag
}

// Navigate performs intent-driven graph navigation. Given a symbol and an intent,
// it does multi-hop traversal internally and returns only what's relevant.
// This replaces multiple tool calls with a single focused response.
func Navigate(g *graph.Graph, rootDir, symbol string, intent NavigateIntent) *NavigateResult {
	// Find the symbol
	nodes := Node(g, symbol)
	if len(nodes) == 0 {
		return nil
	}

	// Prefer exact match (struct > method > function)
	var target *graph.SymbolNode
	nl := strings.ToLower(symbol)
	for i := range g.Symbols {
		s := &g.Symbols[i]
		if strings.ToLower(s.Name) == nl {
			if target == nil || symbolPriority(s.Kind) > symbolPriority(target.Kind) {
				target = s
			}
		}
	}
	if target == nil {
		return nil
	}

	result := &NavigateResult{
		Symbol: target.Name,
		Kind:   string(target.Kind),
		File:   target.File,
		Line:   target.Line,
		Doc:    truncateDoc(target.Doc, 200),
	}

	switch intent {
	case IntentFields:
		result.Fields = extractFieldInfos(target)

	case IntentSource:
		src, _ := Source(g, rootDir, symbol)
		result.Source = src

	case IntentMarshal:
		// Find MarshalJSONObject or MarshalJSON method for this type
		methodName := findMethod(g, target.Name, "MarshalJSONObject", "MarshalJSON")
		if methodName != "" {
			src, _ := Source(g, rootDir, methodName)
			result.Source = src
		}

	case IntentUnmarshal:
		methodName := findMethod(g, target.Name, "UnmarshalJSONObject", "UnmarshalJSON")
		if methodName != "" {
			src, _ := Source(g, rootDir, methodName)
			result.Source = src
		}

	case IntentCallers:
		callers := Callers(g, symbol, false)
		for _, c := range callers {
			if len(result.Methods) < 20 {
				result.Methods = append(result.Methods, c.Name+" ("+c.File+")")
			}
		}

	case IntentCallees:
		callees := Callees(g, symbol, false)
		for _, c := range callees {
			if len(result.Methods) < 20 {
				result.Methods = append(result.Methods, c.Name)
			}
		}

	case IntentOverview:
		result.Fields = extractFieldInfos(target)
		// Add key methods (exported, non-boilerplate)
		for _, s := range g.Symbols {
			if s.Kind == graph.KindMethod && strings.EqualFold(s.Receiver, "*"+target.Name) {
				if isInterestingMethod(s.Name) {
					result.Methods = append(result.Methods, s.Name)
				}
			}
		}
	}

	return result
}

// DetectIntent infers the navigation intent from a natural language hint.
func DetectIntent(hint string) NavigateIntent {
	h := strings.ToLower(hint)
	switch {
	case strings.Contains(h, "marshal") && !strings.Contains(h, "unmarshal"):
		return IntentMarshal
	case strings.Contains(h, "unmarshal") || strings.Contains(h, "decode") || strings.Contains(h, "parse"):
		return IntentUnmarshal
	case strings.Contains(h, "field") || strings.Contains(h, "struct") || strings.Contains(h, "schema"):
		return IntentFields
	case strings.Contains(h, "source") || strings.Contains(h, "implementation") || strings.Contains(h, "code"):
		return IntentSource
	case strings.Contains(h, "caller") || strings.Contains(h, "who call") || strings.Contains(h, "used by"):
		return IntentCallers
	case strings.Contains(h, "callee") || strings.Contains(h, "calls") || strings.Contains(h, "depend"):
		return IntentCallees
	default:
		return IntentOverview
	}
}

func symbolPriority(k graph.SymbolKind) int {
	switch k {
	case graph.KindStruct:
		return 4
	case graph.KindInterface:
		return 3
	case graph.KindFunction:
		return 2
	case graph.KindMethod:
		return 1
	default:
		return 0
	}
}

func extractFieldInfos(s *graph.SymbolNode) []FieldInfo {
	if s == nil {
		return nil
	}
	var fields []FieldInfo
	for _, f := range s.StructFields {
		fi := FieldInfo{Name: f.Name, Type: f.Type}
		// Extract JSON key from tag
		if f.Tag != "" {
			if idx := strings.Index(f.Tag, `json:"`); idx >= 0 {
				rest := f.Tag[idx+6:]
				if end := strings.Index(rest, `"`); end >= 0 {
					jsonTag := rest[:end]
					if comma := strings.Index(jsonTag, ","); comma >= 0 {
						jsonTag = jsonTag[:comma]
					}
					if jsonTag != "-" && jsonTag != "" {
						fi.JSON = jsonTag
					}
				}
			}
		}
		fields = append(fields, fi)
	}
	return fields
}

func findMethod(g *graph.Graph, typeName string, methodNames ...string) string {
	tnl := strings.ToLower(typeName)
	for _, mname := range methodNames {
		mnl := strings.ToLower(mname)
		for _, s := range g.Symbols {
			if s.Kind == graph.KindMethod && strings.ToLower(s.Name) == mnl {
				recv := strings.ToLower(s.Receiver)
				if recv == "*"+tnl || recv == tnl {
					// Return the full qualified name for source lookup
					return "(*" + typeName + ")." + s.Name
				}
			}
		}
	}
	return ""
}

func isInterestingMethod(name string) bool {
	// Skip protobuf boilerplate
	boring := []string{
		"Reset", "String", "ProtoMessage", "ProtoReflect",
		"Descriptor", "IsNil", "NKeys",
	}
	for _, b := range boring {
		if name == b {
			return false
		}
	}
	// Skip getters (GetX) — they're obvious from fields
	if strings.HasPrefix(name, "Get") && len(name) > 3 {
		return false
	}
	return true
}

func truncateDoc(doc string, maxLen int) string {
	if len(doc) <= maxLen {
		return doc
	}
	// Cut at last space before maxLen
	cut := doc[:maxLen]
	if idx := strings.LastIndex(cut, " "); idx > 0 {
		cut = cut[:idx]
	}
	return cut + "…"
}
