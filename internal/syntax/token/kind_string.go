// Code generated by "stringer -type Kind -linecomment"; DO NOT EDIT.

package token

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[EOF-0]
	_ = x[Error-1]
	_ = x[Comment-2]
	_ = x[Text-3]
	_ = x[Number-4]
	_ = x[Ident-5]
	_ = x[RequestSeparator-6]
	_ = x[At-7]
	_ = x[Eq-8]
	_ = x[Colon-9]
	_ = x[HTTPVersion-10]
	_ = x[MethodGet-11]
	_ = x[MethodHead-12]
	_ = x[MethodPost-13]
	_ = x[MethodPut-14]
	_ = x[MethodDelete-15]
	_ = x[MethodConnect-16]
	_ = x[MethodPatch-17]
	_ = x[MethodOptions-18]
	_ = x[MethodTrace-19]
}

const _Kind_name = "EOFErrorCommentTextNumberIdentRequestSeparatorAtEqColonHTTPVersionMethodGetMethodHeadMethodPostMethodPutMethodDeleteMethodConnectMethodPatchMethodOptionsMethodTrace"

var _Kind_index = [...]uint8{0, 3, 8, 15, 19, 25, 30, 46, 48, 50, 55, 66, 75, 85, 95, 104, 116, 129, 140, 153, 164}

func (i Kind) String() string {
	if i < 0 || i >= Kind(len(_Kind_index)-1) {
		return "Kind(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Kind_name[_Kind_index[i]:_Kind_index[i+1]]
}
