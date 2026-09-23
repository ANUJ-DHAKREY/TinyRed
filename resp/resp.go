package resp

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	TypeSimpleString string = "simple_string"
	TypeArray        string = "array"
	TypeSimpleError  string = "simple_error"
	TypeInteger      string = "integer"
	TypeBulkString   string = "bulk_string"
	TypeNull         string = "null"
	TypeBulkError    string = "bulk_error"
)

var TypeSymbols = map[string]byte{
	TypeSimpleString: '+',
	TypeArray:        '*',
	TypeSimpleError:  '-',
	TypeInteger:      ':',
	TypeBulkString:   '$',
	TypeNull:         '*',
	TypeBulkError:    '(',
}

type ErrorType string

const (
	ERR        ErrorType = "ERR"
	WRONGTYPE  ErrorType = "WRONGTYPE"
	EXECABORT  ErrorType = "EXECABORT"
	NOPERM     ErrorType = "NOPERM"
	READONLY   ErrorType = "READONLY"
	LOADING    ErrorType = "LOADING"
	BUSY       ErrorType = "BUSY"
	NOREPLICAS ErrorType = "NOREPLICAS"
)

const (
	ErrorMessageWrongType  string = "Operation against a key holding the wrong kind of value"
	ErrorMessageNotInteger string = "value is not an integer or out of range"
	ErrorMessageNotNumber  string = "value is not an number"
	ErrorMessageNotFloat   string = "value is not float or out of range"
)

// Pre-marshaled RESP arrays for the replica handshake, ready to write
// straight to the master connection.
//
// StaticRequestReplConfPort still needs the replica's listening port
// filled in (RESP bulk strings are length-prefixed, so the port's
// length/value can't be baked in at compile time). Build it with:
//
//	port := strconv.Itoa(myPort)
//	req := fmt.Sprintf(resp.StaticRequestReplConfPort, len(port), port)
const (
	StaticRequestPing         string = "*1\r\n$4\r\nPING\r\n"
	StaticRequestPsync2       string = "*3\r\n$5\r\nPSYNC\r\n$1\r\n?\r\n$2\r\n-1\r\n"
	StaticRequestReplConfPort string = "*3\r\n$8\r\nREPLCONF\r\n$14\r\nlistening-port\r\n$%d\r\n%s\r\n"
	StaticRequestReplConfCapa string = "*3\r\n$8\r\nREPLCONF\r\n$4\r\ncapa\r\n$6\r\npsync2\r\n"
)

type Value struct {
	Type string
	Val  RespType
}

type RespType interface {
	Unmarshal(reader *bufio.Reader) error
	Marshal() []byte
}

type Array struct {
	Value []string
}

type SimpleString struct {
	Value string
}

type SimpleError struct {
	Type    ErrorType
	Message string
}

type Integer struct {
	Value int64
}

type BulkString struct {
	Value string
}

type NullBulkString struct {
}

type NullArray struct {
}
type BulkError struct {
	Value string
}

func ReadResponse(reader *bufio.Reader) (any, error) {
	prefix, err := reader.Peek(1)
	if err != nil {
		return nil, err
	}

	switch prefix[0] {
	case TypeSymbols[TypeSimpleString]:
		value := &SimpleString{}
		return value, value.Unmarshal(reader)
	case TypeSymbols[TypeSimpleError]:
		value := &SimpleError{}
		return value, value.Unmarshal(reader)
	case TypeSymbols[TypeInteger]:
		value := &Integer{}
		return value, value.Unmarshal(reader)
	case TypeSymbols[TypeBulkString]:
		value := &BulkString{}
		return value, value.Unmarshal(reader)
	default:
		return nil, fmt.Errorf("unsupported RESP response type %q", prefix[0])
	}
}

func ReadHeader(reader *bufio.Reader) ([]byte, error) {
	data, err := reader.ReadBytes('\n')
	if err != nil {
		if err == io.EOF && len(data) > 0 {
			return nil, fmt.Errorf("reading from connection: %w", io.ErrUnexpectedEOF)
		}
		return nil, fmt.Errorf("reading from connection: %w", err)
	}

	if len(data) <= 1 {
		return nil, fmt.Errorf("insufficient data: expected at least 2 bytes and got %d", len(data))
	}

	if data[len(data)-2] != '\r' {
		return nil, fmt.Errorf("invalid RESP type header: expected header ending with \r\n and got %s", string(data))
	}

	return data, nil
}

