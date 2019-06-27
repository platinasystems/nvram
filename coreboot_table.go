package nvram

import (
	"fmt"
	"github.com/platinasystems/nvram/debug"
	"os"
	"syscall"
	"unsafe"
)

type lbHeader struct {
	signature      uint32
	headerBytes    uint32
	headerChecksum uint32
	tableBytes     uint32
	tableChecksum  uint32
	tableEntries   uint32
}

type lbRecord struct {
	tag  uint32
	size uint32
}

type lbForward struct {
	lbRecord
	forward uint64
}

type cmosOptionTable struct {
	lbRecord
	headerLength uint32
}

type CoreBootTable struct {
	mem_file *os.File
	mem      []byte
	baseAddr uintptr

	header *lbHeader
	recs   []*lbRecord
}

func (t *CoreBootTable) Open() (err error) {
	defer func() {
		if err != nil {
			t.Close()
		}
	}()

	t.mem_file, err = os.OpenFile("/dev/mem", os.O_RDONLY, 0)
	if err != nil {
		return
	}

	err = t.openTable(0x00000000, 0x00000fff)
	if err != nil {
		err = t.openTable(0x000f0000, 0x000fffff)
	}
	if err != nil {
		return
	}

	return
}

func (t *CoreBootTable) Close() (err error) {
	debug.Trace(debug.LevelMSG1, "Closing Coreboot table\n")

	if len(t.mem) > 0 {
		syscall.Munmap(t.mem)
		t.mem = nil
	}

	if t.mem_file != nil {
		t.mem_file.Close()
		t.mem_file = nil
	}

	t.baseAddr = 0
	t.header = nil
	t.recs = nil
	return
}

func (t *CoreBootTable) FindCMOSOptionTable() (c *cmosOptionTable, ok bool) {
	for _, lbrec := range t.recs {
		if lbrec.tag == 0xc8 {
			return (*cmosOptionTable)(unsafe.Pointer(lbrec)), true
		}
	}
	return nil, false
}

func (t *CoreBootTable) openTable(start, end uintptr) (err error) {

	debug.Trace(debug.LevelMSG1, "Looking for table @0x%08X\n", start)

	defer func() {
		if err != nil {
			t.header = nil
			t.recs = nil
		}
	}()

	t.mapPages(start, end)

	for i := 0; i < len(t.mem); i += 16 {
		var header = (*lbHeader)(unsafe.Pointer(&t.mem[i]))
		if header.signature == 0x4f49424c {
			debug.Trace(debug.LevelMSG1, "Table found @0x%08X\n", unsafe.Pointer(header))
			if t.computeIpChecksum(uintptr(unsafe.Pointer(header)), uint64(header.headerBytes)) != 0 {
				debug.Trace(debug.LevelMSG1, "Header checksum bad\n")
				continue
			}

			phyAddr := t.baseAddr + uintptr(i)
			t.mapPages(phyAddr, phyAddr+uintptr(header.tableBytes))
			virtAddr := uintptr(unsafe.Pointer(&t.mem[0])) + phyAddr - t.baseAddr
			header = (*lbHeader)(unsafe.Pointer(virtAddr))

			var lbrec = (*lbRecord)(unsafe.Pointer(virtAddr + uintptr(header.headerBytes)))

			if t.computeIpChecksum(uintptr(unsafe.Pointer(lbrec)), uint64(header.tableBytes)) != header.tableChecksum {
				debug.Trace(debug.LevelMSG1, "Table checksum bad\n")
				continue
			}

			t.header = header
			t.recs = nil
			var lbforward *lbForward
			for i := uint32(0); i < header.tableBytes; {
				debug.Trace(debug.LevelMSG3, "Found lbRecord tag = %X len = %d\n", lbrec.tag, lbrec.size)

				if lbforward == nil && lbrec.tag == 0x11 {
					lbforward = (*lbForward)(unsafe.Pointer(lbrec))
				}

				t.recs = append(t.recs, lbrec)
				i += lbrec.size
				lbrec = (*lbRecord)(unsafe.Pointer(uintptr(unsafe.Pointer(lbrec)) + uintptr(lbrec.size)))
			}

			if len(t.recs) != int(header.tableEntries) {
				debug.Trace(debug.LevelMSG1, "Unexpected number of table entries.\n")
				continue
			}

			if lbforward != nil {
				debug.Trace(debug.LevelMSG1, "Forwarding table found.\n")
				err = t.openTable(uintptr(lbforward.forward), uintptr(lbforward.forward)+uintptr(os.Getpagesize()))
				return
			}

			return
		}
	}

	err = fmt.Errorf("Coreboot table not found.")
	return
}

func (t *CoreBootTable) mapPages(start, end uintptr) (err error) {
	t.baseAddr = start
	length := end - start
	pagesize := uintptr(os.Getpagesize())

	numPages := (length +
		(t.baseAddr & (pagesize - 1)) +
		pagesize - 1) / pagesize
	t.baseAddr &= ^(pagesize - 1)

	if len(t.mem) > 0 {
		syscall.Munmap(t.mem)
		t.mem = nil
	}

	t.mem, err = syscall.Mmap(int(t.mem_file.Fd()),
		int64(t.baseAddr), int(numPages*pagesize),
		syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return
	}

	return
}

func (t *CoreBootTable) computeIpChecksum(start uintptr, length uint64) uint32 {

	sum := uint32(0)

	for i := start; i < (start + uintptr(length)); i++ {
		ptr := (*byte)(unsafe.Pointer(i))
		value := uint32(*ptr)
		if (i & 1) != 0 {
			value <<= 8
		}

		sum += value

		if sum > 0xFFFF {
			sum = (sum + (sum >> 16)) & 0xFFFF
		}
	}
	return (^sum) & 0xFFFF
}
