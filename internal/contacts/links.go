package contacts

import (
	"net/url"
	"regexp"
	"strings"
)

type platform struct {
	hosts []string
	base  string
	// A path that isn't someone's own page (a forwarder, a post) never becomes a link (§5.2).
	profile     *regexp.Regexp
	handle      func(h string) string
	notProfiles map[string]bool
	numbered    bool
}

var (
	handlePattern = regexp.MustCompile(`^@?([A-Za-z0-9._-]{1,100})$`)
	hasAlnum      = regexp.MustCompile(`[A-Za-z0-9]`)

	platforms = map[string]platform{
		"instagram": {
			hosts: []string{"instagram.com"}, base: "https://www.instagram.com",
			profile: regexp.MustCompile(`^/([A-Za-z0-9._]{1,100})/?$`), handle: func(h string) string { return "/" + h },
			notProfiles: set("explore", "accounts", "direct", "p", "reel", "reels", "stories", "linkshim", "about", "legal"),
		},
		"facebook": {
			hosts: []string{"facebook.com"}, base: "https://www.facebook.com",
			profile: regexp.MustCompile(`^/(?:[A-Za-z0-9.-]{1,100}|profile\.php\?id=[0-9]{1,30}|people/(?:[A-Za-z0-9_-]|%[0-9A-Fa-f]{2}){1,100}/[0-9]{1,30})/?$`),
			handle:  func(h string) string { return "/" + h },
			notProfiles: set("home", "login", "search", "groups", "events", "pages", "watch", "marketplace", "share",
				"sharer", "dialog", "flx", "settings", "messages", "notifications", "people", "reel", "reels", "stories",
				"friends", "gaming", "help", "privacy", "business"),
			numbered: true,
		},
		"tiktok": {
			hosts: []string{"tiktok.com"}, base: "https://www.tiktok.com",
			profile: regexp.MustCompile(`^/@[A-Za-z0-9._]{1,100}/?$`), handle: func(h string) string { return "/@" + h },
		},
		"linkedin": {
			hosts: []string{"linkedin.com"}, base: "https://www.linkedin.com",
			profile: regexp.MustCompile(`^/in/(?:[A-Za-z0-9_-]|%[0-9A-Fa-f]{2}){1,100}/?$`), handle: func(h string) string { return "/in/" + h },
		},
		"x": {
			hosts: []string{"x.com", "twitter.com"}, base: "https://x.com",
			profile: regexp.MustCompile(`^/[A-Za-z0-9_]{1,100}/?$`), handle: func(h string) string { return "/" + h },
			notProfiles: set("home", "search", "explore", "i", "intent", "settings", "messages", "notifications", "share",
				"login", "signup", "compose"),
		},
		"youtube": {
			hosts: []string{"youtube.com"}, base: "https://www.youtube.com",
			profile: regexp.MustCompile(`^/(?:@[A-Za-z0-9._-]{1,100}|channel/[A-Za-z0-9_-]{1,100}|c/[A-Za-z0-9._-]{1,100}|user/[A-Za-z0-9._-]{1,100})/?$`),
			handle:  func(h string) string { return "/@" + h },
		},
	}
)

// ProfileLink is the https link to someone's page on a known platform, built from a handle or from a link to
// their profile there; nil for anything else, which is then shown as text and never linked.
func ProfileLink(platformName *string, value string) *string {
	if platformName == nil {
		return nil
	}
	p, ok := platforms[*platformName]
	if !ok {
		return nil
	}
	if m := handlePattern.FindStringSubmatch(value); m != nil {
		// "www.instagram.com" typed without https is the site, not someone's handle.
		if onHost(strings.ToLower(m[1]), p.hosts) {
			return nil
		}
		return profilePath(p, p.handle(m[1]))
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return nil
	}
	if !onHost(strings.ToLower(u.Hostname()), p.hosts) {
		return nil
	}
	path := u.EscapedPath()
	// Tracking like ?igsh= is dropped; only Facebook's numbered profile needs its query.
	if p.numbered && strings.EqualFold(strings.TrimSuffix(path, "/"), "/profile.php") {
		path = "/profile.php?id=" + url.QueryEscape(u.Query().Get("id"))
	}
	return profilePath(p, path)
}

func profilePath(p platform, path string) *string {
	segments := strings.Split(strings.Trim(strings.SplitN(path, "?", 2)[0], "/"), "/")
	only := strings.ToLower(segments[0])
	numbered := p.numbered && strings.HasPrefix(path, "/profile.php?id=")
	page := len(segments) == 1 && (p.notProfiles[only] || (strings.HasSuffix(only, ".php") && !numbered))
	// Dots, plain or encoded, would resolve to another page.
	dots := !hasAlnum.MatchString(segments[len(segments)-1]) || strings.Contains(strings.ToLower(path), "%2e")
	if !p.profile.MatchString(path) || dots || page {
		return nil
	}
	link := p.base + path
	return &link
}

func set(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// onHost allows the site and its www/m/mobile/web subdomains, never others like l.facebook.com, a forwarder.
func onHost(host string, hosts []string) bool {
	for _, h := range hosts {
		for _, sub := range []string{"", "www.", "m.", "mobile.", "web."} {
			if host == sub+h {
				return true
			}
		}
	}
	return false
}
