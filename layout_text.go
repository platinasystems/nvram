package nvram

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func ReadLayoutFromTextFile(filename string) (layout *Layout, err error) {
	var file *os.File

	// Create new empty layout
	layout = NewLayout()

	// Open layout text file
	file, err = os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()

	// Start parsing comment region
	var mode int = 0

	var linenum uint = 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Get line and ignore blanks and comments
		line := strings.TrimSpace(scanner.Text())
		linenum++
		if len(line) == 0 {
			continue
		}
		if line[0] == '#' {
			continue
		}

		// Break line into files
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		// A single filed indicates a new region
		// Change mode to parsing entries, enumerations or checksums
		if len(fields) == 1 {
			switch fields[0] {
			case "entries":
				mode = 1
			case "enumerations":
				mode = 2
			case "checksums":
				mode = 3
			default:
				err = fmt.Errorf("Unexpected section header on line %d", linenum)
				return
			}
			continue
		}

		switch mode {
		case 1:
			// Entries have 5 fields
			if len(fields) != 5 {
				err = fmt.Errorf("Unexpected data in entries on line %d", linenum)
				return
			}

			// Scan line and parse CMOS entry data
			var entry CMOSEntry
			var n int
			n, err = fmt.Sscanf(line, "%d %d %c %d %s",
				&entry.bit, &entry.length, &entry.config, &entry.config_id, &entry.name)
			if err != nil || n != 5 {
				err = fmt.Errorf("Unexpected data in entries on line %d", linenum)
				return
			}

			// Add entry to layout
			err = layout.AddCMOSEntry(&entry)
			if err != nil {
				return
			}

		case 2:
			// Enumerations have 3 fields
			if len(fields) != 3 {
				err = fmt.Errorf("Unexpected data in enumerations on line %d", linenum)
				return
			}

			// Scan line and parse CMOS enumeration data
			var item CMOSEnumItem
			var n int
			n, err = fmt.Sscanf(line, "%d %d %s",
				&item.id, &item.value, &item.text)
			if err != nil || n != 3 {
				err = fmt.Errorf("Unexpected data in enumerations on line %d", linenum)
				return
			}

			// Check if enumeration value already exists for an id.
			_, ok := layout.FindCMOSEnumText(item.id, item.value)
			if ok {
				fmt.Errorf("Enum %d already exists for id %d on line %d",
					item.id, item.value, linenum)
				return
			}

			// Add enumeration to layout
			layout.AddCMOSEnum(&item)

		case 3:
			// Checksums have 4 fields
			if len(fields) != 4 {
				err = fmt.Errorf("Unexpected data in checksums on line %d", linenum)
				return
			}

			// Scan line and parase checksum info.
			var start, end, index uint
			var label string
			var n int
			n, err = fmt.Sscanf(line, "%s %d %d %d", &label,
				&start, &end, &index)
			if err != nil || n != 4 {
				err = fmt.Errorf("Unexpected data in checksums on line %d", linenum)
				return
			}

			// Check label
			if label != "checksum" {
				err = fmt.Errorf("Missing checksum on line %d", linenum)
				return
			}

			// Create and test new CMOS checksum for layout
			layout.cmosChecksum, err = NewCMOSChecksum(start, end, index)
			if err != nil {
				return
			}

		default:
			err = fmt.Errorf("Unexpected data on line %d", linenum)
			return
		}
	}

	// Return any errors
	err = scanner.Err()
	return
}
