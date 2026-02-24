package auth

import (
	"net/http"
	"strings"
)

// ClientType 客户端类型
type ClientType string

const (
	ClientTypeWeb     ClientType = "web"     // Web浏览器
	ClientTypeMobile  ClientType = "mobile"  // 移动端应用
	ClientTypeUnknown ClientType = "unknown" // 未知类型
)

// DetectClientType 检测客户端类型
// 优先级：自定义请求头 > User-Agent检测 > 默认策略
func DetectClientType(r *http.Request) ClientType {
	// 1. 最高优先级：自定义请求头（明确标识）
	if clientType := r.Header.Get("X-Client-Type"); clientType != "" {
		switch strings.ToLower(clientType) {
		case "web":
			return ClientTypeWeb
		case "mobile":
			return ClientTypeMobile
		}
	}

	// 2. 中等优先级：User-Agent启发式检测
	userAgent := r.Header.Get("User-Agent")

	// 2.1 明确的移动端标识
	if isMobileUserAgent(userAgent) {
		return ClientTypeMobile
	}

	// 2.2 明确的桌面浏览器标识
	if isDesktopBrowser(userAgent) {
		return ClientTypeWeb
	}

	// 3. 默认策略：安全起见，默认为移动端（不使用Cookie）
	// 原因：移动端误判为Web的后果更严重（Cookie不支持）
	return ClientTypeMobile
}

// isMobileUserAgent 检测是否为移动端User-Agent
func isMobileUserAgent(userAgent string) bool {
	mobileKeywords := []string{
		"Android",
		"iPhone", "iPad", "iPod",
		"Mobile", "mobile",
		"BlackBerry",
		"Windows Phone",
		"React Native", // React Native应用
		"okhttp",       // Android常用网络库
		"Alamofire",    // iOS常用网络库
		"Dart",         // Flutter应用
	}

	for _, keyword := range mobileKeywords {
		if strings.Contains(userAgent, keyword) {
			return true
		}
	}

	return false
}

// isDesktopBrowser 检测是否为桌面浏览器
func isDesktopBrowser(userAgent string) bool {
	// 1. 必须包含桌面操作系统标识
	desktopOS := []string{"Windows", "Macintosh", "Linux", "X11"}
	hasDesktopOS := false

	for _, os := range desktopOS {
		if strings.Contains(userAgent, os) {
			hasDesktopOS = true
			break
		}
	}

	if !hasDesktopOS {
		return false
	}

	// 2. 排除移动设备（避免误判移动浏览器）
	mobileKeywords := []string{
		"Android", "iPhone", "iPad", "iPod", "Mobile",
	}

	for _, keyword := range mobileKeywords {
		if strings.Contains(userAgent, keyword) {
			return false
		}
	}

	// 3. 必须包含浏览器标识
	return strings.Contains(userAgent, "Mozilla") ||
		strings.Contains(userAgent, "Chrome") ||
		strings.Contains(userAgent, "Safari") ||
		strings.Contains(userAgent, "Firefox") ||
		strings.Contains(userAgent, "Edge")
}
