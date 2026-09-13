// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package libbp

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrForbiddenCommand  = errors.New("forbidden command")
	ErrDestructiveAction = errors.New("destructive action refused by client guard")
)

var dangerousCommands = map[string]bool{
	"FLUSHALL": true,
	"FLUSHDB":  true,
	"SHUTDOWN": true,
	"DEBUG":    true,
	"CONFIG":   true,
	"KEYS":     true, // Use SCAN instead
}

// AssertAllowed checks that a command and its arguments obey backplane safety rules
func AssertAllowed(callerID string, cmd string, args ...string) error {
	upperCmd := strings.ToUpper(strings.TrimSpace(cmd))
	if dangerousCommands[upperCmd] {
		return fmt.Errorf("%w: command %s is disabled in agent backplane", ErrForbiddenCommand, upperCmd)
	}

	// Defense-in-depth: Inspect XADD calls
	if upperCmd == "XADD" {
		if len(args) < 1 {
			return fmt.Errorf("%w: XADD requires a target stream key", ErrForbiddenCommand)
		}
		targetKey := strings.ToLower(args[0])

		// If writing to someone else's inbox, ensure no MAXLEN or MINID is passed
		if strings.HasSuffix(targetKey, ":inbox") {
			owner := strings.TrimSuffix(targetKey, ":inbox")
			if owner != strings.ToLower(callerID) {
				for _, arg := range args[1:] {
					upperArg := strings.ToUpper(arg)
					if upperArg == "MAXLEN" || upperArg == "MINID" || upperArg == "LIMIT" {
						return fmt.Errorf("%w: XADD with %s is forbidden on peer inbox %s", ErrDestructiveAction, upperArg, targetKey)
					}
				}
			}
		}
	}

	return nil
}
