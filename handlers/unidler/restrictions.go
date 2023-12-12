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

func (h *Unidler) checkAccess(annotations map[string]string, userAgent, trueClientIP string, xForwardedFor []string) bool {
	allowedIP := false
	allowedAgent := false
	blockedIP := false
	blockedAgent := false

	hasIPAllowList := false
	hasAllowedAgentList := false
	hasIPBlockList := false
	hasBlockedAgentList := false

	if alist, ok := annotations["idling.amazee.io/ip-allow-list"]; ok {
		// there is an allow list, we want to deny any requests now unless they are the trueclientip
		// or xforwardedfor if trueclientip is not defined
		hasIPAllowList = true
		allowedIP = checkIPList(strings.Split(alist, ","), xForwardedFor, trueClientIP)
	} else {
		if h.AllowedIPs != nil {
			hasIPAllowList = true
			allowedIP = checkIPList(h.AllowedIPs, xForwardedFor, trueClientIP)
		}
	}

	if blist, ok := annotations["idling.amazee.io/ip-block-list"]; ok {
		// there is a block list, we want to allow any requests now unless they are the trueclientip
		// or xforwardedfor if trueclientip is not defined
		hasIPBlockList = true
		blockedIP = checkIPList(strings.Split(blist, ","), xForwardedFor, trueClientIP)
	} else {
		if h.BlockedIPs != nil {
			hasIPBlockList = true
			blockedIP = checkIPList(h.BlockedIPs, xForwardedFor, trueClientIP)
		}
	}

	// deal with ip allow/blocks first
	if allowedIP && hasIPAllowList {
		return true
	}
	if blockedIP && hasIPBlockList {
		return false
	}

	if agents, ok := annotations["idling.amazee.io/allowed-agents"]; ok {
		hasAllowedAgentList = true
		allowedAgent = checkAgents(strings.Split(agents, ","), userAgent)
	} else {
		if h.AllowedUserAgents != nil {
			hasAllowedAgentList = true
			allowedAgent = checkAgents(h.AllowedUserAgents, userAgent)
		}
	}

	if agents, ok := annotations["idling.amazee.io/blocked-agents"]; ok {
		hasBlockedAgentList = true
		blockedAgent = checkAgents(strings.Split(agents, ","), userAgent)
	} else {
		if h.BlockedUserAgents != nil {
			hasBlockedAgentList = true
			blockedAgent = checkAgents(h.BlockedUserAgents, userAgent)
		}
	}

	if allowedAgent && hasAllowedAgentList {
		return true
	}
	if blockedAgent && hasBlockedAgentList {
		return false
	}
	// else fallthrough
	return true
}
