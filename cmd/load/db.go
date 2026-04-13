package load

// The load command delegates entirely to cli.LoadCSV which manages its own
// database connection. No command-level DB interface is needed.
//
//go:generate mockery
