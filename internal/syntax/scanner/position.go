package scanner

import "fmt"

// TODO(@FollowTheProcess): Move this and ErrorHandler to the top level syntax package so it can be used across everything

// Position is an arbitrary source file position including file, line
// and column information. It can also express a range of source via StartCol
// and EndCol, this is useful for error reporting.
//
// Position's without filenames are considered invalid, in the case of stdin
// the string "stdin" may be used.
type Position struct {
	Name     string // Filename
	Line     int    // Line number (1 indexed)
	StartCol int    // Start column (1 indexed)
	EndCol   int    // End column (1 indexed), EndCol == StartCol when pointing to a single character
}

// IsValid reports whether the [Position] describes a valid source position.
//
// The rules are:
//
//   - At least Name, Line and StartCol must be set (and non zero)
//   - EndCol cannot be 0, it's only allowed values are StartCol or any number greater than StartCol
func (p Position) IsValid() bool {
	if p.Name == "" || p.Line < 1 || p.StartCol < 1 || p.EndCol < 1 || (p.EndCol >= 1 && p.EndCol < p.StartCol) {
		return false
	}
	return true
}

// String returns a string representation of a [Position].
//
// It is formatted such that most text editors/terminals will be able to support clicking on it
// and navigating to the position.
//
// Depending on which fields are set, the string returned will be different:
//
//   - "file:line:start-end": valid position pointing to a range of text on the line
//   - "file:line:start": valid position pointing to a single character on the line (EndCol == StartCol)
//
// At least Name, Line and StartCol must be present for a valid position, and Line and StarCol must be > 0. If not, an error
// string will be returned.
func (p Position) String() string {
	if !p.IsValid() {
		return fmt.Sprintf(
			"BadPosition: {Name: %q, Line: %d, StartCol: %d, EndCol: %d}",
			p.Name,
			p.Line,
			p.StartCol,
			p.EndCol,
		)
	}

	if p.StartCol == p.EndCol {
		// No range, just a single position
		return fmt.Sprintf("%s:%d:%d", p.Name, p.Line, p.StartCol)
	}

	return fmt.Sprintf("%s:%d:%d-%d", p.Name, p.Line, p.StartCol, p.EndCol)
}
