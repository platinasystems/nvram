// Copyright © 2019 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

// Package nvram provides access to the coreboot CMOS option table and allows
// reading, writing and listing CMOS parameters.
package nvram

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/platinasystems/nvram/debug"
	"strings"
	"sync/atomic"
)

var (
	ErrNVRAMAccessInUse = errors.New("nvram: NVRAM is busy.")
	ErrInvalidCMOSIndex = errors.New("nvram: Invalid CMOS index!")
	ErrCMOSNotOpen = errors.New("nvram: CMOS Not Opened")
)

var lockstate uint32

type NVRAM struct {
	CMOS
	*Layout
	modified bool
}

// Open opens NVRAM access.
//
// Calling Open with no parameters will use the machine's coreboot table
// and NVRAM hardware.
//		nv.Open()
// Calling Open with a single layout file name will use the CMOS layout
// text file.
//		nv.Open("cmos_layout")
// If the layout file name ends in .bin the coreboot CMOS layout will be
// read in binary form.
//		nv.Open("cmos_layout.bin")
// If the first argument is empty the machine's coreboot table will be used.
//		nv.Open("")
// Calling Open with a second CMOS memory file name will use the mem mapped
// CMOS file instead of the NVRAM hardware.
//		nv.Open("", "cmos.bin")

func (nv *NVRAM) Open(args ...string) (err error) {
	// Only one NVRAM access is allowed at a time.
	if !atomic.CompareAndSwapUint32(&lockstate, 0, 1) {
		return ErrNVRAMAccessInUse
	}

	// Get file name arguments if they exist.
	var layoutFileName, cmosMemFileName string
	if len(args) > 0 {
		layoutFileName = args[0]
	}
	if len(args) > 1 {
		cmosMemFileName = args[1]
	}

	// Load layout file from machine's Coreboot table, coreboot table binary,
	// or CMOS layout text file.
	if layoutFileName == "" {
		nv.Layout, err = ReadLayoutFromCoreBootTable()
	} else {
		if strings.HasSuffix(layoutFileName, ".bin") {
			nv.Layout, err = ReadLayoutFromCMOSTableBinary(layoutFileName)
		} else {
			nv.Layout, err = ReadLayoutFromTextFile(layoutFileName)
		}
	}

	// If we don't have any CMOS layout return error.
	if err != nil {
		return
	}

	// Open CMOS NVRAM access with hardware access or using a binary file.
	if cmosMemFileName == "" {
		err = nv.CMOS.Open()
	} else {
		err = nv.CMOS.OpenMem(cmosMemFileName)
	}

	// If we don't have any CMOS access return error
	if err != nil {
		return
	}

	// Initialize CMOS with layout checksum
	nv.CMOS.checksum = *nv.Layout.cmosChecksum

	return
}

// Close closes the currently opened CMOS layout and NVRAM access.
// If the CMOS data has been modified a new checksum is calculed and written
// before closing the CMOS access.
func (nv *NVRAM) Close() (err error) {

	defer atomic.StoreUint32(&lockstate, 0)

	if nv.modified {
		debug.Trace(debug.LevelMSG1, "NVRAM Modified computing checksum.\n")
		sum, err := nv.CMOS.ComputeChecksum()
		if err == nil {
			debug.Trace(debug.LevelMSG1, "NVRAM Modified writing checksum %02X.\n", sum)
			err = nv.CMOS.WriteChecksum(sum)
			if err == nil {
				debug.Trace(debug.LevelMSG1, "NVRAM cheksum updated.\n")
				nv.modified = false
			}
		}
	}

	return nv.CMOS.Close()
}

// ValidateChechsum will calculate the CMOS checksum on the checksum area
// and compare it to the checksum value.
// If there is an error it will be a warning and contain the computed and
// stored checksum value.
func (nv *NVRAM) ValidateChecksum() (err error) {
	computed_sum, err := nv.CMOS.ComputeChecksum()
	if err != nil {
		return
	}
	stored_sum, err := nv.CMOS.ReadChecksum()
	if err != nil {
		return
	}

	if computed_sum != stored_sum {
		err = fmt.Errorf("Warning: coreboot CMOS checksum is bad.\nComputed checksum: 0x%X. Stored checksum: 0x%X",
			computed_sum, stored_sum)
	}
	return
}

