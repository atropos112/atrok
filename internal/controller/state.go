package controller

import (
	"sync"
	"time"

	"github.com/atropos112/atrok.git/types"
)

var (
	State      = make(map[string]*AppBundleState)
	StateMutex = &sync.Mutex{}
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
func RegisterStateIfNotAlreadyRegistered(app types.AppId) error {
	StateMutex.Lock()
	defer StateMutex.Unlock()

	_, ok := State[app.ID()]

	// If state doesn't exists in the cache, register it then get it again from the cache and then return it.
	if !ok {
		currentSpecHash, err := app.GetSpecHash()
		if err != nil {
			return err
		}

		// Initialize the state of the appbundle state object
		state := AppBundleState{
			SpecHash:           currentSpecHash,
			LastReconciliation: time.Unix(0, 0),
			NextBackoffInSec:   10,
		}

		State[app.ID()] = &state
		return nil
	}

	return StateAlreadyRegisteredError{
		objectID: app.ID(),
	}
}

func UpdateState(app types.AppId) error {
	StateMutex.Lock()
	defer StateMutex.Unlock()

	val, ok := State[app.ID()]

	if !ok {
		return StateNotRegisteredError{objectID: app.ID()}
	}

	currentSpecHash, err := app.GetSpecHash()
	if err != nil {
		return err
	}

	val.SpecHash = currentSpecHash
	val.LastReconciliation = time.Now()

	State[app.ID()] = val
	return nil
}

func GetState(app types.AppId) (*AppBundleState, error) {
	StateMutex.Lock()
	defer StateMutex.Unlock()

	val, ok := State[app.ID()]
	if !ok {
		return nil, StateNotRegisteredError{objectID: app.ID()}
	}

	return val, nil
}

func StateNeedsUpdating(app types.AppId, alreadyRegistered bool) (bool, error) {
	if !alreadyRegistered {
		return true, nil
	}

	// No need to lock here as GetState locks the mutex by itself.
	state, err := GetState(app)
	if err != nil {
		return false, err
	}

	currentSpecHash, err := app.GetSpecHash()
	if err != nil {
		return false, err
	}

	anyRecon, _ := app.GetLastReconciliation()

	if state.SpecHash != currentSpecHash ||
		!anyRecon ||
		time.Now().Unix()-state.LastReconciliation.Unix() > 30 {
		return true, nil
	}

	return false, nil
}