func size(data []byte) (int, error) {
	sizeArr := data[1 : len(data)-2]
	count, err := strconv.Atoi(string(sizeArr))
	if err != nil {
		return 0, fmt.Errorf("invalid array RESP header: expecting size of Array got %s", string(sizeArr))
	}
	return count, nil
}

func (a *Array) Unmarshal(reader *bufio.Reader) error {
	//check the length of the elemets
	//for each element fetch and then parse it accordingly to its symbol
	//store result in a result array with any format
	//and then return it

	data, err := ReadHeader(reader)
	if err != nil {
		return err
	}
	symbol := data[0]
	if symbol != TypeSymbols[TypeArray] {
		return fmt.Errorf("unexpected RESP type expected %q, got %q", TypeSymbols[TypeArray], symbol)
	}
	size, err := size(data)
	if err != nil {
		return err
	}

	for i := 0; i < size; i++ {
		char, err := reader.Peek(1)
		if err != nil {
			return fmt.Errorf("reading from connection: %w", err)
		}
		symbol := char[0]
		var value string
		switch symbol {
		case TypeSymbols[TypeSimpleString]:
			ss := &SimpleString{}
			err = ss.Unmarshal(reader)
			if err != nil {
				return err
			}
			value = ss.Value
		case TypeSymbols[TypeBulkString]:
			bs := &BulkString{}
			err = bs.Unmarshal(reader)
			if err != nil {
				return err
			}
			value = bs.Value
		default:
			//fix this error with correct error message
			return fmt.Errorf("unexpected RESP type expected %q, got %q", TypeSymbols[TypeArray], symbol)
		}
		a.Value = append(a.Value, value)
	}

	return nil
}

func (a *Array) Marshal() []byte {
	//marshal format
	// *<number-of-elements>\r\n<element-1>...<element-n>
	//Empty array *0\r\n
	//example : *2\r\n$5\r\nhello\r\n$5\r\nworld\r\n
	count := len(a.Value)
	data := make([]byte, 0)
	data = fmt.Appendf(data, "*%d\r\n", count)
	for _, val := range a.Value {
		data = append(data, []byte(val)...)
	}
	return data
}

func (a *NullArray) Marshal() []byte {
	return []byte("*-1\r\n")
}

func (ss *SimpleString) Unmarshal(reader *bufio.Reader) error {
	data, err := reader.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("reading from connection: %w", err)
	}

	if len(data) <= 3 {
		return fmt.Errorf("RESP simple string: invalid data count, %q", len(data))
	}

	if data[0] != TypeSymbols[TypeSimpleString] {
		return fmt.Errorf("RESP simple string: expected %q, got %q", TypeSymbols[TypeSimpleString], data[0])
	}

	if data[len(data)-2] != '\r' {
		return fmt.Errorf("RESP simple string: missing CRLF terminator")
	}

	data = data[1 : len(data)-2]
	ss.Value = string(data)
	return nil
}

// +OK\r\n
func (a *SimpleString) Marshal() []byte {
	data := make([]byte, 0)
	data = fmt.Appendf(data, "+%s\r\n", a.Value)
	return data
}

func (ss *BulkString) Unmarshal(reader *bufio.Reader) error {

	data, err := ReadHeader(reader)
	if err != nil {
		return err
	}

	if data[0] != TypeSymbols[TypeBulkString] {
		return fmt.Errorf("RESP simple string: expected %q, got %q", TypeSymbols[TypeBulkString], data[0])
	}

	data = data[1 : len(data)-2]
	size, err := strconv.ParseInt(string(data), 10, 32)
	if err != nil {
		return fmt.Errorf("Resp bulk string: parsing size string ")
	}

	buffer := make([]byte, size+2)
	_, err = io.ReadFull(reader, buffer)

	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return fmt.Errorf("reading from connection: %w", io.ErrUnexpectedEOF)
		}
		return fmt.Errorf("reading from connection %w", err)
	}
	CRLF := string(buffer[len(buffer)-2:])
	if CRLF != "\r\n" {
		return fmt.Errorf("RESP bulk string: missing crlf terminator")
	}

	buffer = buffer[:len(buffer)-2]
	ss.Value = string(buffer)
	return nil
}

