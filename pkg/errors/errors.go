package errors

import (
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// ErrInvalidParameters indicates invalid parameters specified
	ErrInvalidParameters = 1
	// ErrUnauthorized indicates not authorized
	ErrUnauthorized = 4
)

// AddonError represents application errors for Addon
type AddonError struct {
	Code    int
	Reason  string
	Requeue bool
	//TODO: try out requeue later RequeueAfter int
}

func (e AddonError) Error() string {
	return e.Reason
}

// IsAddonError checks if error is of type AddonError
func IsAddonError(e error) (AddonError, bool) {
	ae, ok := e.(AddonError)
	if ok {
		return ae, true
	}
	return ae, false
}

// InvalidParams returns error of type Invalid params
func InvalidParams(msg string) AddonError {
	return AddonError{
		Code:    ErrInvalidParameters,
		Reason:  fmt.Sprintf("Required parameter %s missing", msg),
		Requeue: false,
	}
}

// IsInvalidParams checks if error is of type InvalidParams
func IsInvalidParams(e error) bool {
	ae, ok := e.(AddonError)
	if ok && ae.Code == ErrInvalidParameters {
		return true
	}

	return false
}

// NotAuthorized is thrown when user is not authorized to make an API call
func NotAuthorized() AddonError {
	return AddonError{
		Code:   ErrUnauthorized,
		Reason: fmt.Sprintf("Not authorized"),
	}
}

// IsNotAuthorized checks if error is of type NotAuthorized
func IsNotAuthorized(e error) bool {
	ae, ok := e.(AddonError)
	if ok && ae.Code == ErrUnauthorized {
		return true
	}

	return false
}

//ProcessError returns an error for the Reconcile loop
func ProcessError(e error) (ctrl.Result, error) {

	ae, ok := IsAddonError(e)
	if !ok {
		return ctrl.Result{}, e
	}

	if ae.Requeue {
		return ctrl.Result{
			Requeue: ae.Requeue,
			//TODO check if we can have retry using this
			//right now this does not work
			//RequeueAfter: time.Duration(ae.RequeueAfter) * time.Second,
		}, e
	}

	return ctrl.Result{}, nil
}
