package mysql

import (
	"bytes"
	"database/sql/driver"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

type Bitfield []byte

func NewBitfield(bitSize uint) Bitfield {
	return make(Bitfield, (bitSize+7)/8)
}

func (bits Bitfield) isSet(index uint) bool {
	return bits[index/8]&(1<<(index%8)) != 0
}

type eventType byte

const (
	UNKNOWN_EVENT eventType = iota
	START_EVENT_V3
	QUERY_EVENT
	STOP_EVENT
	ROTATE_EVENT
	INTVAR_EVENT
	LOAD_EVENT
	SLAVE_EVENT
	CREATE_FILE_EVENT
	APPEND_BLOCK_EVENT
	EXEC_LOAD_EVENT
	DELETE_FILE_EVENT
	NEW_LOAD_EVENT
	RAND_EVENT
	USER_VAR_EVENT
	FORMAT_DESCRIPTION_EVENT
	XID_EVENT
	BEGIN_LOAD_QUERY_EVENT
	EXECUTE_LOAD_QUERY_EVENT
	TABLE_MAP_EVENT
	WRITE_ROWS_EVENTv0
	UPDATE_ROWS_EVENTv0
	DELETE_ROWS_EVENTv0
	WRITE_ROWS_EVENTv1
	UPDATE_ROWS_EVENTv1
	DELETE_ROWS_EVENTv1
	INCIDENT_EVENT
	HEARTBEAT_EVENT
	IGNORABLE_EVENT
	ROWS_QUERY_EVENT
	WRITE_ROWS_EVENTv2
	UPDATE_ROWS_EVENTv2
	DELETE_ROWS_EVENTv2
	GTID_EVENT
	ANONYMOUS_GTID_EVENT
	PREVIOUS_GTIDS_EVENT
)

type eventFlag uint16

const (
	LOG_EVENT_BINLOG_IN_USE_F eventFlag = 1 << iota
	LOG_EVENT_FORCED_ROTATE_F
	LOG_EVENT_THREAD_SPECIFIC_F
	LOG_EVENT_SUPPRESS_USE_F
	LOG_EVENT_UPDATE_TABLE_MAP_VERSION_F
	LOG_EVENT_ARTIFICIAL_F
	LOG_EVENT_RELAY_LOG_F
	LOG_EVENT_IGNORABLE_F
	LOG_EVENT_NO_FILTER_F
	LOG_EVENT_MTS_ISOLATE_F
)

type EventHeader struct {
	Timestamp uint32
	EventType eventType
	ServerId  uint32
	EventSize uint32
	LogPos    uint32
	Flags     eventFlag
}

type GenericEvent struct {
	header EventHeader
	data   []byte
}

func parseGenericEvent(buf *bytes.Buffer) (event *GenericEvent, err error) {
	event = new(GenericEvent)
	err = binary.Read(buf, binary.LittleEndian, &event.header)
	event.data = buf.Bytes()
	return
}

func (event *GenericEvent) Header() *EventHeader {
	return &event.header
}

func (event *GenericEvent) Print() {
	event.header.Print()
	fmt.Printf("Event Data:\n%s\n\n", hex.Dump(event.data))
}

type RotateEvent struct {
	header   EventHeader
	position uint64
	filename string
}

func parseRotateEvent(buf *bytes.Buffer) (event *RotateEvent, err error) {
	event = new(RotateEvent)
	err = binary.Read(buf, binary.LittleEndian, &event.header)
	err = binary.Read(buf, binary.LittleEndian, &event.position)
	event.filename = buf.String()
	return
}

func (event *RotateEvent) Header() *EventHeader {
	return &event.header
}

func (event *RotateEvent) Print() {
	event.header.Print()
	fmt.Printf("position: %v, filename: %#v\n", event.position, event.filename)
}

type HeartbeatEvent struct {
	header   EventHeader
	position uint64
	filename string
}

func parsHeartbeatEvent(buf *bytes.Buffer) (event *HeartbeatEvent, err error) {
	event = new(HeartbeatEvent)
	err = binary.Read(buf, binary.LittleEndian, &event.header)
	err = binary.Read(buf, binary.LittleEndian, &event.position)
	return
}

func (event *HeartbeatEvent) Header() *EventHeader {
	return &event.header
}

func (event *HeartbeatEvent) Print() {
	event.header.Print()
	fmt.Printf("position: %v, filename: %#v\n", event.position, event.filename)
}

type QueryEvent struct {
	header        EventHeader
	slaveProxyId  uint32
	executionTime uint32
	errorCode     uint16
	schema        string
	statusVars    string
	query         string
}

func parseQueryEvent(buf *bytes.Buffer) (event *QueryEvent, err error) {
	var schemaLength byte
	var statusVarsLength uint16

	event = new(QueryEvent)
	err = binary.Read(buf, binary.LittleEndian, &event.header)
	err = binary.Read(buf, binary.LittleEndian, &event.slaveProxyId)
	err = binary.Read(buf, binary.LittleEndian, &event.executionTime)
	err = binary.Read(buf, binary.LittleEndian, &schemaLength)
	err = binary.Read(buf, binary.LittleEndian, &event.errorCode)
	err = binary.Read(buf, binary.LittleEndian, &statusVarsLength)
	event.statusVars = string(buf.Next(int(statusVarsLength)))
	event.schema = string(buf.Next(int(schemaLength)))
	_, err = buf.ReadByte()
	event.query = buf.String()
	return
}

func (event *QueryEvent) Header() *EventHeader {
	return &event.header
}

func (event *QueryEvent) Print() {
	event.header.Print()
	fmt.Printf("slaveProxyId: %v, executionTime: %v, errorCode: %v, schema: %v, statusVars: %#v, query: %#v\n",
		event.slaveProxyId, event.executionTime, event.errorCode, event.schema, event.statusVars, event.query)
}

type FormatDescriptionEvent struct {
	header                 EventHeader
	binlogVersion          uint16
	mysqlServerVersion     string
	createTimestamp        uint32
	eventHeaderLength      uint8
	eventTypeHeaderLengths []uint8
}

func parseFormatDescriptionEvent(buf *bytes.Buffer) (event *FormatDescriptionEvent, err error) {
	event = new(FormatDescriptionEvent)
	err = binary.Read(buf, binary.LittleEndian, &event.header)
	err = binary.Read(buf, binary.LittleEndian, &event.binlogVersion)
	event.mysqlServerVersion = string(buf.Next(50))
	err = binary.Read(buf, binary.LittleEndian, &event.createTimestamp)
	event.eventHeaderLength, err = buf.ReadByte()
	event.eventTypeHeaderLengths = buf.Bytes()
	return
}

func (event *FormatDescriptionEvent) Header() *EventHeader {
	return &event.header
}

func (event *FormatDescriptionEvent) Print() {
	event.header.Print()
	fmt.Printf("binlogVersion: %v, mysqlServerVersion: %v, createTimestamp: %v, eventHeaderLength: %v, eventTypeHeaderLengths: %#v\n",
		event.binlogVersion, event.mysqlServerVersion, event.createTimestamp, event.eventHeaderLength, event.eventTypeHeaderLengths)
}

type RowsEvent struct {
	header                EventHeader
	tableId               uint64
	tableMap              *TableMapEvent
	flags                 uint16
	columnsPresentBitmap1 Bitfield
	columnsPresentBitmap2 Bitfield
	rows                  []*[]driver.Value
}

type RowsEventV2 struct {
	header                EventHeader
	tableId               uint64
	tableMap              *TableMapEvent
	flags                 uint16
	columnsPresentBitmap1 Bitfield
	columnsPresentBitmap2 Bitfield
	rows                  []*[]driver.Value
}

func (parser *eventParser) parseRowsEventV2(buf *bytes.Buffer) (event *RowsEventV2, err error) {
	var columnCount uint64

	event = new(RowsEventV2)
	err = binary.Read(buf, binary.LittleEndian, &event.header)

	headerSize := parser.format.eventTypeHeaderLengths[event.header.EventType-1]
	var tableIdSize int
	if headerSize == 6 {
		tableIdSize = 4
	} else {
		tableIdSize = 6
	}
	event.tableId, err = readFixedLengthInteger(buf, tableIdSize)

	err = binary.Read(buf, binary.LittleEndian, &event.flags)
	columnCount, _, err = readLengthEncodedInt(buf)
	fmt.Println("cols=", columnCount)

	event.columnsPresentBitmap1 = Bitfield(buf.Next(int((columnCount + 7) / 8)))
	switch event.header.EventType {
	case UPDATE_ROWS_EVENTv1, UPDATE_ROWS_EVENTv2:
		event.columnsPresentBitmap2 = Bitfield(buf.Next(int((columnCount + 7) / 8)))
	}

	event.tableMap = parser.tableMap[event.tableId]
	for buf.Len() > 0 {
		var row []driver.Value
		row, err = parseEventRow(buf, event.tableMap)
		if err != nil {
			return
		}

		event.rows = append(event.rows, &row)
	}

	return
}

func (event *RowsEventV2) Header() *EventHeader {
	return &event.header
}

func (event *RowsEventV2) Print() {
	event.header.Print()
}

func parseEventRow(buf *bytes.Buffer, tableMap *TableMapEvent) (row []driver.Value, e error) {
	columnsCount := len(tableMap.columnTypes)

	row = make([]driver.Value, columnsCount)

	bitFieldSize := (columnsCount + 7) / 8
	nullBitMap := Bitfield(buf.Next(bitFieldSize))

	//var fNull bool
	for i := 0; i < columnsCount; i++ {
		if nullBitMap.isSet(uint(i)) {
			row[i] = nil
			continue
		}

		_fieldType := tableMap.columnTypes[i]
		var fieldLen uint16

		switch _fieldType {
		case fieldTypeNULL:
			row[i] = nil

		case fieldTypeTiny:
			var b byte
			b, e = buf.ReadByte()
			row[i] = int64(b)

		case fieldTypeShort:
			var short int16
			e = binary.Read(buf, binary.LittleEndian, &short)
			row[i] = int64(short)

		case fieldTypeYear:
			var b byte
			b, e = buf.ReadByte()
			if e == nil && b != 0 {
				row[i] = time.Date(int(b)+1900, time.January, 0, 0, 0, 0, 0, time.UTC)
			}

		case fieldTypeInt24:
			row[i], e = readFixedLengthInteger(buf, 3)

		case fieldTypeLong:
			var long int32
			e = binary.Read(buf, binary.LittleEndian, &long)
			row[i] = int64(long)

		case fieldTypeLongLong:
			var longlong int64
			e = binary.Read(buf, binary.LittleEndian, &longlong)
			row[i] = longlong

		case fieldTypeFloat:
			var float float32
			e = binary.Read(buf, binary.LittleEndian, &float)
			row[i] = float64(float)

		case fieldTypeDouble:
			var double float64
			e = binary.Read(buf, binary.LittleEndian, &double)
			row[i] = double

		case fieldTypeDecimal, fieldTypeNewDecimal:
			return nil, fmt.Errorf("parseEventRow unimplemented for field type %s", fieldTypeName(tableMap.columnTypes[i]))

		case fieldTypeVarChar:
			maxLength := tableMap.columnMeta[i]
			var length int
			if maxLength > 255 {
				var short uint16
				e = binary.Read(buf, binary.LittleEndian, &short)
				length = int(short)
			} else {
				var b byte
				b, e = buf.ReadByte()
				length = int(b)
			}
			if buf.Len() < length {
				e = io.EOF
			}
			row[i] = string(buf.Next(length))

		case fieldTypeString:
			if fieldLen < 256 {
				var _fieldLenU8 uint8
				e = binary.Read(buf, binary.LittleEndian, &_fieldLenU8)
				row[i] = string(buf.Next(int(_fieldLenU8)))
			} else {
				var _fieldLenU16 uint8
				e = binary.Read(buf, binary.LittleEndian, &_fieldLenU16)
				row[i] = string(buf.Next(int(_fieldLenU16)))
			}
		case fieldTypeBLOB:
			var length uint64
			length, e = readFixedLengthInteger(buf, int(tableMap.columnMeta[i]))
			row[i] = string(buf.Next(int(length)))
		case fieldTypeBit, fieldTypeEnum,
			fieldTypeSet, fieldTypeTinyBLOB, fieldTypeMediumBLOB,
			fieldTypeLongBLOB, fieldTypeVarString,
			fieldTypeGeometry:

			return nil, fmt.Errorf("parseEventRow unimplemented for field type %s", fieldTypeName(tableMap.columnTypes[i]))

		case fieldTypeDate, fieldTypeNewDate:
			return nil, fmt.Errorf("parseEventRow unimplemented for field type %s", fieldTypeName(tableMap.columnTypes[i]))

		case fieldTypeTime:
			return nil, fmt.Errorf("parseEventRow unimplemented for field type %s", fieldTypeName(tableMap.columnTypes[i]))

		case fieldTypeTimestamp:
			return nil, fmt.Errorf("parseEventRow unimplemented for field type %s", fieldTypeName(tableMap.columnTypes[i]))
		case fieldTypeDateTime:
			var t int64
			e = binary.Read(buf, binary.LittleEndian, &t)

			second := int(t % 100)
			minute := int((t % 10000) / 100)
			hour := int((t % 1000000) / 10000)

			d := int(t / 1000000)
			day := d % 100
			month := time.Month((d % 10000) / 100)
			year := d / 10000

			row[i] = time.Date(year, month, day, hour, minute, second, 0, time.UTC)

		default:
			return nil, fmt.Errorf("Unknown FieldType %d", tableMap.columnTypes[i])
		}
		if e != nil {
			return nil, e
		}
	}
	return
}

func (parser *eventParser) parseRowsEvent(buf *bytes.Buffer) (event *RowsEvent, err error) {
	var columnCount uint64

	event = new(RowsEvent)
	err = binary.Read(buf, binary.LittleEndian, &event.header)

	headerSize := parser.format.eventTypeHeaderLengths[event.header.EventType-1]
	var tableIdSize int
	if headerSize == 6 {
		tableIdSize = 4
	} else {
		tableIdSize = 6
	}
	event.tableId, err = readFixedLengthInteger(buf, tableIdSize)

	err = binary.Read(buf, binary.LittleEndian, &event.flags)
	columnCount, _, err = readLengthEncodedInt(buf)

	event.columnsPresentBitmap1 = Bitfield(buf.Next(int((columnCount + 7) / 8)))
	switch event.header.EventType {
	case UPDATE_ROWS_EVENTv1, UPDATE_ROWS_EVENTv2:
		event.columnsPresentBitmap2 = Bitfield(buf.Next(int((columnCount + 7) / 8)))
	}

	event.tableMap = parser.tableMap[event.tableId]
	for buf.Len() > 0 {
		var row []driver.Value
		row, err = parseEventRow(buf, event.tableMap)
		if err != nil {
			return
		}

		event.rows = append(event.rows, &row)
	}

	return
}

func (event *RowsEvent) Header() *EventHeader {
	return &event.header
}

func (event *RowsEvent) Print() {
	event.header.Print()
	fmt.Printf("tableId: %v, flags: %v, columnsPresentBitmap1: %x, columnsPresentBitmap2: %x\n",
		event.tableId, event.flags, event.columnsPresentBitmap1, event.columnsPresentBitmap2)

	tableMap := event.tableMap
	for i, row := range event.rows {
		fmt.Printf("row[%d]:\n", i)
		for j, col := range *row {
			colType := tableMap.columnTypes[j]
			typeName := fieldTypeName(colType)
			switch colType {
			case fieldTypeVarChar, fieldTypeBLOB:
				fmt.Printf("  %s: %#v\n", typeName, col)
			default:
				fmt.Printf("  %s: %v\n", typeName, col)
			}
		}
	}
}

type TableMapEvent struct {
	header      EventHeader
	tableId     uint64
	flags       uint16
	schemaName  string
	tableName   string
	columnTypes []fieldType
	columnMeta  []uint16
	nullBitmap  Bitfield
}

func (event *TableMapEvent) parseColumnMetadata(data []byte) error {
	pos := 0
	event.columnMeta = make([]uint16, len(event.columnTypes))
	for i, t := range event.columnTypes {
		switch t {
		case fieldTypeString,
			fieldTypeVarString,
			fieldTypeVarChar,
			fieldTypeDecimal,
			fieldTypeNewDecimal,
			fieldTypeEnum,
			fieldTypeSet:
			event.columnMeta[i] = bytesToUint16(data[pos : pos+2])
			pos += 2

		case fieldTypeBLOB,
			fieldTypeDouble,
			fieldTypeFloat,
			fieldTypeGeometry,
			fieldTypeTime2,
			fieldTypeDatetime2,
			fieldTypeTimestamp2:
			event.columnMeta[i] = uint16(data[pos])
			pos += 1

		case fieldTypeBit,
			fieldTypeDate,
			fieldTypeDateTime,
			fieldTypeTimestamp,
			fieldTypeTime,
			fieldTypeTiny,
			fieldTypeShort,
			fieldTypeInt24,
			fieldTypeLong,
			fieldTypeLongLong,
			fieldTypeNULL,
			fieldTypeYear,
			fieldTypeNewDate,
			fieldTypeTinyBLOB,
			fieldTypeMediumBLOB,
			fieldTypeLongBLOB:
			event.columnMeta[i] = 0

		default:
			return fmt.Errorf("Unknown FieldType %d", t)
		}
	}
	return nil
}

func (parser *eventParser) parseTableMapEvent(buf *bytes.Buffer) (event *TableMapEvent, err error) {
	var byteLength byte
	var columnCount, variableLength uint64

	event = new(TableMapEvent)
	err = binary.Read(buf, binary.LittleEndian, &event.header)
	if err != nil {
		return
	}

	headerSize := parser.format.eventTypeHeaderLengths[event.header.EventType-1]
	var tableIdSize int
	if headerSize == 6 {
		tableIdSize = 4
	} else {
		tableIdSize = 6
	}
	event.tableId, err = readFixedLengthInteger(buf, tableIdSize)

	err = binary.Read(buf, binary.LittleEndian, &event.flags)
	byteLength, err = buf.ReadByte()
	event.schemaName = string(buf.Next(int(byteLength)))
	_, err = buf.ReadByte()
	byteLength, err = buf.ReadByte()
	event.tableName = string(buf.Next(int(byteLength)))
	_, err = buf.ReadByte()

	columnCount, _, err = readLengthEncodedInt(buf)
	event.columnTypes = make([]fieldType, columnCount)
	columnData := buf.Next(int(columnCount))
	for i, b := range columnData {
		event.columnTypes[i] = fieldType(b)
	}

	variableLength, _, err = readLengthEncodedInt(buf)
	if err = event.parseColumnMetadata(buf.Next(int(variableLength))); err != nil {
		return
	}

	if buf.Len() < int((columnCount+7)/8) {
		err = io.EOF
	}
	event.nullBitmap = Bitfield(buf.Next(int((columnCount + 7) / 8)))

	return
}

func (event *TableMapEvent) Header() *EventHeader {
	return &event.header
}

func (event *TableMapEvent) Print() {
	event.header.Print()
	fmt.Printf("tableId: %v, flags: %v, schemaName: %v, tableName: %v, columnTypes: %v, columnMeta = %v, nullBitmap = %x\n",
		event.tableId, event.flags, event.schemaName, event.tableName, event.columnTypeNames(), event.columnMeta, event.nullBitmap)
}

func fieldTypeName(t fieldType) string {
	switch t {
	case fieldTypeDecimal:
		return "FIELD_TYPE_DECIMAL"
	case fieldTypeTiny:
		return "FIELD_TYPE_TINY"
	case fieldTypeShort:
		return "FIELD_TYPE_SHORT"
	case fieldTypeLong:
		return "FIELD_TYPE_LONG"
	case fieldTypeFloat:
		return "FIELD_TYPE_FLOAT"
	case fieldTypeDouble:
		return "FIELD_TYPE_DOUBLE"
	case fieldTypeNULL:
		return "FIELD_TYPE_NULL"
	case fieldTypeTimestamp:
		return "fieldTypeTimeSTAMP"
	case fieldTypeLongLong:
		return "FIELD_TYPE_LONGLONG"
	case fieldTypeInt24:
		return "FIELD_TYPE_INT24"
	case fieldTypeDate:
		return "FIELD_TYPE_DATE"
	case fieldTypeTime:
		return "FIELD_TYPE_TIME"
	case fieldTypeDateTime:
		return "FIELD_TYPE_DATE_TIME"
	case fieldTypeYear:
		return "FIELD_TYPE_YEAR"
	case fieldTypeNewDate:
		return "FIELD_TYPE_NEW_DATE"
	case fieldTypeVarChar:
		return "FIELD_TYPE_VARCHAR"
	case fieldTypeBit:
		return "FIELD_TYPE_BIT"
	case fieldTypeNewDecimal:
		return "FIELD_TYPE_NEWDECIMAL"
	case fieldTypeEnum:
		return "fieldTypeEnum"
	case fieldTypeSet:
		return "FIELD_TYPE_SET"
	case fieldTypeTinyBLOB:
		return "FIELD_TYPE_TINY_BLOB"
	case fieldTypeMediumBLOB:
		return "FIELD_TYPE_MEDIUM_BLOB"
	case fieldTypeLongBLOB:
		return "FIELD_TYPE_LONG_BLOB"
	case fieldTypeBLOB:
		return "FIELD_TYPE_BLOB"
	case fieldTypeVarString:
		return "FIELD_TYPE_VAR_STRING"
	case fieldTypeString:
		return "FIELD_TYPE_STRING"
	case fieldTypeGeometry:
		return "FIELD_TYPE_GEOMETRY"
	}
	return fmt.Sprintf("%d", t)
}

func (event *TableMapEvent) columnTypeNames() (names []string) {
	names = make([]string, len(event.columnTypes))
	for i, t := range event.columnTypes {
		names[i] = fieldTypeName(t)
	}
	return
}

type BinlogEvent interface {
	Header() *EventHeader
	Print()
}

func (parser *eventParser) parseEvent(data []byte) (event BinlogEvent, err error) {
	buf := bytes.NewBuffer(data)

	switch eventType(data[4]) {
	case FORMAT_DESCRIPTION_EVENT:
		parser.format, err = parseFormatDescriptionEvent(buf)
		event = parser.format
		return
	case QUERY_EVENT:
		return parseQueryEvent(buf)
	case ROTATE_EVENT:
		return parseRotateEvent(buf)
	case HEARTBEAT_EVENT:
		return parsHeartbeatEvent(buf)
	case TABLE_MAP_EVENT:
		var tableMapEvent *TableMapEvent
		tableMapEvent, err = parser.parseTableMapEvent(buf)
		parser.tableMap[tableMapEvent.tableId] = tableMapEvent
		event = tableMapEvent
		return
	case WRITE_ROWS_EVENTv1, UPDATE_ROWS_EVENTv1, DELETE_ROWS_EVENTv1:
		return parser.parseRowsEvent(buf)
	case UPDATE_ROWS_EVENTv2:
		fmt.Printf("parseRowsEventV2 Data:\n%s\n\n", hex.Dump(data))
		return parser.parseRowsEventV2(buf)
	default:
		return parseGenericEvent(buf)
	}
	return
}

func (header *EventHeader) Read(data []byte) error {
	buf := bytes.NewBuffer(data)
	return binary.Read(buf, binary.LittleEndian, header)
}

func (header *EventHeader) EventName() string {
	switch header.EventType {
	case UNKNOWN_EVENT:
		return "UNKNOWN_EVENT"
	case START_EVENT_V3:
		return "START_EVENT_V3"
	case QUERY_EVENT:
		return "QUERY_EVENT"
	case STOP_EVENT:
		return "STOP_EVENT"
	case ROTATE_EVENT:
		return "ROTATE_EVENT"
	case INTVAR_EVENT:
		return "INTVAR_EVENT"
	case LOAD_EVENT:
		return "LOAD_EVENT"
	case SLAVE_EVENT:
		return "SLAVE_EVENT"
	case CREATE_FILE_EVENT:
		return "CREATE_FILE_EVENT"
	case APPEND_BLOCK_EVENT:
		return "APPEND_BLOCK_EVENT"
	case EXEC_LOAD_EVENT:
		return "EXEC_LOAD_EVENT"
	case DELETE_FILE_EVENT:
		return "DELETE_FILE_EVENT"
	case NEW_LOAD_EVENT:
		return "NEW_LOAD_EVENT"
	case RAND_EVENT:
		return "RAND_EVENT"
	case USER_VAR_EVENT:
		return "USER_VAR_EVENT"
	case FORMAT_DESCRIPTION_EVENT:
		return "FORMAT_DESCRIPTION_EVENT"
	case XID_EVENT:
		return "XID_EVENT"
	case BEGIN_LOAD_QUERY_EVENT:
		return "BEGIN_LOAD_QUERY_EVENT"
	case EXECUTE_LOAD_QUERY_EVENT:
		return "EXECUTE_LOAD_QUERY_EVENT"
	case TABLE_MAP_EVENT:
		return "TABLE_MAP_EVENT"
	case WRITE_ROWS_EVENTv0:
		return "WRITE_ROWS_EVENTv0"
	case UPDATE_ROWS_EVENTv0:
		return "UPDATE_ROWS_EVENTv0"
	case DELETE_ROWS_EVENTv0:
		return "DELETE_ROWS_EVENTv0"
	case WRITE_ROWS_EVENTv1:
		return "WRITE_ROWS_EVENTv1"
	case UPDATE_ROWS_EVENTv1:
		return "UPDATE_ROWS_EVENTv1"
	case DELETE_ROWS_EVENTv1:
		return "DELETE_ROWS_EVENTv1"
	case INCIDENT_EVENT:
		return "INCIDENT_EVENT"
	case HEARTBEAT_EVENT:
		return "HEARTBEAT_EVENT"
	case IGNORABLE_EVENT:
		return "IGNORABLE_EVENT"
	case ROWS_QUERY_EVENT:
		return "ROWS_QUERY_EVENT"
	case WRITE_ROWS_EVENTv2:
		return "WRITE_ROWS_EVENTv2"
	case UPDATE_ROWS_EVENTv2:
		return "UPDATE_ROWS_EVENTv2"
	case DELETE_ROWS_EVENTv2:
		return "DELETE_ROWS_EVENTv2"
	case GTID_EVENT:
		return "GTID_EVENT"
	case ANONYMOUS_GTID_EVENT:
		return "ANONYMOUS_GTID_EVENT"
	case PREVIOUS_GTIDS_EVENT:
		return "PREVIOUS_GTIDS_EVENT"
	}
	return fmt.Sprintf("%d", header.EventType)
}

func (header *EventHeader) FlagNames() (names []string) {
	if header.Flags&LOG_EVENT_BINLOG_IN_USE_F != 0 {
		names = append(names, "LOG_EVENT_BINLOG_IN_USE_F")
	}
	if header.Flags&LOG_EVENT_FORCED_ROTATE_F != 0 {
		names = append(names, "LOG_EVENT_FORCED_ROTATE_F")
	}
	if header.Flags&LOG_EVENT_THREAD_SPECIFIC_F != 0 {
		names = append(names, "LOG_EVENT_THREAD_SPECIFIC_F")
	}
	if header.Flags&LOG_EVENT_SUPPRESS_USE_F != 0 {
		names = append(names, "LOG_EVENT_SUPPRESS_USE_F")
	}
	if header.Flags&LOG_EVENT_UPDATE_TABLE_MAP_VERSION_F != 0 {
		names = append(names, "LOG_EVENT_UPDATE_TABLE_MAP_VERSION_F")
	}
	if header.Flags&LOG_EVENT_ARTIFICIAL_F != 0 {
		names = append(names, "LOG_EVENT_ARTIFICIAL_F")
	}
	if header.Flags&LOG_EVENT_RELAY_LOG_F != 0 {
		names = append(names, "LOG_EVENT_RELAY_LOG_F")
	}
	if header.Flags&LOG_EVENT_IGNORABLE_F != 0 {
		names = append(names, "LOG_EVENT_IGNORABLE_F")
	}
	if header.Flags&LOG_EVENT_NO_FILTER_F != 0 {
		names = append(names, "LOG_EVENT_NO_FILTER_F")
	}
	if header.Flags&LOG_EVENT_MTS_ISOLATE_F != 0 {
		names = append(names, "LOG_EVENT_MTS_ISOLATE_F")
	}
	if header.Flags & ^(LOG_EVENT_MTS_ISOLATE_F<<1-1) != 0 { // unknown flags
		names = append(names, string(header.Flags & ^(LOG_EVENT_MTS_ISOLATE_F<<1-1)))
	}
	return names
}

func (header *EventHeader) Print() {
	fmt.Printf("Timestamp: %v, EventType: %v, ServerId: %v, EventSize: %v, LogPos: %v, Flags: %v\n",
		time.Unix(int64(header.Timestamp), 0), header.EventName(), header.ServerId, header.EventSize, header.LogPos, header.FlagNames())
}

type eventParser struct {
	format   *FormatDescriptionEvent
	tableMap map[uint64]*TableMapEvent
}

func newEventParser() (parser *eventParser) {
	parser = new(eventParser)
	parser.tableMap = make(map[uint64]*TableMapEvent)
	return
}

func (mc *mysqlConn) RegisterSlave(serverId uint32) error {

	if e := mc.exec("set wait_timeout=9999999"); e != nil {
		return e
	}
	if e := mc.exec("set net_write_timeout=1800"); e != nil {
		return e
	}
	if e := mc.exec("set net_read_timeout=1800"); e != nil {
		return e
	}
	if e := mc.exec("set names 'binary'"); e != nil {
		return e
	}
	if e := mc.exec("set @master_binlog_checksum= @@global.binlog_checksum"); e != nil {
		return e
	}
	if e := mc.exec("set @slave_uuid=uuid()"); e != nil {
		return e
	}
	if e := mc.exec("SET @master_heartbeat_period=15"); e != nil {
		return e
	}

	mc.sequence = 0

	port := uint16(234)

	data := make([]byte, 4+1+4+3+2+8)
	data[4] = comRegisterSlave

	data[5] = byte(serverId)
	data[6] = byte(serverId >> 8)
	data[7] = byte(serverId >> 16)
	data[8] = byte(serverId >> 24)

	data[12] = byte(port)
	data[13] = byte(port >> 8)

	if e := mc.writePacket(data); e != nil {
		return e
	}

	return mc.readResultOK()
}

func (mc *mysqlConn) DumpBinlog(serverId uint32, filename string, position uint32) (driver.Rows, error) {
	parser := newEventParser()
	flags := uint16(2)

	for {
		mc.sequence = 0
		data := make([]byte, 4+1+4+2+4+len(filename))

		data[4] = comBinlogDump

		// binlog-pos [32 bit]
		data[5] = byte(position)
		data[6] = byte(position >> 8)
		data[7] = byte(position >> 16)
		data[8] = byte(position >> 24)

		// flags [16 bit]
		data[9] = byte(flags)
		data[10] = byte(flags >> 8)

		// server-id [32 bit]
		data[11] = byte(serverId)
		data[12] = byte(serverId >> 8)
		data[13] = byte(serverId >> 16)
		data[14] = byte(serverId >> 24)

		e := mc.writePacket(data)
		if e != nil {
			return nil, e
		}

		for {
			pkt, e := mc.readPacket()
			if e != nil {
				return nil, e
			} else if pkt[0] == 254 { // EOF packet
				break
			}
			if pkt[0] == 0 {
				event, e := parser.parseEvent(pkt[1:])
				event.Print()

				if e != nil {
					if e.Error() != "EOF" {
						return nil, e
					}
				}
				switch event.(type) {
				case *RotateEvent:
					rotateEvent := event.(*RotateEvent)
					filename = rotateEvent.filename
					position = uint32(rotateEvent.position)
				case *HeartbeatEvent:
					continue
				//case *RowsEvent:
				//	rowsEvent := event.(*RowsEvent)
				default:
					event.Print()
				}
			} else {
				fmt.Printf("Unknown packet:\n%s\n\n", hex.Dump(pkt))
			}
			fmt.Println()
		}
	}

	return nil, nil
}
