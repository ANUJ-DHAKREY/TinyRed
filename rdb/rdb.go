package rdb

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
	"tinyred/resp"
	"tinyred/store"
)

const (
	// 52 45 44 49 53              # Magic String "REDIS"
	MAGIC_STRING string = "REDIS"
)

const (
	LenEnc6Bit    byte = 0 // 00 — length in next 6 bits
	LenEnc14Bit   byte = 1 // 01 — length in next 14 bits
	LenEnc32Bit   byte = 2 // 10 — length in next 4 bytes
	LenEncSpecial byte = 3 // 11 — special string encoding
)

const (
	StrEncInt8  byte = 0 // 0xC0 — 8-bit integer as string
	StrEncInt16 byte = 1 // 0xC1 — 16-bit integer as string
	StrEncInt32 byte = 2 // 0xC2 — 32-bit integer as string
	StrEncLZF   byte = 3 // 0xC3 — LZF compressed string
)

const (
	Type8BitInteger   byte = 0
	Type16BitInteger  byte = 1
	Type32BitInteger  byte = 2
	TypeLZFCompressed byte = 3
)

const (
	OpAux       byte = 0xFA // 250
	OpSelectDB  byte = 0xFE // 254
	OpResizeDB  byte = 0xFB // 251
	OpExpirySec byte = 0xFD // 253
	OpExpiryMs  byte = 0xFC // 252
	OpEOF       byte = 0xFF // 255
)

const (
	EXPIRE_TIME_SEC       byte = 1
	EXPIRE_TIME_MILLI_SEC byte = 2
)

const (
	ValTypeString           byte = 0
	ValTypeList             byte = 1
	ValTypeSet              byte = 2
	ValTypeSortedSet        byte = 3
	ValTypeHash             byte = 4
	ValTypeZipmap           byte = 9
	ValTypeZiplist          byte = 10
	ValTypeIntset           byte = 11
	ValTypeSortedSetZiplist byte = 12
	ValTypeHashZiplist      byte = 13
	ValTypeListQuicklist    byte = 14
)

var (
	ErrInvalidMagic  error = errors.New("rdb: invalid magic string")
	ErrBadVersion    error = errors.New("rdb: unsupported version")
	ErrUnexpectedEOF error = errors.New("rdb: unexpected end of file")
	ErrBadChecksum   error = errors.New("rdb: checksum mismatch")
	ErrUnknownOpcode error = errors.New("rdb: unknown opcode")
)

type RDBMetadata struct {
	key   string
	value string
}

type ParsedKeyPair struct {
	// ----------------------------# Key-Value pair starts
	// FD $unsigned-int            # "expiry time in seconds", followed by 4 byte unsigned int
	// $value-type                 # 1 byte flag indicating the type of value
	// $string-encoded-key         # The key, encoded as a redis string
	// $encoded-value              # The value, encoding depends on $value-type
	ExpireType byte
	ValueType  byte
	Key        string
	Value      any //need to be value type eventually with marshal and unmarshal function
	ExpireAt   time.Time
}

// FE 00                       # Indicates database selector. db number = 00
// FB                          # Indicates a resizedb field
// $length-encoded-int         # Size of the corresponding hash table
// $length-encoded-int         # Size of the corresponding expire hash table
type DBMetaData struct {
	DBNumber int
}

type ResizeDBMetaData struct {
	HashTableSize       int
	ExpireHashTableSize int
}

type RDBProcessor interface {
	Load(dir string, store *store.Store) error
}

type RDBLoader struct {
}

func Init() RDBProcessor {
	return &RDBLoader{}
}

func GetTwoSignificantBits(bits byte) (lenEncFormat byte) {
	//hex: 0xC0 , decimal: 192 , binary: 11000000
	bits = bits & 0xC0
	bits = bits >> 6
	return bits
}

func parseLength(reader *bufio.Reader) (isSpecial bool, size int, err error) {
	bits, err := reader.ReadByte()
	if err != nil {
		return false, 0, err
	}
	switch GetTwoSignificantBits(bits) {
	case LenEnc6Bit:
		bits = bits & 0x3F
		return false, int(bits), nil
	case LenEnc14Bit:
		secondByte, err := reader.ReadByte()
		if err != nil {
			return false, 0, err
		}
		bits = bits & 0x3F
		byteArr := []byte{bits, secondByte}
		return false, int(binary.BigEndian.Uint16(byteArr)), nil
	case LenEnc32Bit:
		buf := make([]byte, 4)
		if _, err = io.ReadFull(reader, buf); err != nil {
			return false, 0, err
		}
		return false, int(binary.BigEndian.Uint32(buf)), nil
	case LenEncSpecial:
		return true, int(bits & 0x3F), nil
	default:
		return false, 0, errors.New("rdb: invalid length encoding")
	}
}

