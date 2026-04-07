// Package storage provides page-based storage for XxLdb
package storage

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"

	"github.com/topxeq/xxldb/types"
)

// Page size constants (16KB)
const (
	PageSize16KB     = 16384
	PageHeaderSizeV2 = 32
	MaxPageContentV2 = PageSize16KB - PageHeaderSizeV2 // 16352 bytes
)

// PageType represents the type of a page
type PageType uint8

const (
	PageTypeFree      PageType = 0 // Free/unused page
	PageTypeData      PageType = 1 // Data page (stores row records)
	PageTypeOverflow  PageType = 2 // Overflow page (large row data)
	PageTypeBTree     PageType = 3 // B-tree index page
	PageTypeBTreeLeaf PageType = 4 // B-tree leaf page
)

// String returns the string representation of page type
func (pt PageType) String() string {
	switch pt {
	case PageTypeFree:
		return "FREE"
	case PageTypeData:
		return "DATA"
	case PageTypeOverflow:
		return "OVERFLOW"
	case PageTypeBTree:
		return "BTREE"
	case PageTypeBTreeLeaf:
		return "BTREE_LEAF"
	default:
		return "UNKNOWN"
	}
}

// Page flags
const (
	PageFlagEncrypted uint8 = 1 << iota
	PageFlagCompressed
	PageFlagDirty
)

// PageHeaderV2 is the header for a 16KB page
// Total: 32 bytes
type PageHeaderV2 struct {
	// Basic info (12 bytes)
	PageID   uint64   // 8 bytes - Unique page ID
	PageType PageType // 1 byte - Page type
	Flags    uint8    // 1 byte - Flags (encrypted, compressed, dirty)
	Version  uint16   // 2 bytes - Format version

	// Space management (8 bytes)
	FreeSpace  uint16 // 2 bytes - Free space offset from end
	CellCount  uint16 // 2 bytes - Number of cells/rows
	Fragmented uint16 // 2 bytes - Fragmented space size
	Reserved   uint16 // 2 bytes - Reserved

	// Link structure (8 bytes)
	RightPage uint64 // 8 bytes - Right sibling page ID (for B-tree or data page list)

	// Checksum (4 bytes)
	Checksum uint32 // 4 bytes - CRC32 checksum
}

// PageV2 represents a 16KB page
type PageV2 struct {
	Header  PageHeaderV2
	Content [MaxPageContentV2]byte
}

// NewPageV2 creates a new page with the given ID and type
func NewPageV2(pageID uint64, pageType PageType) *PageV2 {
	p := &PageV2{
		Header: PageHeaderV2{
			PageID:     pageID,
			PageType:   pageType,
			Version:    1,
			FreeSpace:  MaxPageContentV2,
			CellCount:  0,
			Fragmented: 0,
			RightPage:  0,
		},
	}
	return p
}

// AvailableSpace returns the available space in the page
func (p *PageV2) AvailableSpace() uint16 {
	// Available = FreeSpace - CellPointerArray(CellCount * 2) - Fragmented
	pointerArraySize := uint16(p.Header.CellCount * 2)
	if p.Header.FreeSpace < pointerArraySize {
		return 0
	}
	available := p.Header.FreeSpace - pointerArraySize
	if available < p.Header.Fragmented {
		return 0
	}
	return available - p.Header.Fragmented
}

// CanInsert checks if a row of the given size can be inserted
func (p *PageV2) CanInsert(rowSize int) bool {
	// Need: row data + cell pointer (2 bytes)
	needed := uint16(rowSize) + 2
	return p.AvailableSpace() >= needed
}

// UsageRatio returns the page usage ratio (0.0 to 1.0)
func (p *PageV2) UsageRatio() float64 {
	used := MaxPageContentV2 - p.Header.FreeSpace
	return float64(used) / float64(MaxPageContentV2)
}

// CalculateChecksum calculates the CRC32 checksum of the page content
func (p *PageV2) CalculateChecksum() uint32 {
	return crc32.ChecksumIEEE(p.Content[:])
}

// UpdateChecksum updates the checksum field
func (p *PageV2) UpdateChecksum() {
	p.Header.Checksum = p.CalculateChecksum()
}

// VerifyChecksum verifies the page checksum
func (p *PageV2) VerifyChecksum() bool {
	return p.Header.Checksum == p.CalculateChecksum()
}

