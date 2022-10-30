package main

import (
	"context"
)

// ErrorResponse wraps an error message in the Error format
func ErrorResponse(message string) Error {
	ret := Error{}
	ret.Error.Message = message

	return ret
}

// GetAuthTargetsByPrincipal gets authorized targets by principal
func GetAuthTargetsByPrincipal(ctx context.Context, db *RVPNDatabase, principal string) ([]string, error) {
	// get targets where ACL rules allow principal authUser
	allowedTargets, err := db.getTargetsByPrincipal(ctx, principal)
	if err != nil {
		return nil, err
	}

	// get targets where authUser is the owner
	ownedTargets, err := db.getTargetsByOwner(ctx, principal)
	if err != nil {
		return nil, err
	}

	allowedTargets = append(allowedTargets, ownedTargets...)
	return allowedTargets, nil
}