// NewParameterType will return an interface value for the CMOS parameter.
// This will wither be a string or a uint64.
func (nv *NVRAM) NewParameterType(name string) (value interface{}, err error) {
	e, ok := nv.FindCMOSEntry(name)
	if !ok {
		err = fmt.Errorf("CMOS parameter %s not found.", name)
		return
	}

	switch e.config {
	case CMOSEntryString:
		fallthrough
	case CMOSEntryEnum:
		value = string("")
	case CMOSEntryHex:
		value = uint64(0)
	case CMOSEntryReserved:
		err = fmt.Errorf("Parameter %s is reserved.", e.name)
	default:
		err = fmt.Errorf("CMOS entry %s has invalid config type.", e.name)
	}

	return
}

// WriteCMOSParameter writes provided value to a named CMOS parameter.
func (nv *NVRAM) WriteCMOSParameter(name string, value interface{}) (err error) {
	e, ok := nv.FindCMOSEntry(name)
	if !ok || name == "check_sum" {
		err = fmt.Errorf("CMOS parameter %s not found.", name)
		return
	}

	var v []byte

	switch e.config {
	case CMOSEntryString:
		s, ok := value.(string)
		if !ok {
			err = fmt.Errorf("A string value is required.")
		}
		if e.length < uint(len(s)*8) {
			err = fmt.Errorf("Can not write value %s to CMOS parameter %s that is only %d-bits wide.", s, name, e.length)
			return
		}
		// Copy string to byte array
		v = make([]byte, (e.length+7)/8)
		copy(v[:], []byte(s))

	case CMOSEntryEnum:
		s, ok := value.(string)
		if !ok {
			err = fmt.Errorf("A string value is required.")
		}
		n, ok := nv.FindCMOSEnumValue(e.config_id, s)
		if !ok {
			err = fmt.Errorf("Bad value for parameter %s", name)
			return
		}
		// Check length
		if e.length < 64 && (uint64(n) >= (uint64(1) << e.length)) {
			err = fmt.Errorf("Enum value is too wide for parameter %s", name)
			return
		}
		// Copy uint64 to byte array
		v = make([]byte, 8)
		binary.LittleEndian.PutUint64(v, uint64(n))

	case CMOSEntryHex:
		n, ok := value.(uint64)
		if !ok {
			err = fmt.Errorf("A uint64 value is required.")
		}
		// Check length
		if e.length < 64 && (n >= (uint64(1) << e.length)) {
			err = fmt.Errorf("Can not write value 0x%X to CMOS parameter %s that is only %d-bits wide.", n, name, e.length)
			return
		}

		// Copy uint64 to byte array
		v = make([]byte, 8)
		binary.LittleEndian.PutUint64(v, n)
	}

	err = nv.CMOS.WriteEntry(e, v)
	if err == nil {
		nv.modified = true
	}
	return
}

// ReadCMOSParameter read the current value of a named CMOS parameter.
func (nv *NVRAM) ReadCMOSParameter(name string) (value interface{}, err error) {
	e, ok := nv.FindCMOSEntry(name)
	if !ok || name == "check_sum" {
		err = fmt.Errorf("CMOS parameter %s not found.", name)
		return
	}

	v, err := nv.CMOS.ReadEntry(e)
	if err != nil {
		return
	}

	switch e.config {
	case CMOSEntryString:
		value = string(v)
	case CMOSEntryEnum:
		n := binary.LittleEndian.Uint64(v)
		s, ok := nv.FindCMOSEnumText(e.config_id, uint(n))
		if !ok {
			s = fmt.Sprintf("0x%X # Bad Value", n)
		}
		value = s
	case CMOSEntryHex:
		value = binary.LittleEndian.Uint64(v)
	default:
		err = fmt.Errorf("CMOS entry %s has invalid config type.", e.name)
	}

	return
}