// ToBytes serializes the page to bytes
func (p *PageV2) ToBytes() []byte {
	data := make([]byte, PageSize16KB)

	// Write header (32 bytes)
	binary.LittleEndian.PutUint64(data[0:8], p.Header.PageID)
	data[8] = byte(p.Header.PageType)
	data[9] = p.Header.Flags
	binary.LittleEndian.PutUint16(data[10:12], p.Header.Version)
	binary.LittleEndian.PutUint16(data[12:14], p.Header.FreeSpace)
	binary.LittleEndian.PutUint16(data[14:16], p.Header.CellCount)
	binary.LittleEndian.PutUint16(data[16:18], p.Header.Fragmented)
	binary.LittleEndian.PutUint16(data[18:20], p.Header.Reserved)
	binary.LittleEndian.PutUint64(data[20:28], p.Header.RightPage)
	binary.LittleEndian.PutUint32(data[28:32], p.Header.Checksum)

	// Write content
	copy(data[PageHeaderSizeV2:], p.Content[:])

	return data
}

// FromBytes deserializes the page from bytes
func (p *PageV2) FromBytes(data []byte) error {
	if len(data) != PageSize16KB {
		return fmt.Errorf("invalid page size: expected %d, got %d", PageSize16KB, len(data))
	}

	// Read header
	p.Header.PageID = binary.LittleEndian.Uint64(data[0:8])
	p.Header.PageType = PageType(data[8])
	p.Header.Flags = data[9]
	p.Header.Version = binary.LittleEndian.Uint16(data[10:12])
	p.Header.FreeSpace = binary.LittleEndian.Uint16(data[12:14])
	p.Header.CellCount = binary.LittleEndian.Uint16(data[14:16])
	p.Header.Fragmented = binary.LittleEndian.Uint16(data[16:18])
	p.Header.Reserved = binary.LittleEndian.Uint16(data[18:20])
	p.Header.RightPage = binary.LittleEndian.Uint64(data[20:28])
	p.Header.Checksum = binary.LittleEndian.Uint32(data[28:32])

	// Read content
	copy(p.Content[:], data[PageHeaderSizeV2:])

	return nil
}

// Cell pointer operations

// GetCellPointer returns the offset of a cell by index
func (p *PageV2) GetCellPointer(index int) uint16 {
	if index < 0 || index >= int(p.Header.CellCount) {
		return 0
	}
	offset := index * 2
	return binary.LittleEndian.Uint16(p.Content[offset : offset+2])
}

// SetCellPointer sets the offset of a cell by index
func (p *PageV2) SetCellPointer(index int, cellOffset uint16) {
	offset := index * 2
	binary.LittleEndian.PutUint16(p.Content[offset:offset+2], cellOffset)
}

// AddCellPointer adds a new cell pointer
func (p *PageV2) AddCellPointer(cellOffset uint16) {
	p.SetCellPointer(int(p.Header.CellCount), cellOffset)
	p.Header.CellCount++
}

// === Row encoding ===

// Row encoding format:
// [RowID:8B][ColCount:2B][Col1Data][Col2Data]...[OverflowPage:8B (if has overflow)]
//
// Column encoding:
// - NULL: [Type:1B] = 0xFF
// - INT/SEQ: [Type:1B][Value:8B] = 9 bytes
// - FLOAT: [Type:1B][Value:8B] = 9 bytes
// - VARCHAR/TEXT/CHAR: [Type:1B][Len:4B][Data:N]
// - BLOB/IMAGE (inline): [Type:1B][Len:4B][Data:N]
// - BLOB/IMAGE (external): [Type:1B][0xFE][BlobID:8B][Size:8B] = 18 bytes

// Column encoding constants
const (
	ColNullMarker   = 0xFF
	ColBlobRefMarker = 0xFE
)

// EncodeRow encodes a row into bytes
func EncodeRow(rowID uint64, row []types.Value) []byte {
	// Estimate size
	estimatedSize := 10 + len(row)*10 // RowID + ColCount + approx per column
	for _, v := range row {
		if !v.IsNull && v.BlobRef == nil {
			switch v.Type {
			case types.TypeVarchar, types.TypeText, types.TypeChar, types.TypeBlob, types.TypeImage:
				if s, ok := v.Data.(string); ok {
					estimatedSize += len(s)
				} else if b, ok := v.Data.([]byte); ok {
					estimatedSize += len(b)
				}
			}
		}
	}

	buf := make([]byte, 0, estimatedSize)

	// RowID (8 bytes)
	buf = binary.LittleEndian.AppendUint64(buf, rowID)

	// Column count (2 bytes)
	buf = binary.LittleEndian.AppendUint16(buf, uint16(len(row)))

	// Encode each column
	for _, v := range row {
		buf = append(buf, encodeColumn(v)...)
	}

	return buf
}

