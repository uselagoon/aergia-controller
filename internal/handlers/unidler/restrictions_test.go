package unidler

import (
	"testing"
)

func TestUnidler_checkAgents(t *testing.T) {

	type args struct {
		requestUserAgent string
		blockedAgents    []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test1",
			args: args{
				blockedAgents: []string{
					"@(example|internal).test.?$",
				},
				requestUserAgent: "This is a bot, complaints to: complain@example.test.",
			},
			want: true,
		},
		{
			name: "test1",
			args: args{
				blockedAgents: []string{
					"@(example|internal).test.?$",
				},
				requestUserAgent: "",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkAgents(tt.args.blockedAgents, tt.args.requestUserAgent); got != tt.want {
				t.Errorf("checkAgents() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkIPList(t *testing.T) {
	type args struct {
		allowList     []string
		xForwardedFor []string
		trueClientIP  string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test1",
			args: args{
				allowList: []string{
					"1.2.3.4",
				},
				xForwardedFor: []string{
					"1.2.3.4",
					"172.168.0.1",
				},
				trueClientIP: "",
			},
			want: true,
		},
		{
			name: "test2",
			args: args{
				allowList: []string{
					"1.2.3.5",
				},
				xForwardedFor: []string{
					"1.2.3.4",
					"172.168.0.1",
				},
				trueClientIP: "",
			},
			want: false,
		},
		{
			name: "test2",
			args: args{
				allowList: []string{
					"1.2.3.5",
				},
				xForwardedFor: []string{
					"1.2.3.4",
					"172.168.0.1",
				},
				trueClientIP: "1.2.3.5",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkIPList(tt.args.allowList, tt.args.xForwardedFor, tt.args.trueClientIP); got != tt.want {
				t.Errorf("checkIPList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnidler_checkAccess(t *testing.T) {
	type fields struct {
		AllowedUserAgents []string
		BlockedUserAgents []string
		AllowedIPs        []string
		BlockedIPs        []string
	}
	type args struct {
		nsannotations map[string]string
		annotations   map[string]string
		userAgent     string
		trueClientIP  string
		xForwardedFor []string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "test1 - standard request",
			args: args{
				annotations:   nil,
				userAgent:     "Mozilla/5.0 (Macintosh; Intel Mac OS X 13_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36",
				trueClientIP:  "1.2.3.4",
				xForwardedFor: nil,
			},
			fields: fields{
				BlockedUserAgents: nil,
				BlockedIPs:        nil,
				AllowedIPs:        nil,
			},
			want: true,
		},
		{
			name: "test2 - blocked agent global",
			args: args{
				annotations:   nil,
				userAgent:     "This is a bot, complaints to: complain@example.test.",
				trueClientIP:  "1.2.3.4",
				xForwardedFor: nil,
			},
			fields: fields{
				BlockedUserAgents: []string{
					"@(example).test.?$",
					"@(internal).test.?$",
				},
				BlockedIPs: nil,
				AllowedIPs: nil,
			},
			want: false,
		},
		{
			name: "test3 - blocked agent annotation",
			args: args{
				annotations: map[string]string{
					"idling.lagoon.sh/blocked-agents": "@(example).test.?$,@(internal).test.?$",
				},
				userAgent:     "This is a bot, complaints to: complain@example.test.",
				trueClientIP:  "1.2.3.4",
				xForwardedFor: nil,
			},
			fields: fields{
				BlockedUserAgents: nil,
				BlockedIPs:        nil,
				AllowedIPs:        nil,
			},
			want: false,
		},
		{
			name: "test4 - blocked ip global",
			args: args{
				annotations:   nil,
				userAgent:     "This is a bot, complaints to: complain@example.test.",
				trueClientIP:  "1.2.3.4",
				xForwardedFor: nil,
			},
			fields: fields{
				BlockedUserAgents: nil,
				BlockedIPs: []string{
					"1.2.3.4",
				},
				AllowedIPs: nil,
			},
			want: false,
		},
		{
			name: "test5 - blocked ip annotation",
			args: args{
				annotations: map[string]string{
					"idling.lagoon.sh/ip-block-list": "1.2.3.4",
				},
				userAgent:     "This is a bot, complaints to: complain@example.test.",
				trueClientIP:  "1.2.3.4",
				xForwardedFor: nil,
			},
			fields: fields{
				BlockedUserAgents: nil,
				BlockedIPs:        nil,
				AllowedIPs:        nil,
			},
			want: false,
		},
		{
			name: "test6 - allowed ip global",
			args: args{
				annotations:   nil,
				userAgent:     "Mozilla/5.0 (Macintosh; Intel Mac OS X 13_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36",
				trueClientIP:  "1.2.3.4",
				xForwardedFor: nil,
			},
			fields: fields{
				BlockedUserAgents: nil,
				BlockedIPs:        nil,
				AllowedIPs: []string{
					"1.2.3.4",
				},
			},
			want: true,
		},
		{
			name: "test7 - allowed ip annotation",
			args: args{
				annotations: map[string]string{
					"idling.lagoon.sh/ip-allow-list": "1.2.3.4",
				},
				userAgent:     "Mozilla/5.0 (Macintosh; Intel Mac OS X 13_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36",
				trueClientIP:  "1.2.3.4",
				xForwardedFor: nil,
			},
			fields: fields{
				BlockedUserAgents: nil,
				BlockedIPs:        nil,
				AllowedIPs:        nil,
			},
			want: true,
		},
		{
			name: "test8 - allowed agent global",
			args: args{
				annotations:   nil,
				userAgent:     "This is not a bot, don't complaint to: complain@example.test.",
				trueClientIP:  "1.2.3.4",
				xForwardedFor: nil,
			},
			fields: fields{
				AllowedUserAgents: []string{
					"@(example).test.?$",
					"@(internal).test.?$",
				},
				BlockedUserAgents: nil,
				BlockedIPs:        nil,
				AllowedIPs:        nil,
			},
			want: true,
		},
		{
			name: "test9 - allowed agent annotation",
			args: args{
				annotations: map[string]string{
					"idling.lagoon.sh/allowed-agents": "@(example).test.?$,@(internal).test.?$",
				},
				userAgent:     "This is not a bot, don't complaint to: complain@example.test.",
				trueClientIP:  "1.2.3.4",
				xForwardedFor: nil,
			},
			fields: fields{
				AllowedUserAgents: nil,
				BlockedUserAgents: nil,
				BlockedIPs:        nil,
				AllowedIPs:        nil,
			},
			want: true,
		},
		{
			name: "test10 - allowed agent blocked ip global",
			args: args{
				annotations:   nil,
				userAgent:     "This is not a bot, don't complaint to: complain@example.test.",
				trueClientIP:  "1.2.3.4",
				xForwardedFor: nil,
			},
			fields: fields{
				AllowedUserAgents: []string{
					"@(example).test.?$",
					"@(internal).test.?$",
				},
				BlockedUserAgents: nil,
				BlockedIPs: []string{
					"1.2.3.4",
				},
				AllowedIPs: nil,
			},
			want: false,
		},
		{
			name: "test11 - allowed agent blocked ip annotation",
			args: args{
				annotations: map[string]string{
					"idling.lagoon.sh/allowed-agents": "@(example).test.?$,@(internal).test.?$",
					"idling.lagoon.sh/ip-block-list":  "1.2.3.4",
				},
				userAgent:     "This is not a bot, don't complaint to: complain@example.test.",
				trueClientIP:  "1.2.3.4",
				xForwardedFor: nil,
			},
			fields: fields{
				AllowedUserAgents: nil,
				BlockedUserAgents: nil,
				BlockedIPs:        nil,
				AllowedIPs:        nil,
			},
			want: false,
		},
		{
			name: "test12 - allowed ip blocked agent global",
			args: args{
				annotations:   nil,
				userAgent:     "This is not a bot, don't complaint to: complain@example.test.",
				trueClientIP:  "1.2.3.4",
				xForwardedFor: nil,
			},
			fields: fields{
				AllowedUserAgents: nil,
				BlockedUserAgents: []string{
					"@(example).test.?$",
					"@(internal).test.?$",
				},
				BlockedIPs: nil,
				AllowedIPs: []string{
					"1.2.3.4",
				},
			},
			want: true,
		},
		{
			name: "test13 - allowed ip blocked agent annotation",
			args: args{
				annotations: map[string]string{
					"idling.lagoon.sh/blocked-agents": "@(example).test.?$,@(internal).test.?$",
					"idling.lagoon.sh/ip-allow-list":  "1.2.3.4",
				},
				userAgent:     "This is not a bot, don't complaint to: complain@example.test.",
				trueClientIP:  "1.2.3.4",
				xForwardedFor: nil,
			},
			fields: fields{
				AllowedUserAgents: nil,
				BlockedUserAgents: nil,
				BlockedIPs:        nil,
				AllowedIPs:        nil,
			},
			want: true,
		},
		{
			name: "test15 - allowed ip blocked agent namespace annotation",
			args: args{
				nsannotations: map[string]string{
					"idling.lagoon.sh/blocked-agents": "@(example).test.?$,@(internal).test.?$",
					"idling.lagoon.sh/ip-allow-list":  "1.2.3.4",
				},
				userAgent:     "This is not a bot, don't complaint to: complain@example.test.",
				trueClientIP:  "1.2.3.4",
				xForwardedFor: nil,
			},
			fields: fields{
				AllowedUserAgents: nil,
				BlockedUserAgents: nil,
				BlockedIPs:        nil,
				AllowedIPs:        nil,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Unidler{
				AllowedUserAgents: tt.fields.AllowedUserAgents,
				BlockedUserAgents: tt.fields.BlockedUserAgents,
				AllowedIPs:        tt.fields.AllowedIPs,
				BlockedIPs:        tt.fields.BlockedIPs,
			}
			if got := h.checkAccess(tt.args.nsannotations, tt.args.annotations, tt.args.userAgent, tt.args.trueClientIP, tt.args.xForwardedFor); got != tt.want {
				t.Errorf("Unidler.checkAccess() = %v, want %v", got, tt.want)
			}
		})
	}
}
