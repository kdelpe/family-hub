package contacts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProfileLink(t *testing.T) {
	tests := []struct {
		name     string
		platform *string
		value    string
		want     *string
	}{
		{name: "a handle", platform: new("instagram"), value: "@mackenson.jl", want: new("https://www.instagram.com/mackenson.jl")},
		{name: "a TikTok handle", platform: new("tiktok"), value: "mackensonjl", want: new("https://www.tiktok.com/@mackensonjl")},
		{name: "a YouTube handle", platform: new("youtube"), value: "@fritz", want: new("https://www.youtube.com/@fritz")},
		{name: "a LinkedIn handle", platform: new("linkedin"), value: "naika-jl", want: new("https://www.linkedin.com/in/naika-jl")},
		{name: "a profile link", platform: new("facebook"), value: "https://www.facebook.com/fritz.pluviose", want: new("https://www.facebook.com/fritz.pluviose")},
		{name: "a numbered Facebook profile", platform: new("facebook"), value: "https://m.facebook.com/profile.php?id=1000123", want: new("https://www.facebook.com/profile.php?id=1000123")},
		{name: "the old X host", platform: new("x"), value: "https://mobile.twitter.com/naika", want: new("https://x.com/naika")},
		{name: "a profile starting with r", platform: new("instagram"), value: "https://www.instagram.com/rachel", want: new("https://www.instagram.com/rachel")},
		{name: "a YouTube channel", platform: new("youtube"), value: "https://www.youtube.com/channel/UC123", want: new("https://www.youtube.com/channel/UC123")},
		{name: "Facebook's forwarder host", platform: new("facebook"), value: "https://l.facebook.com/l.php?u=https%3A%2F%2Fevil.example.com"},
		{name: "Facebook's forwarder path", platform: new("facebook"), value: "https://www.facebook.com/l.php?u=https%3A%2F%2Fevil.example.com"},
		{name: "LinkedIn's redirect", platform: new("linkedin"), value: "https://www.linkedin.com/redir/redirect?url=https://evil.example.com"},
		{name: "Instagram's link shim", platform: new("instagram"), value: "https://www.instagram.com/linkshim/?u=https://evil.example.com"},
		{name: "YouTube's redirect", platform: new("youtube"), value: "https://www.youtube.com/redirect?q=https://evil.example.com"},
		{name: "a lookalike host", platform: new("facebook"), value: "https://facebook.com.example.com/fritz"},
		{name: "plain http", platform: new("facebook"), value: "http://www.facebook.com/fritz"},
		{name: "a login in the link", platform: new("facebook"), value: "https://a:b@facebook.com/fritz"},
		{name: "dots only", platform: new("instagram"), value: ".."},
		{name: "another platform's link", platform: new("x"), value: "https://www.facebook.com/fritz"},
		{name: "an unknown platform", platform: new("other"), value: "@someone"},
		{name: "WhatsApp, a phone", platform: new("whatsapp"), value: "+1 555 010 0000"},
		{name: "no platform", value: "@someone"},
		{name: "a share link's tracking", platform: new("instagram"), value: "https://www.instagram.com/maya.p/?igsh=abc123", want: new("https://www.instagram.com/maya.p/")},
		{name: "YouTube's share tracking", platform: new("youtube"), value: "https://www.youtube.com/@fritz?si=xyz", want: new("https://www.youtube.com/@fritz")},
		{name: "Facebook's forwarder without a query", platform: new("facebook"), value: "https://www.facebook.com/l.php"},
		{name: "a site page on X", platform: new("x"), value: "https://x.com/home"},
		{name: "Instagram's explore page", platform: new("instagram"), value: "https://www.instagram.com/explore/"},
		{name: "a handle the platform can't have", platform: new("instagram"), value: "maya-p"},
		{name: "a dotted LinkedIn handle", platform: new("linkedin"), value: "a.b"},
		{name: "a handle that's a site page", platform: new("x"), value: "home"},
		{name: "a name that's a site page on another platform", platform: new("instagram"), value: "events", want: new("https://www.instagram.com/events")},
		{name: "a YouTube handle named like a page", platform: new("youtube"), value: "@watch", want: new("https://www.youtube.com/@watch")},
		{name: "X's sign-up page as a handle", platform: new("x"), value: "signup"},
		{name: "X's sign-up page as a link", platform: new("x"), value: "https://x.com/signup"},
		{name: "the bare site as a handle", platform: new("instagram"), value: "www.instagram.com"},
		{name: "Facebook's friends page", platform: new("facebook"), value: "https://www.facebook.com/friends"},
		{name: "a numbered profile on another platform", platform: new("x"), value: "https://x.com/profile.php?id=1"},
		{name: "a Facebook .php page", platform: new("facebook"), value: "https://www.facebook.com/logout.php"},
		{name: "profile.php as a handle", platform: new("facebook"), value: "profile.php"},
		{name: "profile.php without an id", platform: new("facebook"), value: "https://www.facebook.com/Profile.PHP"},
		{name: "a numbered profile in other case", platform: new("facebook"), value: "https://www.facebook.com/Profile.php?id=12", want: new("https://www.facebook.com/profile.php?id=12")},
		{name: "encoded dots in a people link", platform: new("facebook"), value: "https://www.facebook.com/people/%2e%2e/123"},
		{name: "an accented name", platform: new("linkedin"), value: "https://www.linkedin.com/in/nad%C3%A8ge-p", want: new("https://www.linkedin.com/in/nad%C3%A8ge-p")},
		{name: "a link ending in #", platform: new("x"), value: "https://x.com/naika#top", want: new("https://x.com/naika")},
		{name: "a people link", platform: new("facebook"), value: "https://www.facebook.com/people/Maya-Pluviose/1000123", want: new("https://www.facebook.com/people/Maya-Pluviose/1000123")},
		{name: "dots after a YouTube prefix", platform: new("youtube"), value: "https://www.youtube.com/c/.."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ProfileLink(tt.platform, tt.value))
		})
	}
}
