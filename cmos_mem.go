// Copyright © 2019 Platina Systems, Inc. All rights reserved.
// Use of this source code is governed by the GPL-2 license described in the
// LICENSE file.

package nvram

import (
	"fmt"
	"github.com/platinasystems/nvram/debug"
	"os"
	"syscall"
)

type CMOSMem struct {
	mem_file *os.File
	mem      []byte
}

func (c *CMOSMem) Open(filename string) (err error) {
	// Close in case it is already opened
	c.Close()

	// Close on any error
	defer func() {
		if err != nil {
			c.Close()
		}
	}()

	debug.Trace(debug.LevelMSG1, "Opening CMOS Mem file %s\n", filename)

	// Open CMOS data file
	c.mem_file, err = os.OpenFile(filename, os.O_RDWR|os.O_SYNC, 0)
	if err != nil {
		return
	}

	fi, err := c.mem_file.Stat()
	if err != nil {
		return
	}
	size := fi.Size()

	if size < 0 {
		err = fmt.Errorf("nvram: File %s has negative size.", filename)
		return
	}

	// Memory map file for access.
	c.mem, err = syscall.Mmap(int(c.mem_file.Fd()), 0, int(size),
		syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return
	}

	debug.Trace(debug.LevelMSG3, "c.mem len = %d\n", len(c.mem))

	return
}

func (c *CMOSMem) Close() (err error) {

	debug.Trace(debug.LevelMSG1, "Closing CMOS Mem\n")

	// Unmap file if it has been mapped
	if len(c.mem) > 0 {
		syscall.Munmap(c.mem)
		c.mem = nil
	}

	// Close file
	if c.mem_file != nil {
		c.mem_file.Close()
		c.mem_file = nil
	}

	return
}

func (c *CMOSMem) ReadByte(off uint) (byte, error) {
	if len(c.mem) == 0 {
		return 0, ErrCMOSNotOpen
	}
	if !verifyCMOSByteIndex(off) {
		return 0, ErrInvalidCMOSIndex
	}
	return c.mem[off], nil
}

func (c *CMOSMem) WriteByte(off uint, b byte) error {
	if len(c.mem) == 0 {
		return ErrCMOSNotOpen
	}
	if !verifyCMOSByteIndex(off) {
		return ErrInvalidCMOSIndex
	}
	c.mem[off] = b
	return nil
}
