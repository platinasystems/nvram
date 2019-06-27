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

const (
	sys_iopl   = 172 //amd64
	sys_ioperm = 173 //amd64
)

type CMOSHW struct {
	port_file *os.File
}

func (c *CMOSHW) Open() (err error) {
	// Close in case it is already opened
	c.Close()

	// Close on any error
	defer func() {
		if err != nil {
			c.Close()
		}
	}()

	debug.Trace(debug.LevelMSG1, "Opening CMOS HW\n")

	// Set IO privilege level to 3. 
	if _, _, errno := syscall.Syscall(sys_iopl,
		uintptr(3), 0, 0); errno != 0 {
		return errno
	}

	// Open device ports for access to CMOS NVRAM
	c.port_file, err = os.OpenFile("/dev/port", os.O_RDWR|os.O_SYNC, 0755)
	if err != nil {
		return
	}

	return
}

func (c *CMOSHW) Close() error {

	debug.Trace(debug.LevelMSG1, "Closing CMOS HW\n")

	// Set IO privilege level to normal
	if _, _, errno := syscall.Syscall(sys_iopl,
		uintptr(0), 0, 0); errno != 0 {
		return errno
	}

	// Close port file if opened
	if c.port_file != nil {
		c.port_file.Close()
		c.port_file = nil
	}

	return nil
}

func (c *CMOSHW) ReadByte(off uint) (byte, error) {
	if c.port_file == nil {
		return 0, ErrCMOSNotOpen
	}
	if !verifyCMOSByteIndex(off) {
		return 0, ErrInvalidCMOSIndex
	}

	// Find port0 and 1 to set CMOS data offset
	var port_0, port_1 int64
	if off < 128 {
		port_0 = 0x70
		port_1 = 0x71
	} else {
		port_0 = 0x72
		port_1 = 0x73
	}

	// Set offset
	if err := c.ioWriteReg8(port_0, byte(off)); err != nil {
		return 0, err
	}

	// Read data from NVRAM at offset
	return c.ioReadReg8(port_1)
}

func (c *CMOSHW) WriteByte(off uint, b byte) error {
	if c.port_file == nil {
		return ErrCMOSNotOpen
	}

	if !verifyCMOSByteIndex(off) {
		return ErrInvalidCMOSIndex
	}

	// Find port0 and 1 to set CMOS data offset
	var port_0, port_1 int64
	if off < 128 {
		port_0 = 0x70
		port_1 = 0x71
	} else {
		port_0 = 0x72
		port_1 = 0x73
	}

	// Set offset
	if err := c.ioWriteReg8(port_0, byte(off)); err != nil {
		return err
	}

	// Write data to NVRAM at offset
	if err := c.ioWriteReg8(port_1, b); err != nil {
		return err
	}

	return nil
}

func (c *CMOSHW) ioReadReg8(addr int64) (b byte, err error) {
	// Seek to port address
	if _, err = c.port_file.Seek(addr, 0); err != nil {
		return
	}

	// Read data from port into buffer
	buf := make([]byte, 1)
	n, err := c.port_file.Read(buf)
	if err != nil {
		return
	}

	if n != 1 {
		err = fmt.Errorf("nvram: Unable to read port.")
		return
	}

	// Return data read
	b = buf[0]
	return
}

func (c *CMOSHW) ioWriteReg8(addr int64, b byte) (err error) {
	// Prepare write buffer
	buf := make([]byte, 1)
	buf[0] = b

	// Seek to port address
	if _, err = c.port_file.Seek(addr, 0); err != nil {
		return err
	}

	// Write data to port
	n, err := c.port_file.Write(buf)
	if err != nil {
		return err
	}

	// Sync write
	c.port_file.Sync()

	if n != 1 {
		return fmt.Errorf("nvram: Unable to write port.")
	}

	return
}
