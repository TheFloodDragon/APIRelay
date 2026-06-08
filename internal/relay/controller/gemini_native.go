package controller

import (
	"fmt"
	"net/url"
	"strings"
)

type geminiNativeRouteKind string

const (
	geminiNativeRouteUnknown     geminiNativeRouteKind = "unknown"
	geminiNativeRouteModels      geminiNativeRouteKind = "models"
	geminiNativeRouteModel       geminiNativeRouteKind = "model"
	geminiNativeRouteGenerate    geminiNativeRouteKind = "generateContent"
	geminiNativeRouteCountTokens geminiNativeRouteKind = "countTokens"
)

type geminiNativeRoute struct {
	Kind    geminiNativeRouteKind
	Version string
	Model   string
	Action  string
	Stream  bool
}

func parseGeminiNativePath(path, rawQuery string) (geminiNativeRoute, error) {
	route := geminiNativeRoute{Kind: geminiNativeRouteUnknown}
	rest, version, ok := stripGeminiNamespace(path)
	if !ok {
		return route, fmt.Errorf("不支持的 Gemini namespace: %s", path)
	}
	route.Version = version

	rest = strings.Trim(rest, "/")
	if rest == "" {
		return route, fmt.Errorf("Gemini 路径缺少资源")
	}
	if rest == "models" {
		route.Kind = geminiNativeRouteModels
		return route, nil
	}

	modelAction := rest
	if strings.HasPrefix(modelAction, "models/") {
		modelAction = strings.TrimPrefix(modelAction, "models/")
	}
	if decoded, err := url.PathUnescape(modelAction); err == nil {
		modelAction = decoded
	} else {
		return route, fmt.Errorf("解析 Gemini 路径失败: %w", err)
	}

	modelPart, action, hasAction := strings.Cut(modelAction, ":")
	route.Model = normalizeGeminiModelName(modelPart)
	if route.Model == "" {
		return route, fmt.Errorf("Gemini 路径缺少 model")
	}
	if !hasAction || action == "" {
		route.Kind = geminiNativeRouteModel
		return route, nil
	}

	route.Action = action
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return route, fmt.Errorf("解析 Gemini query 失败: %w", err)
	}
	altSSE := strings.EqualFold(strings.TrimSpace(query.Get("alt")), "sse")

	switch action {
	case "generateContent":
		route.Kind = geminiNativeRouteGenerate
		route.Stream = altSSE
	case "streamGenerateContent":
		route.Kind = geminiNativeRouteGenerate
		route.Stream = true
	case "countTokens":
		route.Kind = geminiNativeRouteCountTokens
	default:
		return route, fmt.Errorf("不支持的 Gemini 操作: %s", action)
	}
	return route, nil
}

func stripGeminiNamespace(path string) (rest, version string, ok bool) {
	path = "/" + strings.TrimLeft(strings.TrimSpace(path), "/")
	for _, namespace := range []string{"/gemini/v1beta", "/gemini/v1", "/v1beta"} {
		version = geminiNamespaceVersion(namespace)
		if path == namespace {
			return "", version, true
		}
		prefix := namespace + "/"
		if strings.HasPrefix(path, prefix) {
			return strings.TrimPrefix(path, prefix), version, true
		}
	}
	return "", "", false
}

func geminiNamespaceVersion(namespace string) string {
	return strings.TrimPrefix(strings.TrimPrefix(namespace, "/gemini/"), "/")
}

func normalizeGeminiModelName(model string) string {
	model = strings.TrimSpace(strings.TrimPrefix(model, "/"))
	model = strings.TrimPrefix(model, "models/")
	return strings.Trim(model, "/")
}
