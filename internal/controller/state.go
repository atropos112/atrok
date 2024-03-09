package controller

import (
	"context"
	"time"

	atro "github.com/atropos112/atrok.git/api/v1alpha1"
	rxhash "github.com/rxwycdh/rxhash"
)

// AppBundleBaseState is the in-memory cached state of an appbundlebase object.
type AppBundleBaseState struct {
	SpecHash           string
	LastReconciliation time.Time
	NextBackoffInSec   int16
}

// AppBundleState is the in-memory cached state of an appbundle object.
type AppBundleState struct {
	SpecHash           string
	LastReconciliation time.Time
	NextBackoffInSec   int16
}

type StateAlreadyRegisteredError struct {
	objectID string
}

type StateNotRegisteredError struct {
	objectID string
}

func (e StateAlreadyRegisteredError) Error() string {
	return "State for " + e.objectID + " is already registered."
}

func (e StateNotRegisteredError) Error() string {
	return "State for " + e.objectID + " is not registered."
}

// GetState returns the state of an appbundle object from the cache. Returns bool if the state was initialized and error if any.
func RegisterStateIfNotAlreadyRegistered(ctx context.Context, ab *atro.AppBundle) error {
	val := ctx.Value(ab.ID())

	// If state doesnt exists in the cache, register it then get it again from the cache and then return it.
	if val == nil {
		currentSpecHash, err := rxhash.HashStruct(ab.Spec)
		if err != nil {
			return err
		}

		// Initialize the state of the appbundle state object
		state := AppBundleState{
			SpecHash:           currentSpecHash,
			LastReconciliation: time.Unix(0, 0),
			NextBackoffInSec:   10,
		}

		ctx = context.WithValue(ctx, ab.ID(), &state)
		return nil
	}

	return StateAlreadyRegisteredError{
		objectID: ab.ID(),
	}
}

func UpdateState(ctx context.Context, ab *atro.AppBundle) error {
	maybeVal := ctx.Value(ab.ID())
	if maybeVal == nil {
		return StateNotRegisteredError{objectID: ab.ID()}
	}
	val := maybeVal.(*AppBundleState)

	currentSpecHash, err := rxhash.HashStruct(ab.Spec)
	if err != nil {
		return err
	}

	val.SpecHash = currentSpecHash
	val.LastReconciliation = time.Now()

	ctx = context.WithValue(ctx, ab.ID(), val)
	return nil
}

func GetState(ctx context.Context, ab *atro.AppBundle) (*AppBundleState, error) {
	maybeVal := ctx.Value(ab.ID())
	if maybeVal == nil {
		return nil, StateNotRegisteredError{objectID: ab.ID()}
	}
	val := maybeVal.(*AppBundleState)

	return val, nil
}

func StateNeedsUpdating(ctx context.Context, ab *atro.AppBundle, alreadyRegistered bool) (bool, error) {
	if !alreadyRegistered {
		return true, nil
	}

	state, err := GetState(ctx, ab)
	if err != nil {
		return false, err
	}

	currentSpecHash, err := rxhash.HashStruct(ab.Spec)
	if err != nil {
		return false, err
	}

	if state.SpecHash != currentSpecHash ||
		ab.Status.LastReconciliation == nil ||
		time.Now().Unix()-state.LastReconciliation.Unix() > 30 {
		return true, nil
	}

	return false, nil
}
