// Copyright © 2019 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

// +build debug

package debug

import (
	"fmt"
)

const (
	LevelNone = iota
	LevelMSG1 = iota
	LevelMSG2 = iota
	LevelMSG3 = iota
)

var dbgLevel = LevelMSG3

func SetDebugLevel(level int) {
	dbgLevel = level
}

func Trace(level int, s string, a ...interface{}) {
	if level <= dbgLevel {
		fmt.Printf(s, a...)
	}
}