// encodeColumn encodes a single column value
func encodeColumn(v types.Value) []byte {
	if v.IsNull {
		return []byte{ColNullMarker}
	}

	// Check for external blob reference
	if v.BlobRef != nil {
		buf := make([]byte, 18)
		buf[0] = byte(v.Type)
		buf[1] = ColBlobRefMarker
		binary.LittleEndian.PutUint64(buf[2:10], v.BlobRef.ID)
		binary.LittleEndian.PutUint64(buf[10:18], uint64(v.BlobRef.Size))
		return buf
	}

	// Encode based on type
	switch v.Type {
	case types.TypeInt, types.TypeSeq:
		buf := make([]byte, 9)
		buf[0] = byte(v.Type)
		var val int64
		switch d := v.Data.(type) {
		case int64:
			val = d
		case int:
			val = int64(d)
		case int32:
			val = int64(d)
		case float64:
			val = int64(d)
		}
		binary.LittleEndian.PutUint64(buf[1:9], uint64(val))
		return buf

	case types.TypeFloat:
		buf := make([]byte, 9)
		buf[0] = byte(v.Type)
		var val float64
		switch d := v.Data.(type) {
		case float64:
			val = d
		case float32:
			val = float64(d)
		}
		binary.LittleEndian.PutUint64(buf[1:9], uint64(val))
		return buf

	case types.TypeVarchar, types.TypeText, types.TypeChar:
		var str string
		switch d := v.Data.(type) {
		case string:
			str = d
		default:
			str = fmt.Sprintf("%v", d)
		}
		buf := make([]byte, 5+len(str))
		buf[0] = byte(v.Type)
		binary.LittleEndian.PutUint32(buf[1:5], uint32(len(str)))
		copy(buf[5:], str)
		return buf

	case types.TypeBlob, types.TypeImage:
		var data []byte
		switch d := v.Data.(type) {
		case []byte:
			data = d
		case string:
			data = []byte(d)
		default:
			data = []byte(fmt.Sprintf("%v", d))
		}
		buf := make([]byte, 5+len(data))
		buf[0] = byte(v.Type)
		binary.LittleEndian.PutUint32(buf[1:5], uint32(len(data)))
		copy(buf[5:], data)
		return buf

	case types.TypeDate, types.TypeTime, types.TypeDatetime:
		var str string
		switch d := v.Data.(type) {
		case string:
			str = d
		default:
			str = fmt.Sprintf("%v", d)
		}
		buf := make([]byte, 5+len(str))
		buf[0] = byte(v.Type)
		binary.LittleEndian.PutUint32(buf[1:5], uint32(len(str)))
		copy(buf[5:], str)
		return buf

	default:
		// Generic encoding
		str := fmt.Sprintf("%v", v.Data)
		buf := make([]byte, 5+len(str))
		buf[0] = byte(v.Type)
		binary.LittleEndian.PutUint32(buf[1:5], uint32(len(str)))
		copy(buf[5:], str)
		return buf
	}
}

// DecodeRow decodes a row from bytes
func DecodeRow(data []byte, colDefs []types.ColumnDef) (rowID uint64, row []types.Value, err error) {
	if len(data) < 10 {
		return 0, nil, fmt.Errorf("row data too short: %d bytes", len(data))
	}

	offset := 0

	// Read RowID
	rowID = binary.LittleEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Read column count
	colCount := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
	offset += 2

	row = make([]types.Value, colCount)

	// Decode each column
	for i := 0; i < colCount; i++ {
		var colDef types.ColumnDef
		if i < len(colDefs) {
			colDef = colDefs[i]
		}

		val, bytesRead, err := decodeColumn(data[offset:], colDef)
		if err != nil {
			return 0, nil, fmt.Errorf("column %d: %w", i, err)
		}
		row[i] = val
		offset += bytesRead
	}

	return rowID, row, nil
}