// $<length>\r\n<data>\r\n
func (a *BulkString) Marshal() []byte {
	data := make([]byte, 0, len(a.Value)+14)
	length := strconv.Itoa(len(a.Value))
	data = fmt.Appendf(data, "$%s\r\n%s\r\n", length, a.Value)
	return data
}

func (a *SimpleError) Unmarshal(reader *bufio.Reader) error {
	data, err := ReadHeader(reader)
	if err != nil {
		return err
	}
	if data[0] != TypeSymbols[TypeSimpleError] {
		return fmt.Errorf("RESP simple error: expected %q, got %q", TypeSymbols[TypeSimpleError], data[0])
	}

	parts := string(data[1 : len(data)-2])
	if parts == "" {
		return fmt.Errorf("RESP simple error: empty error")
	}

	fields := strings.SplitN(parts, " ", 2)
	a.Type = ErrorType(fields[0])
	if len(fields) == 2 {
		a.Message = fields[1]
	}
	return nil
}

// Marshal produces: -TYPE message\r\n
func (a *SimpleError) Marshal() []byte {

	// RESP error format: -TYPE message\r\n
	data := []byte{'-'}
	data = append(data, []byte(a.Type)...)
	data = append(data, ' ')
	data = append(data, []byte(a.Message)...)
	data = append(data, '\r', '\n')
	return data
}

func (a *SimpleError) Error() string {
	return string(a.Type) + ": " + a.Message
}

// --- Standard Redis error helpers ---

// ErrWrongArgCount returns the standard Redis error for wrong argument count.
// Redis format: "-ERR wrong number of arguments for 'cmd' command\r\n"
// can we make it a little specific or similar to redis??,
func ErrWrongArgCount(cmd string) []byte {
	e := &SimpleError{
		Type:    ERR,
		Message: fmt.Sprintf("wrong number of arguments for '%s' command", cmd),
	}
	data := e.Marshal()
	return data
}

// ErrUnknownCommand returns the standard Redis error for unknown commands.
// Redis format: "-ERR unknown command 'cmd'\r\n"
func ErrUnknownCommand(cmd string) []byte {
	e := &SimpleError{
		Type:    ERR,
		Message: fmt.Sprintf("unknown command '%s'", cmd),
	}
	data := e.Marshal()
	return data
}

func ErrGeneric() []byte {
	data := make([]byte, 0)
	data = fmt.Appendf(data, "internal error occured")
	return data
}

// ErrWrongType returns the standard Redis WRONGTYPE error.
// Redis format: "-WRONGTYPE Operation against a key holding the wrong kind of value\r\n"
func ErrWrongType() []byte {
	e := &SimpleError{
		Type:    WRONGTYPE,
		Message: ErrorMessageWrongType,
	}
	data := e.Marshal()
	return data
}

// ErrSyntax returns the standard Redis syntax error.
func ErrSyntax() []byte {
	e := &SimpleError{
		Type:    ERR,
		Message: "syntax error",
	}
	data := e.Marshal()
	return data
}

// ErrNotInteger returns the standard Redis error for non-integer values.
func ErrNotInteger() []byte {
	e := &SimpleError{
		Type:    ERR,
		Message: ErrorMessageNotInteger,
	}
	data := e.Marshal()
	return data
}

func (a *BulkError) Unmarshal(reader *bufio.Reader) error {

	return nil
}

func (a *BulkError) Marshal() []byte {

	return []byte{'1'}
}

func (a *NullBulkString) Unmarshal(reader *bufio.Reader) error {

	return nil
}

func (a *NullBulkString) Marshal() []byte {
	data := make([]byte, 0)
	return fmt.Appendf(data, "$-1\r\n")
}

func (a *Integer) Unmarshal(reader *bufio.Reader) error {
	data, err := ReadHeader(reader)
	if err != nil {
		return err
	}
	if data[0] != TypeSymbols[TypeInteger] {
		return fmt.Errorf("RESP integer: expected %q, got %q", TypeSymbols[TypeInteger], data[0])
	}

	value, err := strconv.ParseInt(string(data[1:len(data)-2]), 10, 64)
	if err != nil {
		return fmt.Errorf("RESP integer: %w", err)
	}
	a.Value = value
	return nil
}

func (a *Integer) Marshal() []byte {
	//:[<+|->]<value>\r\n
	data := make([]byte, 0)
	return fmt.Appendf(data, ":%d\r\n", a.Value)
}
