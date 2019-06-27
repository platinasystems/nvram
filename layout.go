package nvram

import (
	"fmt"
	"sort"
)

type CMOSEnumItem struct {
	id    uint
	value uint
	text  string
}

func (i CMOSEnumItem) String() string {
	return fmt.Sprintf("%d %d %s", i.id, i.value, i.text)
}

func (i CMOSEnumItem) Id() uint {
	return i.id
}

func (i CMOSEnumItem) Value() uint {
	return i.value
}

func (i CMOSEnumItem) Text() string {
	return i.text
}

type CMOSEnum struct {
	itos map[uint]string
	stoi map[string]uint
}

type Layout struct {
	enums        map[uint]*CMOSEnum
	entries      map[string]*CMOSEntry
	entrieslist  []*CMOSEntry
	cmosChecksum *CMOSChecksum
}

func NewLayout() *Layout {
	c, _ := NewCMOSChecksum(392, 1007, 1008)
	return &Layout{
		enums:        make(map[uint]*CMOSEnum),
		entries:      make(map[string]*CMOSEntry),
		cmosChecksum: c}
}

func (l *Layout) AddCMOSEntry(entry *CMOSEntry) (err error) {
	// Verify CMOS Entry
	err = verifyCMOSEntry(entry)
	if err != nil {
		return
	}

	// Add entries to entry list sorted by starting bit.
	var pos int = 0
	for i, e := range l.entrieslist {
		pos = i
		if entry.bit < e.bit {
			break
		}
	}

	// Check if new entry overlaps current entry in sorted list.
	if len(l.entrieslist) > 0 {
		if pos > 0 {
			if entry.IsOverlap(l.entrieslist[pos-1]) {
				err = fmt.Errorf("Entry %s overlaps %s", *entry, l.entrieslist[pos-1])
				return
			}
		}

		if entry.IsOverlap(l.entrieslist[pos]) {
			err = fmt.Errorf("Entry %s overlaps %s", *entry, l.entrieslist[pos])
			return
		}
	}

	// Add new entry to list at correct position
	l.entrieslist = append(l.entrieslist, nil)
	copy(l.entrieslist[pos+1:], l.entrieslist[pos:])
	l.entrieslist[pos] = entry

	// Add entry to enteries map
	l.entries[entry.name] = entry
	return
}

func (l *Layout) GetCMOSEntriesList() []*CMOSEntry {
	// Return a copy of the sorted CMOS entry list.
	return l.entrieslist
}

func (l *Layout) FindCMOSEntry(name string) (entry *CMOSEntry, ok bool) {
	entry, ok = l.entries[name]
	return
}

func (l *Layout) AddCMOSEnum(item *CMOSEnumItem) {

	// Create new CMOS Enum for each item's id.
	enum, ok := l.enums[item.id]
	if !ok {
		enum = new(CMOSEnum)
		// Create maps to search by enum name or value.
		enum.itos = make(map[uint]string)
		enum.stoi = make(map[string]uint)
		l.enums[item.id] = enum
	}

	// Add item text and value to maps
	enum.itos[item.value] = item.text
	enum.stoi[item.text] = item.value
}

func (l *Layout) FindCMOSEnumText(id uint, value uint) (text string, ok bool) {
	var enum *CMOSEnum

	// Find CMOS Enum by id
	enum, ok = l.enums[id]
	if !ok {
		return "", false
	}

	// Find CMOS Enum text from value
	text, ok = enum.itos[value]
	return
}

func (l *Layout) FindCMOSEnumValue(id uint, text string) (value uint, ok bool) {
	var enum *CMOSEnum

	// Find CMOS Enum by id
	enum, ok = l.enums[id]
	if !ok {
		return 0, false
	}

	// Find CMOS Enum value from text.
	value, ok = enum.stoi[text]
	return
}

func (l *Layout) GetCMOSEnumItemsById(id uint) (items []CMOSEnumItem, ok bool) {

	// Find CMOS Enum by id
	enum, ok := l.enums[id]
	if !ok {
		return
	}

	// Create sorted list of Enum values
	var values []int
	for value := range enum.itos {
		values = append(values, int(value))
	}
	sort.Ints(values)

	// Add CMOS Itmes to list sorted by value
	for _, value := range values {
		var text string
		text, ok = enum.itos[uint(value)]
		if !ok {
			continue
		}
		items = append(items, CMOSEnumItem{uint(id), uint(value), text})
	}

	return
}

func (l *Layout) GetCMOSEnumItems() (items []CMOSEnumItem) {
	// Create sorted list of enum ids.	
	var ids []int
	for id := range l.enums {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)

	// Add all enum items for all ids to list
	for _, id := range ids {
		items_for_id, _ := l.GetCMOSEnumItemsById(uint(id))
		items = append(items, items_for_id...)
	}

	return
}

func (l *Layout) GetCheckChecksum() CMOSChecksum {
	return *l.cmosChecksum
}