// decodeColumn decodes a single column value from bytes
func decodeColumn(data []byte, colDef types.ColumnDef) (types.Value, int, error) {
	if len(data) < 1 {
		return types.NewNullValue(), 0, fmt.Errorf("empty column data")
	}

	// Check for NULL marker
	if data[0] == ColNullMarker {
		return types.NewNullValue(), 1, nil
	}

	typeTag := data[0]
	dataType := types.DataType(typeTag)

	// Check for blob reference marker (only valid for BLOB/IMAGE types)
	// Format: [Type:1B][0xFE][BlobID:8B][Size:8B] = 18 bytes
	if (dataType == types.TypeBlob || dataType == types.TypeImage) && len(data) >= 18 && data[1] == ColBlobRefMarker {
		blobID := binary.LittleEndian.Uint64(data[2:10])
		blobSize := binary.LittleEndian.Uint64(data[10:18])
		return types.NewBlobRefValue(blobID, int64(blobSize)), 18, nil
	}

	switch dataType {
	case types.TypeInt, types.TypeSeq:
		if len(data) < 9 {
			return types.NewNullValue(), 0, fmt.Errorf("int data too short")
		}
		val := int64(binary.LittleEndian.Uint64(data[1:9]))
		return types.NewIntValue(val), 9, nil

	case types.TypeFloat:
		if len(data) < 9 {
			return types.NewNullValue(), 0, fmt.Errorf("float data too short")
		}
		val := binary.LittleEndian.Uint64(data[1:9])
		return types.NewFloatValue(float64(val)), 9, nil

	case types.TypeVarchar, types.TypeText, types.TypeChar:
		if len(data) < 5 {
			return types.NewNullValue(), 0, fmt.Errorf("varchar data too short")
		}
		length := binary.LittleEndian.Uint32(data[1:5])
		if len(data) < 5+int(length) {
			return types.NewNullValue(), 0, fmt.Errorf("varchar data truncated")
		}
		str := string(data[5 : 5+length])
		return types.NewStringValue(str), int(5 + length), nil

	case types.TypeBlob, types.TypeImage:
		if len(data) < 5 {
			return types.NewNullValue(), 0, fmt.Errorf("blob data too short")
		}
		length := binary.LittleEndian.Uint32(data[1:5])
		if len(data) < 5+int(length) {
			return types.NewNullValue(), 0, fmt.Errorf("blob data truncated")
		}
		blob := make([]byte, length)
		copy(blob, data[5:5+length])
		return types.NewBlobValue(blob), int(5 + length), nil

	case types.TypeDate, types.TypeTime, types.TypeDatetime:
		if len(data) < 5 {
			return types.NewNullValue(), 0, fmt.Errorf("datetime data too short")
		}
		length := binary.LittleEndian.Uint32(data[1:5])
		if len(data) < 5+int(length) {
			return types.NewNullValue(), 0, fmt.Errorf("datetime data truncated")
		}
		str := string(data[5 : 5+length])
		return types.NewStringValue(str), int(5 + length), nil

	default:
		// Use column definition type
		if colDef.Type != types.TypeNull {
			dataType = colDef.Type
		}
		// Try to read as string
		if len(data) >= 5 {
			length := binary.LittleEndian.Uint32(data[1:5])
			if len(data) >= 5+int(length) {
				str := string(data[5 : 5+length])
				return types.NewStringValue(str), int(5 + length), nil
			}
		}
		return types.NewNullValue(), 1, nil
	}
}

// InsertRowToPage inserts a row into a page
// Returns the row offset within the page, or error if page is full
func InsertRowToPage(page *PageV2, rowID uint64, row []types.Value) (int, error) {
	rowData := EncodeRow(rowID, row)
	rowSize := len(rowData)

	// Check if we can fit the row
	if !page.CanInsert(rowSize) {
		return -1, fmt.Errorf("page full, available: %d, needed: %d", page.AvailableSpace(), rowSize+2)
	}

	// Calculate where to store the row (from end of page)
	cellOffset := page.Header.FreeSpace - uint16(rowSize)

	// Write row data
	copy(page.Content[cellOffset:], rowData)

	// Add cell pointer
	page.AddCellPointer(cellOffset)

	// Update free space
	page.Header.FreeSpace = cellOffset

	// Mark as dirty
	page.Header.Flags |= PageFlagDirty

	// Return the index of the new cell
	return int(page.Header.CellCount) - 1, nil
}

