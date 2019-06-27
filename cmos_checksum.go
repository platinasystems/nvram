// Copyright © 2019 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package nvram

import (
	"fmt"
	"github.com/platinasystems/nvram/debug"
)

type CMOSChecksum struct {
	start, end, index uint
}

func (c CMOSChecksum) String() string {
	return fmt.Sprintf("%d %d %d", c.start*8, c.end*8, c.index*8)
}

func NewCMOSChecksum(start, end, index uint) (c *CMOSChecksum, err error) {

	debug.Trace(debug.LevelMSG3, "New CMOS Checksum %d %d %d\n", start, end, index)

	// Check that checksum area is aligned
	if start%8 != 0 {
		err = fmt.Errorf("Checksum area start not aligned")
		return
	}

	if end%8 != 7 {
		err = fmt.Errorf("Checksum area end not aligned")
		return
	}

	// Check that checksum location is aligned
	if index%8 != 0 {
		err = fmt.Errorf("Checksum location not aligned")
		return
	}

	// Check that checksum area is valid.
	if end <= start {
		err = fmt.Errorf("Checksum area invalid.")
		return
	}

	// Convert to bytes
	start /= 8
	end /= 8
	index /= 8

	// Verify checksum area range
	if !verifyCMOSByteIndex(start) || !verifyCMOSByteIndex(end) {
		err = fmt.Errorf("Checksum area out of range.")
		return
	}

	// Verify checksum location range
	if !verifyCMOSByteIndex(index) {
		err = fmt.Errorf("Checksum location out of range.")
		return
	}

	// Make sure checksum area does not overlap location
	if checkAreaOverLap(start, end-start+1, index, index+1) {
		err = fmt.Errorf("Checksum overlaps summed area.")
		return
	}

	debug.Trace(debug.LevelMSG3, "Valid checksum %d %d %d\n", start, end, index)

	// Create new checksum with byte values
	c = &CMOSChecksum{start: start, end: end, index: index}

	return
}
