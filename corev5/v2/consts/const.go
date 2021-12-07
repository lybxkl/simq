package consts

import (
	messagev52 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
)

const (
	// MWC is the multi-level wildcard
	MWC = "#"

	// SWC is the single level wildcard
	SWC = "+"

	// SEP is the topic level separator
	SEP = "/"

	// SYS is the starting character of the system level topics
	//SYS是系统级主题的起始字符
	SYS = "$"

	// Both wildcards
	_WC = "#+"
)

const (
	// MaxQosAllowed is the maximum QOS supported by this server
	MaxQosAllowed = messagev52.QosExactlyOnce
)

const (
	StateCHR byte = iota // Regular character 普通字符
	StateMWC             // Multi-level wildcard 多层次的通配符
	StateSWC             // Single-level wildcard 单层通配符
	StateSEP             // Topic level separator 主题水平分隔符
	StateSYS             // System level topic ($) 系统级主题($)
)
