package types

// SudoCommand is a privileged command request.
type SudoCommand struct {
	Slug    string `json:"slug"`
	Command string `json:"command"`
}
