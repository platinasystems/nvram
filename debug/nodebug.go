// Copyright © 2019 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

// +build !debug

package debug

const (
	LevelNone = iota
	LevelMSG1 = iota
	LevelMSG2 = iota
	LevelMSG3 = iota
)

func SetDebugLevel(level int) {
}

func Trace(level int, s string, a ...interface{}) {
}
