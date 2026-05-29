package app

//go:generate stringer -type=DoctorKind -linecomment

// DoctorKind classifies which layer a DoctorIssue belongs to.
//

// DoctorKind classifies which layer a DoctorIssue belongs to.
type DoctorKind int

//nolint:revive // linecomment strings serve as the stringer output; no separate doc needed
const (
	DoctorKindConfig     DoctorKind = iota + 1 // config
	DoctorKindEventLog                         // event_log
	DoctorKindProjection                       // projection
)