func ParseString(reader *bufio.Reader) (string, error) {
	isSpecial, size, err := parseLength(reader)
	if err != nil {
		return "", err
	}
	if !isSpecial {
		buf := make([]byte, size)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return "", err
		}
		return string(buf), nil
	}
	switch byte(size) {
	case StrEncInt8:
		number, err := reader.ReadByte()
		if err != nil {
			return "", err
		}
		return strconv.Itoa(int(int8(number))), nil
	case StrEncInt16:
		buf := make([]byte, 2)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return "", err
		}
		return strconv.Itoa(int(int16(binary.LittleEndian.Uint16(buf)))), nil
	case StrEncInt32:
		buf := make([]byte, 4)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return "", err
		}
		return strconv.Itoa(int(int32(binary.LittleEndian.Uint32(buf)))), nil
	case StrEncLZF:
		return "", errors.New("We do not support LZF string parsing")
	default:
		return "", errors.New("invalid string header received")
	}
}
func handleAux(reader *bufio.Reader) (RDBMetadata, error) {
	key, err := ParseString(reader)
	if err != nil {
		return RDBMetadata{}, err
	}
	value, err := ParseString(reader)
	if err != nil {
		return RDBMetadata{}, err
	}
	return RDBMetadata{key: key, value: value}, nil
}

func ParseLengthEncoding(reader *bufio.Reader) (int, error) {
	isSpecial, size, err := parseLength(reader)
	if err != nil {
		return 0, err
	}
	if isSpecial {
		return 0, errors.New("rdb: unexpected special encoding in length")
	}
	return size, nil
}
func ParseDBMetaData(reader *bufio.Reader) (DBMetaData, error) {
	// FE 00                       # Indicates database selector. db number = 00
	// FB                          # Indicates a resizedb field
	// $length-encoded-int         # Size of the corresponding hash table
	// $length-encoded-int         # Size of the corresponding expire hash table
	len, err := ParseLengthEncoding(reader)

	if err != nil {
		return DBMetaData{}, err
	}
	return DBMetaData{
		DBNumber: len,
	}, nil
}

func ParseResizeMetadata(reader *bufio.Reader) (ResizeDBMetaData, error) {
	// FB                          # Indicates a resizedb field
	// $length-encoded-int         # Size of the corresponding hash table
	// $length-encoded-int         # Size of the corresponding expire hash table
	var resizeMetadata ResizeDBMetaData
	hashSize, err := ParseLengthEncoding(reader)
	if err != nil {
		return resizeMetadata, err
	}
	expireHashSize, err := ParseLengthEncoding(reader)
	if err != nil {
		return resizeMetadata, err
	}
	return ResizeDBMetaData{
		HashTableSize:       hashSize,
		ExpireHashTableSize: expireHashSize,
	}, nil
}
func ParseExpireKey(reader *bufio.Reader, expireUnit byte) (ParsedKeyPair, error) {
	var expireAt time.Time
	switch expireUnit {
	case EXPIRE_TIME_SEC:
		// 4 bytes unsigned int, Unix timestamp in seconds
		buf := make([]byte, 4)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return ParsedKeyPair{}, err
		}
		expireAt = time.Unix(int64(binary.LittleEndian.Uint32(buf)), 0)
	case EXPIRE_TIME_MILLI_SEC:
		// 8 bytes unsigned long, Unix timestamp in milliseconds
		buf := make([]byte, 8)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return ParsedKeyPair{}, err
		}
		ms := int64(binary.LittleEndian.Uint64(buf))
		expireAt = time.Unix(ms/1000, (ms%1000)*int64(time.Millisecond))
	default:
		return ParsedKeyPair{}, errors.New("rdb: unknown expire unit")
	}

	// After expire comes: value-type, key, value (same as a normal key)
	parsedKey, err := ParseKey(reader)
	if err != nil {
		return ParsedKeyPair{}, err
	}
	parsedKey.ExpireAt = expireAt
	return parsedKey, nil
}

