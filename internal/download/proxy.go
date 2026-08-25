package download

import (
	"net/url"
	"strings"
)

// proxyRule defines whether a domain should use a proxy.
type proxyRule struct {
	suffix string
	useProxy bool
}

// Domains that should NOT use a proxy (China-mainland sites).
// These sites are either blocked by GFW reversely (proxy IP gets flagged)
// or simply don't need a proxy and work better with a direct connection.
var noProxyDomains = []string{
	// --- video platforms ---
	"bilibili.com",
	"b23.tv",           // B站短链
	"bilivideo.com",
	"v.douyin.com",     // 抖音短链
	"douyin.com",
	"iesdouyin.com",
	"xiaohongshu.com",
	"xhslink.com",      // 小红书短链
	"kuaishou.com",
	"gifshow.com",
	"weixin.qq.com",    // 微信视频号
	"weixinbridge.com",
	"ixigua.com",       // 西瓜视频

	// --- file hosting / direct links typically on CN servers ---
	"yqrii5.org",       // Anna's Archive mirror (CN accessible)
	"annas-archive.org",
	"z-lib.gs",
	"zlibraryglobal.com",

	// --- CDNs / generic ---
	"cn",
	"aliyuncs.com",
	"myqcloud.com",
	"bdstatic.com",
	"hdslb.com",        // B站 CDN
}

// ShouldUseProxy decides whether to route a URL through the proxy based on
// its domain. China-mainland sites bypass the proxy to avoid 412/rate-limit
// issues; everything else (YouTube, Twitter, Instagram, TikTok international,
// etc.) goes through the proxy.
func ShouldUseProxy(rawURL string, proxy string) bool {
	if proxy == "" {
		return false
	}
	host := extractHost(rawURL)
	if host == "" {
		return true // unknown host — default to proxy
	}
	host = strings.ToLower(host)
	for _, d := range noProxyDomains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return false
		}
	}
	return true
}

// extractHost parses a URL and returns the hostname (without port).
func extractHost(rawURL string) string {
	// Add scheme if missing so url.Parse works correctly.
	s := strings.TrimSpace(rawURL)
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// NeedsCookies returns true if the URL's domain is known to require cookies
// for yt-dlp to work (e.g. Douyin, Xiaohongshu).
func NeedsCookies(rawURL string) bool {
	host := extractHost(rawURL)
	if host == "" {
		return false
	}
	host = strings.ToLower(host)
	cookieDomains := []string{
		"douyin.com",
		"v.douyin.com",
		"iesdouyin.com",
		"xiaohongshu.com",
		"xhslink.com",
		"bilibili.com",
		"b23.tv",
		"bilivideo.com",
		"tiktok.com",
	}
	for _, d := range cookieDomains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}
