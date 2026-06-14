package app

//go:generate stringer -type=DoctorKind -linecomment

// DoctorKind classifies which layer a DoctorIssue belongs to.
type DoctorKind int

const (
	// DoctorKindConfig classifies issues found in the configuration layer.
	DoctorKindConfig DoctorKind = iota + 1 // config
	// DoctorKindEventLog classifies issues found in the event stream itself.
	DoctorKindEventLog // event_log
	// DoctorKindProjection classifies issues found in derived projection state.
	DoctorKindProjection // projection
)
