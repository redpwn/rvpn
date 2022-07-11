package main

func ErrorResponse(message string) Error {
	ret := Error{}
	ret.Error.Message = message

	return ret
}
