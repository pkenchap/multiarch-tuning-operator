package common

const SingletonResourceObjectName = "cluster"

type Plugin int

const (
	// MainPlugin checks the core pod placement resources.
	NodeAffinityScoringPluginName Plugin = iota
	// ENoExecPlugin checks the ENoExecEvent resources.
	ExecFormatErrorMonitorPluginName
	// CelArchPlacementPlugin provides CEL-based architecture selection rules.
	CelArchPlacementPluginName
)
