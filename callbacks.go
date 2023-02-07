package steamworks

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"unsafe"
)

type callbackID int
type EResult int

const (
	k_iSteamUserStatsCallbacks callbackID = 1100
	callbackUserStatsReceived             = k_iSteamUserStatsCallbacks + 1
)

const (
	EResultNone EResult = 0
	EResultOK           = 1
	EResultFail         = 2 // Generic failure
)

type hSteamPipe int
type hSteamUser int
type steamApiCall uint64

type Callbacks struct {
	UserStatsReceived func(gameID uint64, result EResult, user CSteamID)
}

type callbackmsg struct {
	user             hSteamUser
	callbackID       callbackID
	callbackData     *byte
	callbackDataSize int
}

type apiCallCompleted struct {
	asyncCall  steamApiCall
	callbackID int
	paramSize  uint32
}

// Dispatch implements a high-level wrapper over the manual dispatch loop for handling Steam callbacks.
// If this is used, RunCallbacks must NOT be used.
type Dispatch struct {
	pipe hSteamPipe
	cb   *Callbacks
}

// NewDispatch creates a new handler for Steam callback dispatch.
// This should be called after Init but before callbacks need to be handled.
func NewDispatch(cb *Callbacks) (*Dispatch, error) {
	if err := manualDispatchInit(); err != nil {
		return nil, err
	}
	pipe, err := getHSteamPipe()
	if err != nil {
		return nil, err
	}

	return &Dispatch{
		pipe: pipe,
		cb:   cb,
	}, nil
}

func (m *Dispatch) ProcessCallbacks() error {
	var hasCallback bool
	var err error
	msg := callbackmsg{}
	for {
		if hasCallback, err = m.pipe.manualDispatchGetNextCallback(&msg); hasCallback && err == nil {
			defer m.pipe.manualDispatchFreeLastCallback()
			r := bytes.NewReader(unsafe.Slice(msg.callbackData, msg.callbackDataSize))
			switch msg.callbackID {
			case callbackUserStatsReceived:
				if m.cb.UserStatsReceived != nil {
					var gameID uint64
					var result EResult
					var user CSteamID
					for _, f := range []any{&gameID, &result, &user} {
						if err := binary.Read(r, binary.BigEndian, f); err != nil {
							return fmt.Errorf("UserStatsReceived: %w", err)
						}
					}
					m.cb.UserStatsReceived(gameID, result, user)
				}
			}
		} else {
			break
		}
	}
	return err
}
