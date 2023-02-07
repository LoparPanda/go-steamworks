package steamworks

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"unsafe"
)

type callbackID int32
type EResult int32

const (
	k_iSteamUserStatsCallbacks callbackID = 1100
	callbackUserStatsReceived             = k_iSteamUserStatsCallbacks + 1
)

const (
	EResultNone EResult = 0
	EResultOK           = 1
	EResultFail         = 2 // Generic failure
)

type hSteamPipe int32
type hSteamUser int32
type steamApiCall uint64

type Callbacks struct {
	UserStatsReceived func(gameID uint64, result EResult, user CSteamID)
}

type callbackmsg struct {
	User             hSteamUser
	CallbackID       callbackID
	CallbackData     uintptr
	CallbackDataSize int32
}

// Space for 32- or 64-bit struct.
const callbackmsgSize = 20

// Dispatch implements a high-level wrapper over the manual dispatch loop for handling Steam callbacks.
// If this is used, RunCallbacks must NOT be used.
type Dispatch struct {
	cb *Callbacks
}

// NewDispatch creates a new handler for Steam callback dispatch.
// This should be called after Init but before callbacks need to be handled.
func NewDispatch(cb *Callbacks) (*Dispatch, error) {
	if err := manualDispatchInit(); err != nil {
		return nil, err
	}
	return &Dispatch{
		cb: cb,
	}, nil
}

func (m *Dispatch) ProcessCallbacks() error {
	pipe, err := getHSteamPipe()
	if err != nil {
		return fmt.Errorf("SteamPipe: %w", err)
	}

	if err := pipe.manualDispatchRunFrame(); err != nil {
		return fmt.Errorf("Steam RunFrame: %w", err)
	}

	var hasCallback bool
	var buf [callbackmsgSize]byte
	msg := callbackmsg{}
	for {
		if hasCallback, err = pipe.manualDispatchGetNextCallback(&buf); hasCallback && err == nil {
			{
				r := bytes.NewReader(buf[:])
				// Temporary message to use proper primitive types.
				tmp := struct {
					User             int32
					CallbackID       int32
					CallbackData     uint64
					CallbackDataSize int32
				}{}
				if err := binary.Read(r, binary.LittleEndian, &tmp); err != nil {
					return err
				}
				msg.User = hSteamUser(tmp.User)
				msg.CallbackID = callbackID(tmp.CallbackID)
				msg.CallbackData = uintptr(tmp.CallbackData)
				msg.CallbackDataSize = tmp.CallbackDataSize
			}
			var cbData []byte
			hdr := (*reflect.SliceHeader)((unsafe.Pointer(&cbData)))
			hdr.Cap = int(msg.CallbackDataSize)
			hdr.Len = int(msg.CallbackDataSize)
			hdr.Data = uintptr(msg.CallbackData)

			r := bytes.NewReader(cbData)
			switch msg.CallbackID {
			case callbackUserStatsReceived:
				if m.cb.UserStatsReceived != nil {
					var gameID uint64
					var result EResult
					var user CSteamID
					for _, f := range []any{&gameID, &result, &user} {
						if err := binary.Read(r, binary.LittleEndian, f); err != nil {
							return fmt.Errorf("UserStatsReceived: %w", err)
						}
					}
					m.cb.UserStatsReceived(gameID, result, user)
				}
			}
			pipe.manualDispatchFreeLastCallback()
		} else {
			break
		}
	}
	return err
}