// GetRowFromPage retrieves a row from a page by cell index
// Cell data is stored from the end of Content, growing toward the beginning.
// Cell pointers are stored from the beginning of Content, growing toward the end.
// Pointers are in decreasing order: pointer[0] is largest (first inserted), pointer[n-1] is smallest (last inserted).
// For each cell: data is at Content[pointer[index]:pointer[index-1]] or Content[pointer[0]:MaxContent] for index=0.
func GetRowFromPage(page *PageV2, index int, colDefs []types.ColumnDef) (rowID uint64, row []types.Value, err error) {
	if index < 0 || index >= int(page.Header.CellCount) {
		return 0, nil, fmt.Errorf("cell index out of range: %d", index)
	}

	cellStart := page.GetCellPointer(index)
	if cellStart == 0 || cellStart >= MaxPageContentV2 {
		return 0, nil, fmt.Errorf("invalid cell offset: %d", cellStart)
	}

	// Find end of cell data
	// First cell (index == 0, earliest inserted) extends to MaxPageContentV2
	// Other cells extend to the previous cell's pointer
	var cellEnd uint16
	if index == 0 {
		// First inserted cell has data extending to end of Content
		cellEnd = MaxPageContentV2
	} else {
		// Cell's data ends where the previous cell's data begins
		cellEnd = page.GetCellPointer(index - 1)
		if cellEnd == 0 || cellEnd >= MaxPageContentV2 {
			return 0, nil, fmt.Errorf("invalid previous cell offset: %d", cellEnd)
		}
	}

	// Validate bounds
	if cellStart >= cellEnd {
		return 0, nil, fmt.Errorf("invalid cell bounds: start=%d, end=%d", cellStart, cellEnd)
	}

	cellData := page.Content[cellStart:cellEnd]
	return DecodeRow(cellData, colDefs)
}

// DeleteRowFromPage marks a row as deleted (by setting fragmented space)
func DeleteRowFromPage(page *PageV2, index int) error {
	if index < 0 || index >= int(page.Header.CellCount) {
		return fmt.Errorf("cell index out of range: %d", index)
	}

	cellStart := page.GetCellPointer(index)
	if cellStart == 0 {
		return fmt.Errorf("cell already deleted or invalid")
	}

	// Calculate cell size
	var cellEnd uint16
	if index == 0 {
		cellEnd = MaxPageContentV2
	} else {
		cellEnd = page.GetCellPointer(index - 1)
	}

	if cellStart >= cellEnd {
		return fmt.Errorf("invalid cell bounds for deletion")
	}
	cellSize := cellEnd - cellStart

	// Add to fragmented space
	page.Header.Fragmented += cellSize

	// Clear the cell pointer
	page.SetCellPointer(index, 0)

	// Mark as dirty
	page.Header.Flags |= PageFlagDirty

	return nil
}

// GetCellData returns the raw cell data for a given index
// This is a helper function to avoid code duplication
func GetCellData(page *PageV2, index int) ([]byte, error) {
	if index < 0 || index >= int(page.Header.CellCount) {
		return nil, fmt.Errorf("cell index out of range: %d", index)
	}

	cellStart := page.GetCellPointer(index)
	if cellStart == 0 || cellStart >= MaxPageContentV2 {
		return nil, fmt.Errorf("invalid cell offset: %d", cellStart)
	}

	var cellEnd uint16
	if index == 0 {
		cellEnd = MaxPageContentV2
	} else {
		cellEnd = page.GetCellPointer(index - 1)
	}

	if cellStart >= cellEnd {
		return nil, fmt.Errorf("invalid cell bounds: start=%d, end=%d", cellStart, cellEnd)
	}

	return page.Content[cellStart:cellEnd], nil
}

// EstimateRowSize estimates the encoded size of a row
func EstimateRowSize(row []types.Value) int {
	size := 10 // RowID + ColCount

	for _, v := range row {
		if v.IsNull {
			size += 1
		} else if v.BlobRef != nil {
			size += 18 // External blob reference
		} else {
			switch v.Type {
			case types.TypeInt, types.TypeSeq, types.TypeFloat:
				size += 9
			case types.TypeVarchar, types.TypeText, types.TypeChar, types.TypeBlob, types.TypeImage, types.TypeDate, types.TypeTime, types.TypeDatetime:
				var dataLen int
				switch d := v.Data.(type) {
				case string:
					dataLen = len(d)
				case []byte:
					dataLen = len(d)
				default:
					dataLen = len(fmt.Sprintf("%v", d))
				}
				size += 5 + dataLen
			default:
				size += 10 // Estimate
			}
		}
	}

	return size
}