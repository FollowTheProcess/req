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
	_ = x[URL-5]
	_ = x[Header-6]
	_ = x[Body-7]
	_ = x[Ident-8]
	_ = x[RequestSeparator-9]
	_ = x[At-10]
	_ = x[Eq-11]
	_ = x[Colon-12]
	_ = x[LeftAngle-13]
	_ = x[RightAngle-14]
	_ = x[HTTPVersion-15]
	_ = x[MethodGet-16]
	_ = x[MethodHead-17]
	_ = x[MethodPost-18]
	_ = x[MethodPut-19]
	_ = x[MethodDelete-20]
	_ = x[MethodConnect-21]
	_ = x[MethodPatch-22]
	_ = x[MethodOptions-23]
	_ = x[MethodTrace-24]
}

const _Kind_name = "EOFErrorCommentTextNumberURLHeaderBodyIdentRequestSeparatorAtEqColonLeftAngleRightAngleHTTPVersionMethodGetMethodHeadMethodPostMethodPutMethodDeleteMethodConnectMethodPatchMethodOptionsMethodTrace"

var _Kind_index = [...]uint8{0, 3, 8, 15, 19, 25, 28, 34, 38, 43, 59, 61, 63, 68, 77, 87, 98, 107, 117, 127, 136, 148, 161, 172, 185, 196}

func (i Kind) String() string {
	if i < 0 || i >= Kind(len(_Kind_index)-1) {
		return "Kind(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Kind_name[_Kind_index[i]:_Kind_index[i+1]]
}