func ParseKey(reader *bufio.Reader) (ParsedKeyPair, error) {
	valueType, err := reader.ReadByte()
	if err != nil {
		return ParsedKeyPair{}, err
	}
	return parseKeyWithType(reader, valueType)
}

func parseKeyWithType(reader *bufio.Reader, valueType byte) (ParsedKeyPair, error) {

	key, err := ParseString(reader)
	if err != nil {
		return ParsedKeyPair{}, err
	}
	var value any

	switch valueType {
	case ValTypeString:
		{
			str, err := ParseString(reader)
			if err != nil {
				return ParsedKeyPair{}, err
			}
			value = str
		}
	case ValTypeHash, ValTypeHashZiplist, ValTypeIntset, ValTypeList, ValTypeListQuicklist, ValTypeSet, ValTypeSortedSet, ValTypeSortedSetZiplist, ValTypeZiplist, ValTypeZipmap:
		{
			return ParsedKeyPair{}, errors.New("unsupported value type")
		}
	default:
		{
			return ParsedKeyPair{}, errors.New("unknown value type")
		}
	}

	return ParsedKeyPair{
		Key:       key,
		ValueType: valueType,
		Value:     value,
	}, nil
}

func (r *RDBLoader) Load(filepath string, store *store.Store) error {
	//read the file if not accesible or do not exit then ignore the file
	f, err := os.Open(filepath)
	if err != nil {
		return nil
	}
	defer f.Close()

	reader := bufio.NewReader(f)
	magicString := make([]byte, 5)
	_, err = io.ReadFull(reader, magicString)
	if err != nil {
		return err
	}

	if string(magicString) != MAGIC_STRING {
		return ErrInvalidMagic
	}

	//ignoring RDB version
	RDBVersion := make([]byte, 4)
	_, err = io.ReadFull(reader, RDBVersion)
	if err != nil {
		return err
	}

	//printing metadata for now
	fmt.Printf("RDB version: %d", binary.LittleEndian.Uint32(RDBVersion))
	for {
		opCode, err := reader.ReadByte()
		if err != nil {
			return err
		}

		switch opCode {
		// FA                          # Auxiliary field
		// $string-encoded-key         # May contain arbitrary metadata
		// $string-encoded-value       # such as Redis version, creation time, used memory, ...
		case OpAux:
			metaData, err := handleAux(reader)
			if err != nil {
				return err
			}
			fmt.Println(metaData.key, " ", metaData.value)
		case OpSelectDB:
			DBMetaData, err := ParseDBMetaData(reader)
			if err != nil {
				return err
			}

			fmt.Println(DBMetaData.DBNumber)
		case OpResizeDB:
			{
				resizeMetadata, err := ParseResizeMetadata(reader)
				if err != nil {
					return err
				}
				fmt.Println(resizeMetadata)
			}
		case OpExpirySec:
			parsedData, err := ParseExpireKey(reader, EXPIRE_TIME_SEC)
			if err != nil {
				return err
			}
			if parsedData.ExpireAt.Unix() <= time.Now().Unix() {
				continue
			}
			entry := &resp.Entry{
				Type:     resp.EntryTypeString,
				Value:    parsedData.Value,
				ExpireAt: parsedData.ExpireAt,
			}
			store.Set(parsedData.Key, entry)
		case OpExpiryMs:
			parsedData, err := ParseExpireKey(reader, EXPIRE_TIME_MILLI_SEC)
			if err != nil {
				return err
			}

			if parsedData.ExpireAt.Unix() <= time.Now().Unix() {
				continue
			}
			entry := &resp.Entry{
				Type:     resp.EntryTypeString,
				Value:    parsedData.Value,
				ExpireAt: parsedData.ExpireAt,
			}
			store.Set(parsedData.Key, entry)
		case OpEOF:
			// TODO: verify CRC64 checksum from the next 8 bytes
			return nil
		default:
			// opCode is the value-type byte for a key without expiry
			parsedData, err := parseKeyWithType(reader, opCode)
			if err != nil {
				return err
			}
			entry := &resp.Entry{
				Type:  resp.EntryTypeString,
				Value: parsedData.Value,
			}
			store.Set(parsedData.Key, entry)
		}
	}
}
