package unidler

import (
	"regexp"
	"strings"
)

func checkAgents(blockedAgents []string, requestUserAgent string) bool {
	if requestUserAgent != "" {
		for _, ua := range blockedAgents {
			match, _ := regexp.MatchString(ua, requestUserAgent)
			if match {
				return true
			}
		}
	}
	return false
}

func checkIPList(allowList []string, xForwardedFor []string, trueClientIP string) bool {
	var clientIPs []string
	if trueClientIP != "" {
		clientIPs = append(clientIPs, trueClientIP)
	} else {
		clientIPs = xForwardedFor
	}
	for _, ua := range allowList {
		for _, ip := range clientIPs {
			if ua == ip {
				return true
			}
		}
	}
	return false
}

func (h *Unidler) checkAccess(nsannotations map[string]string, annotations map[string]string, userAgent, trueClientIP string, xForwardedFor []string) bool {
	// deal with ip allow/blocks first
	blockedIP := checkIPAnnotations("idling.amazee.io/ip-block-list", trueClientIP, xForwardedFor, h.BlockedIPs, nsannotations, annotations)
	if blockedIP {
		return false
	}
	allowedIP := checkIPAnnotations("idling.amazee.io/ip-allow-list", trueClientIP, xForwardedFor, h.AllowedIPs, nsannotations, annotations)
	if allowedIP {
		return true
	}
	blockedAgent := checkAgentAnnotations("idling.amazee.io/blocked-agents", userAgent, h.BlockedUserAgents, nsannotations, annotations)
	if blockedAgent {
		return false
	}
	allowedAgent := checkAgentAnnotations("idling.amazee.io/allowed-agents", userAgent, h.AllowedUserAgents, nsannotations, annotations)
	if allowedAgent {
		return true
	}
	// else fallthrough
	return true
}

func checkAgentAnnotations(annotation, ua string, g []string, ns, i map[string]string) bool {
	allow := false
	if agents, ok := i[annotation]; ok {
		allow = checkAgents(strings.Split(agents, ","), ua)
	} else {
		// check for namespace annoation
		if agents, ok := ns[annotation]; ok {
			allow = checkAgents(strings.Split(agents, ","), ua)
		} else {
			// check for globals
			if g != nil {
				allow = checkAgents(g, ua)
			}
		}
	}
	return allow
}

func checkIPAnnotations(annotation, tcip string, xff, g []string, ns, i map[string]string) bool {
	allow := false
	if alist, ok := i[annotation]; ok {
		// there is an allow list, we want to deny any requests now unless they are the trueclientip
		// or xforwardedfor if trueclientip is not defined
		allow = checkIPList(strings.Split(alist, ","), xff, tcip)
	} else {
		// check for namespace annoation
		if alist, ok := ns[annotation]; ok {
			allow = checkIPList(strings.Split(alist, ","), xff, tcip)
		} else {
			// check for globals
			if g != nil {
				allow = checkIPList(g, xff, tcip)
			}
		}
	}
	return allow
}
