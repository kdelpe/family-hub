package contacts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		in      contact
		want    *contact
		wantErr string
	}{
		{name: "an email", in: contact{Kind: "email", Value: " maya@example.com ", Visibility: "family", Label: new(" Personal ")},
			want: &contact{Kind: "email", Value: "maya@example.com", Visibility: "family", Label: new("Personal")}},
		{name: "a blank label is none", in: contact{Kind: "other", Value: "Ask at church", Visibility: "hidden", Label: new("  ")},
			want: &contact{Kind: "other", Value: "Ask at church", Visibility: "hidden"}},
		{name: "a phone", in: contact{Kind: "phone", Value: "+1 (555) 010-0000", Visibility: "on_request"},
			want: &contact{Kind: "phone", Value: "+1 (555) 010-0000", Visibility: "on_request"}},
		{name: "an address over lines", in: contact{Kind: "address", Value: "12 Rue Pavée\r\nJacmel", Visibility: "hidden"},
			want: &contact{Kind: "address", Value: "12 Rue Pavée\nJacmel", Visibility: "hidden"}},
		{name: "a WhatsApp number", in: contact{Kind: "social", Platform: new("whatsapp"), Value: "+509 3400 0000", Visibility: "on_request"},
			want: &contact{Kind: "social", Platform: new("whatsapp"), Value: "+509 3400 0000", Visibility: "on_request"}},
		{name: "a handle", in: contact{Kind: "social", Platform: new("instagram"), Value: "@maya.p", Visibility: "family"},
			want: &contact{Kind: "social", Platform: new("instagram"), Value: "@maya.p", Visibility: "family"}},
		{name: "a link on the platform", in: contact{Kind: "social", Platform: new("x"), Value: "https://www.twitter.com/maya", Visibility: "family"},
			want: &contact{Kind: "social", Platform: new("x"), Value: "https://www.twitter.com/maya", Visibility: "family"}},
		{name: "anything for another platform", in: contact{Kind: "social", Platform: new("other"), Value: "maya on Signal", Visibility: "hidden"},
			want: &contact{Kind: "social", Platform: new("other"), Value: "maya on Signal", Visibility: "hidden"}},

		{name: "an unknown kind", in: contact{Kind: "fax", Value: "1", Visibility: "hidden"}, wantErr: "kind must be"},
		{name: "social without a platform", in: contact{Kind: "social", Value: "maya", Visibility: "hidden"}, wantErr: "a platform goes with kind social"},
		{name: "a platform on an email", in: contact{Kind: "email", Platform: new("x"), Value: "maya@example.com", Visibility: "hidden"}, wantErr: "a platform goes with kind social"},
		{name: "an unknown platform", in: contact{Kind: "social", Platform: new("myspace"), Value: "maya", Visibility: "hidden"}, wantErr: "platform must be"},
		{name: "a phone listed for the family", in: contact{Kind: "phone", Value: "+1 555 010 0000", Visibility: "family"},
			wantErr: "visibility must be hidden or on_request"},
		{name: "WhatsApp listed for the family", in: contact{Kind: "social", Platform: new("whatsapp"), Value: "+1 555 010 0000", Visibility: "family"},
			wantErr: "visibility must be hidden or on_request"},
		{name: "an email on request", in: contact{Kind: "email", Value: "maya@example.com", Visibility: "on_request"}, wantErr: "visibility must be hidden or family"},
		{name: "a blank value", in: contact{Kind: "other", Value: "  ", Visibility: "hidden"}, wantErr: "can't be blank"},
		{name: "a line break outside an address", in: contact{Kind: "other", Value: "a\nb", Visibility: "hidden"}, wantErr: "isn't allowed"},
		{name: "a NUL in an address", in: contact{Kind: "address", Value: "a\x00b", Visibility: "hidden"}, wantErr: "isn't allowed"},
		{name: "a value too long", in: contact{Kind: "other", Value: strings.Repeat("a", maxValueLength+1), Visibility: "hidden"}, wantErr: "longer than 200"},
		{name: "a label over two lines", in: contact{Kind: "other", Value: "a", Label: new("a\nb"), Visibility: "hidden"}, wantErr: "the label must be one line"},
		{name: "an email with a name", in: contact{Kind: "email", Value: "Maya <maya@example.com>", Visibility: "hidden"}, wantErr: "an email address"},
		{name: "a phone with letters", in: contact{Kind: "phone", Value: "call me", Visibility: "hidden"}, wantErr: "a phone number"},
		{name: "a phone too short", in: contact{Kind: "phone", Value: "1234", Visibility: "hidden"}, wantErr: "a phone number"},
		{name: "a link elsewhere", in: contact{Kind: "social", Platform: new("instagram"), Value: "https://evil.example.com/maya", Visibility: "family"},
			wantErr: "a handle or an https link"},
		{name: "a lookalike host", in: contact{Kind: "social", Platform: new("instagram"), Value: "https://notinstagram.com/maya", Visibility: "family"},
			wantErr: "a handle or an https link"},
		{name: "a plain-http link", in: contact{Kind: "social", Platform: new("instagram"), Value: "http://instagram.com/maya", Visibility: "family"},
			wantErr: "a handle or an https link"},
		{name: "a javascript link", in: contact{Kind: "social", Platform: new("youtube"), Value: "javascript:alert(1)", Visibility: "family"},
			wantErr: "a handle or an https link"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.in
			err := got.normalize()

			if tt.wantErr != "" {
				require.ErrorIs(t, err, ErrInvalidContact)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, *tt.want, got)
		})
	}
}
