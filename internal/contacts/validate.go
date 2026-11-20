package contacts

import (
	"fmt"
	"net/mail"
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
	phonePattern = regexp.MustCompile(`^\+?[0-9 ().-]+$`)
)

// normalize checks a row before it's saved; checkLink is false for an edit that keeps an older row as it was.
func (c *Contact) normalize(checkLink bool) error {
	if !slices.Contains([]string{"email", "phone", "address", "social", "other"}, c.Kind) {
		return invalid("kind must be email, phone, address, social or other")
	}
	if (c.Kind == "social") != (c.Platform != nil) {
		return invalid("a platform goes with kind social, and only with it")
	}
	if c.Platform != nil && *c.Platform != "whatsapp" && *c.Platform != "other" && !isLinked(*c.Platform) {
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
	c.Value = cleanValue(c.Value)
	return c.checkValue(checkLink)
}

// visibilities are the two a kind allows: phones, WhatsApp and addresses are asked for, never listed (§5.2).
func visibilities(c *Contact) []string {
	if c.Kind == "phone" || c.Kind == "address" || (c.Platform != nil && *c.Platform == "whatsapp") {
		return []string{"hidden", "on_request"}
	}
	return []string{"hidden", "family"}
}

func (c *Contact) checkValue(checkLink bool) error {
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
	case checkLink && c.Platform != nil && isLinked(*c.Platform):
		if ProfileLink(c.Platform, c.Value) == nil {
			return invalid("the value must be a handle or an https link to a profile on that platform")
		}
	}
	return nil
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

func cleanValue(v string) string {
	return strings.TrimSpace(strings.ReplaceAll(v, "\r\n", "\n"))
}

func isLinked(platform string) bool {
	_, ok := platforms[platform]
	return ok
}
