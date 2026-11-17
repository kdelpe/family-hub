package contacts

import (
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxLabelLength   = 40
	maxValueLength   = 200
	maxAddressLength = 500
)

var (
	phonePattern  = regexp.MustCompile(`^\+?[0-9 ().-]+$`)
	handlePattern = regexp.MustCompile(`^@?[A-Za-z0-9._-]{1,100}$`)
	// Links are only ever built on these hosts, so a stored URL can't point anywhere else (§5.2).
	platformHosts = map[string][]string{
		"instagram": {"instagram.com"},
		"facebook":  {"facebook.com"},
		"tiktok":    {"tiktok.com"},
		"linkedin":  {"linkedin.com"},
		"x":         {"x.com", "twitter.com"},
		"youtube":   {"youtube.com", "youtu.be"},
	}
)

func (c *Contact) normalize() error {
	if !slices.Contains([]string{"email", "phone", "address", "social", "other"}, c.Kind) {
		return invalid("kind must be email, phone, address, social or other")
	}
	if (c.Kind == "social") != (c.Platform != nil) {
		return invalid("a platform goes with kind social, and only with it")
	}
	if c.Platform != nil && *c.Platform != "whatsapp" && *c.Platform != "other" && platformHosts[*c.Platform] == nil {
		return invalid("platform must be whatsapp, instagram, facebook, tiktok, linkedin, x, youtube or other")
	}
	if want := visibilities(c); !slices.Contains(want, c.Visibility) {
		return invalid(fmt.Sprintf("visibility must be %s or %s for this kind", want[0], want[1]))
	}
	if c.Label != nil {
		if t := strings.TrimSpace(*c.Label); t == "" {
			c.Label = nil
		} else {
			c.Label = &t
		}
	}
	if c.Label != nil && (!utf8.ValidString(*c.Label) || hasControl(*c.Label, false) || utf8.RuneCountInString(*c.Label) > maxLabelLength) {
		return invalid(fmt.Sprintf("the label must be one line of at most %d characters", maxLabelLength))
	}
	c.Value = strings.TrimSpace(strings.ReplaceAll(c.Value, "\r\n", "\n"))
	return c.checkValue()
}

// visibilities are the two a kind allows: phones, WhatsApp and addresses are asked for, never listed (§5.2).
func visibilities(c *Contact) []string {
	if c.Kind == "phone" || c.Kind == "address" || (c.Platform != nil && *c.Platform == "whatsapp") {
		return []string{"hidden", "on_request"}
	}
	return []string{"hidden", "family"}
}

func (c *Contact) checkValue() error {
	address := c.Kind == "address"
	limit := maxValueLength
	if address {
		limit = maxAddressLength
	}
	switch {
	case c.Value == "":
		return invalid("the value can't be blank")
	case !utf8.ValidString(c.Value) || hasControl(c.Value, address):
		return invalid("the value contains a character that isn't allowed")
	case utf8.RuneCountInString(c.Value) > limit:
		return invalid(fmt.Sprintf("the value is longer than %d characters", limit))
	}
	switch {
	case c.Kind == "email":
		if a, err := mail.ParseAddress(c.Value); err != nil || a.Address != c.Value {
			return invalid("the value must be an email address like name@example.com")
		}
	case c.Kind == "phone" || (c.Platform != nil && *c.Platform == "whatsapp"):
		if !phonePattern.MatchString(c.Value) || countDigits(c.Value) < 5 || countDigits(c.Value) > 15 {
			return invalid("the value must be a phone number, like +1 555 010 0000")
		}
	case c.Platform != nil && platformHosts[*c.Platform] != nil:
		if !handlePattern.MatchString(c.Value) && !onPlatform(c.Value, platformHosts[*c.Platform]) {
			return invalid("the value must be a handle or an https link on that platform")
		}
	}
	return nil
}

func onPlatform(raw string, hosts []string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range hosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

// hasControl refuses control characters and NUL; an address may span lines.
func hasControl(s string, multiline bool) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) && (!multiline || r != '\n') })
}

func countDigits(s string) int {
	n := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			n++
		}
	}
	return n
}

func invalid(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalidContact, msg)
}
