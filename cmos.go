// Copyright © 2019 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package nvram

import (
	"fmt"
	"github.com/platinasystems/nvram/debug"
)

const (
	cmosSize        uint = 256
	cmosRTCAreaSize uint = 14
)

func verifyCMOSByteIndex(index uint) bool {
	return (index >= cmosRTCAreaSize) && (index < cmosSize)
}

type CMOSer interface {
	Close() error
	ReadByte(off uint) (byte, error)
	WriteByte(off uint, b byte) error
}

type CMOS struct {
	accessor CMOSer
	checksum CMOSChecksum
}

func (c *CMOS) Open() (err error) {
	// Close in case it is already opened.
	c.Close()

	// Open CMOS hardware accessor.
	accessor := new(CMOSHW)
	err = accessor.Open()
	if err != nil {
		return
	}

	c.accessor = accessor
	return
}

func (c *CMOS) OpenMem(filename string) (err error) {
	// Close in case it is already opened
	c.Close()

	// Open CMOS memory file accessor.
	accessor := new(CMOSMem)
	err = accessor.Open(filename)
	if err != nil {
		return
	}

	c.accessor = accessor
	return
}

func (c *CMOS) Close() (err error) {
	// Close any accessor if opened
	if c.accessor != nil {
		err = c.accessor.Close()
		c.accessor = nil
	}
	return
}

func (c *CMOS) WriteEntry(e *CMOSEntry, v []byte) (err error) {
	// Verify CMOS operation
	err = verifyCMOSOp(e)
	if err != nil {
		return
	}

	// Calculate source size, bit offset and remaining bits for entry field.
	src_size := uint(8)
	src_bit := uint(0)
	src_bit_remaining := e.length
	if src_bit_remaining > uint(len(v)*8) {
		src_bit_remaining = uint(len(v) * 8)
	}

	// Start at destination bit
	dst_bit := e.bit
	for src_bit_remaining > 0 {
		// Get write value from source
		wvalue := v[src_bit>>3]

		debug.Trace(debug.LevelMSG3, "src_bit = %d dst_bit = %d src_size = %d  wvalue = %X\n",
			src_bit, dst_bit, src_size, wvalue)

		// Update destination with partial byte data
		if src_size > src_bit_remaining {
			// Find Write value with partial data
			src_size = src_bit_remaining
			wvalue = (wvalue >> (src_bit & 0x7)) & (byte(1<<src_size) - 1)

			// Read current value from destination
			n := byte(0)
			n, err = c.ReadByte(dst_bit >> 3)
			if err != nil {
				return
			}

			// Update destination with remaining bits to write
			mask := (byte(1<<src_size) - 1) << (dst_bit & 0x07)
			n = (n & ^mask) | ((wvalue << (dst_bit & 0x07)) & mask)
			err = c.WriteByte(dst_bit>>3, n)
			return
		} else {
			// Overwrite whole byte values
			err = c.WriteByte(dst_bit>>3, wvalue)
			if err != nil {
				return
			}
		}

		// Move to next byte
		src_bit += src_size
		src_bit_remaining -= src_size
		dst_bit += src_size
	}

	return
}

func (c *CMOS) ReadEntry(e *CMOSEntry) (v []byte, err error) {
	// Verify CMOS operation
	err = verifyCMOSOp(e)
	if err != nil {
		return
	}

	// Calculate source size, bit offset and remaining bits for entry field.
	src_size := uint(8)
	src_bit := e.bit
	src_bit_remaining := e.length

	// Start at destination bit
	dst_bit := uint(0)

	// Create return value buffer.
	if e.config == CMOSEntryString {
		v = make([]byte, (e.length+7)/8)
	} else {
		v = make([]byte, 8)
	}

	for src_bit_remaining > 0 {
		// Read source byte
		n := byte(0)
		n, err = c.ReadByte(src_bit >> 3)
		if err != nil {
			return
		}

		debug.Trace(debug.LevelMSG3, "src_bit = %d dst_bit = %d, src_size = %d  n = %X\n",
			src_bit, dst_bit, src_size, n)

		// For last partial byte mask off extra bits
		if src_size > src_bit_remaining {
			src_size = src_bit_remaining
			n = (n >> (src_bit & 0x7)) & (byte(1<<src_size) - 1)
		}

		// Copy byte read
		v[dst_bit>>3] = n

		// Move to next byte
		src_bit += src_size
		src_bit_remaining -= src_size
		dst_bit += src_size
	}

	return
}

func (c *CMOS) ReadChecksum() (sum uint16, err error) {
	var b0, b1 byte

	// Read checksum b0 and b1
	b0, err = c.ReadByte(c.checksum.index)
	if err != nil {
		return
	}
	b1, err = c.ReadByte(c.checksum.index + 1)
	if err != nil {
		return
	}
	// return in big endian format
	sum = (uint16(b0) << 8) + uint16(b1)
	return
}

func (c *CMOS) WriteChecksum(sum uint16) (err error) {
	// Write checksum byte 0
	err = c.WriteByte(c.checksum.index, byte(sum>>8))
	if err != nil {
		return
	}
	// Write checksum byte 1
	err = c.WriteByte(c.checksum.index+1, byte(sum&0xFF))
	if err != nil {
		return
	}
	return
}

func (c *CMOS) ComputeChecksum() (sum uint16, err error) {
	// Calculate checksum over chemsum area
	for i := c.checksum.start; i <= c.checksum.end; i++ {
		var b byte
		b, err = c.ReadByte(i)
		if err != nil {
			return
		}
		sum += uint16(b)
	}
	return
}

func (c *CMOS) ReadAllMemory() (d []byte, err error) {
	// Retrun buffer with all CMOS data bytes
	// Ignore the RTC area.
	d = make([]byte, cmosSize)
	for i := cmosRTCAreaSize; i < cmosSize; i++ {
		d[i], err = c.ReadByte(i)
		if err != nil {
			return
		}
	}
	return
}

func (c *CMOS) WriteAllMemory(d []byte) (err error) {
	if len(d) < int(cmosSize) {
		return fmt.Errorf("nvram: Not enough data.")
	}
	// Write buffer to entire CMOS area.
	// Ignore RTC area.
	for i := cmosRTCAreaSize; i < cmosSize; i++ {
		err = c.WriteByte(i, d[i])
		if err != nil {
			return
		}
	}
	return
}

func (c *CMOS) ReadByte(off uint) (byte, error) {
	// Read byte using current accessor
	if c.accessor == nil {
		return 0, ErrCMOSNotOpen
	}
	return c.accessor.ReadByte(off)
}

func (c *CMOS) WriteByte(off uint, b byte) error {
	// Write byte using current accessor
	if c.accessor == nil {
		return ErrCMOSNotOpen
	}
	return c.accessor.WriteByte(off, b)
}
