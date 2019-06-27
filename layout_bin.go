package nvram

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

type cmosEntryTableRecord struct {
	lbRecord
	bit      uint32
	length   uint32
	config   uint32
	configId uint32
	name     [32]byte
}

type cmosEnumTableRecord struct {
	lbRecord
	configId uint32
	value    uint32
	text     [32]byte
}

type cmosChecksumTableRecord struct {
	lbRecord
	rangeStart   uint32
	rangeEnd     uint32
	location     uint32
	checksumType uint32
}

func ReadLayoutFromCMOSTable(table *cmosOptionTable) (layout *Layout, err error) {
	// Check that we have a valid CMOS Option table
	if table == nil || table.tag != 200 {
		err = fmt.Errorf("Not a valid CMOS Option Table")
		return
	}

	// Create a new empty CMOS layout.
	layout = NewLayout()

	// Set address into option table after table header.
	var address = uintptr(unsafe.Pointer(table)) + uintptr(table.headerLength)
	var endAddress = address + uintptr(table.size-table.headerLength)

	for {
		// Continue looking for table records till end of table data.
		if address >= endAddress {
			break
		}

		// Look at current table record
		var lbrec = (*lbRecord)(unsafe.Pointer(address))

		switch lbrec.tag {
		// Decode CMOS entry Table Record
		case 201:
			var rec = (*cmosEntryTableRecord)(unsafe.Pointer(lbrec))
			var entry CMOSEntry

			// Read values for CMOS Entry
			entry.bit = uint(rec.bit)
			entry.length = uint(rec.length)
			entry.config = CMOSEntryConfig(rec.config)
			entry.config_id = uint(rec.configId)

			// Copy string from table entry
			for _, v := range rec.name {
				if v == 0 {
					break
				}
				entry.name = entry.name + string(v)
			}

			// Add CMOS entry to layout
			err = layout.AddCMOSEntry(&entry)
			if err != nil {
				return
			}

		// Decode CMOS Enumeration Record		
		case 202:
			var rec = (*cmosEnumTableRecord)(unsafe.Pointer(lbrec))
			var item CMOSEnumItem

			// Read values for CMOS enumeration
			item.id = uint(rec.configId)
			item.value = uint(rec.value)

			// Copy string from table entry
			for _, v := range rec.text {
				if v == 0 {
					break
				}
				item.text = item.text + string(v)
			}

			// Check if enumeration value already exists for an id.
			_, ok := layout.FindCMOSEnumText(item.id, item.value)
			if ok {
				fmt.Errorf("Enum %d already exists for id %d",
					item.id, item.value)
				return
			}

			// Add CMOS enumeration to layout
			layout.AddCMOSEnum(&item)

		// Decode CMOS Checksum Record
		case 204:
			var rec = (*cmosChecksumTableRecord)(unsafe.Pointer(lbrec))

			// Read and check CMOS checksum info.
			layout.cmosChecksum, err = NewCMOSChecksum(uint(rec.rangeStart),
				uint(rec.rangeEnd), uint(rec.location))
			if err != nil {
				return
			}
		}

		// Move to next table record
		address += uintptr(lbrec.size)
	}

	return
}

func ReadLayoutFromCMOSTableBinary(filename string) (layout *Layout, err error) {
	var mem_file *os.File
	var mem []byte

	// Unmap and close CMOS Option table file.
	defer func() {
		if len(mem) > 0 {
			syscall.Munmap(mem)
			mem = nil
		}

		if mem_file != nil {
			mem_file.Close()
			mem_file = nil
		}
	}()

	// Open CMOS option table file
	mem_file, err = os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return
	}

	fi, err := mem_file.Stat()
	if err != nil {
		return
	}
	size := fi.Size()

	if size < 0 {
		err = fmt.Errorf("File %s has negative size.", filename)
		return
	}

	// Map CMOS option table
	mem, err = syscall.Mmap(int(mem_file.Fd()), 0, int(size),
		syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		return
	}


	// Read CMOS Option table and create layout	
	return ReadLayoutFromCMOSTable((*cmosOptionTable)(unsafe.Pointer(&mem[0])))
}

func ReadLayoutFromCoreBootTable() (layout *Layout, err error) {
	var cbtable CoreBootTable

	// Close coreboot able
	defer func() {
		cbtable.Close()
	}()

	// Open coreboot table
	err = cbtable.Open()
	if err != nil {
		return
	}

	// Find the CMOS Option table in the coreboot table
	optionTable, ok := cbtable.FindCMOSOptionTable()
	if !ok {
		err = fmt.Errorf("CMOS Option Table not found")
		return
	}

	// Read layout from CMOS Option table
	return ReadLayoutFromCMOSTable(optionTable)

}
