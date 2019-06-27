// Copyright © 2019 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package nvram

import (
	"fmt"
)

type CMOSEntryConfig byte

const (
	CMOSEntryEnum     CMOSEntryConfig = 'e'
	CMOSEntryHex      CMOSEntryConfig = 'h'
	CMOSEntryString   CMOSEntryConfig = 's'
	CMOSEntryReserved CMOSEntryConfig = 'r'
)

type CMOSEntry struct {
	bit       uint
	length    uint
	config    CMOSEntryConfig
	config_id uint
	name      string
}

func (e CMOSEntry) String() string {
	return fmt.Sprintf("%d %d %c %d %s", e.bit, e.length, e.config, e.config_id, e.name)
}

func (e CMOSEntry) Bit() uint {
	return e.bit
}

func (e CMOSEntry) Length() uint {
	return e.length
}

func (e CMOSEntry) Config() CMOSEntryConfig {
	return e.config
}

func (e CMOSEntry) ConfigId() uint {
	return e.config_id
}

func (e CMOSEntry) Name() string {
	return e.name
}

func verifyCMOSEntry(e *CMOSEntry) error {
	// Check if entry is out of range.
	if (e.bit >= (8 * cmosSize)) || ((e.bit + e.length) > (8 * cmosSize)) {
		return fmt.Errorf("CMOS entry %s out of range.", e.name)
	}

	// Check if entry is unaligned and spanning multiple bytes.
	if ((e.bit % 8) > 0) && ((e.bit / 8) != ((e.bit + e.length - 1) / 8)) {
		return fmt.Errorf("CMOS entry %s unaligned spanning multiple bytes.", e.name)
	}

	// Check for a valid config type
	switch e.config {
	case CMOSEntryString:
	case CMOSEntryEnum:
	case CMOSEntryHex:
	case CMOSEntryReserved:
	default:
		return fmt.Errorf("CMOS entry %s has invalid config type.", e.name)
	}

	return nil
}

func verifyCMOSOp(e *CMOSEntry) error {
	// Check if entry is reserved
	if e.config == CMOSEntryReserved {
		return fmt.Errorf("CMOS entry %s is reserved.", e.name)
	}

	// Check if entry is in the RTC area
	if e.bit < (8 * cmosRTCAreaSize) {
		return fmt.Errorf("CMOS entry %s overlaps RTC.", e.name)
	}

	// Check if entry is more than 64 bits and not a string
	if e.length > 64 && e.config != CMOSEntryString {
		return fmt.Errorf("CMOS entry %s too wide.", e.name)
	}

	// Verify the rest of the entry
	return verifyCMOSEntry(e)
}

func (e *CMOSEntry) IsOverlap(e1 *CMOSEntry) bool {
	// Check if this entry overlaps another entry
	return checkAreaOverLap(e.bit, e.length, e1.bit, e1.length)
}

func checkAreaOverLap(s0, l0, s1, l1 uint) bool {
	e0 := s0 + l0 - 1
	e1 := s1 + l1 - 1
	return ((s1 <= e0) && (s0 <= e1))
}
