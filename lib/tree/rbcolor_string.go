// Code generated by "stringer -type=RBColor"; DO NOT EDIT.

package tree

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Black-0]
	_ = x[Red-1]
}

const _RBColor_name = "BlackRed"

var _RBColor_index = [...]uint8{0, 5, 8}

func (i RBColor) String() string {
	if i >= RBColor(len(_RBColor_index)-1) {
		return "RBColor(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _RBColor_name[_RBColor_index[i]:_RBColor_index[i+1]]
}
