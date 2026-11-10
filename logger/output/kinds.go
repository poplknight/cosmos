package output

// Kind identifies an output backend. It is the value of the "kind" field in
// configuration and in OutputConfig, so consumers should use these constants
// instead of writing the raw strings.
type Kind string

const (
	// KindStderr writes log lines to the process stderr.
	KindStderr Kind = "stderr"
	// KindFile writes log lines to a file with lumberjack rotation.
	KindFile Kind = "file"
	// KindOutputs aggregates several outputs into one logical destination.
	KindOutputs Kind = "outputs"
)
