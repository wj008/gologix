package types

import (
	"bytes"
	"errors"
	"github.com/wj008/gologix/lib"
	"io"
	"strings"
)

type DataType uint16

const (
	NULL  DataType = 0x00
	BOOL  DataType = 0xc1
	SINT  DataType = 0xc2
	INT   DataType = 0xc3
	DINT  DataType = 0xc4
	LINT  DataType = 0xc5
	USINT DataType = 0xc6
	UINT  DataType = 0xc7
	UDINT DataType = 0xc8
	ULINT DataType = 0xc9
	REAL  DataType = 0xca
	LREAL DataType = 0xcb

	STIME           DataType = 0xcc
	DATE            DataType = 0xcd
	TIME_AND_DAY    DataType = 0xce
	DATE_AND_STRING DataType = 0xcf
	STRING          DataType = 0xd0
	WORD            DataType = 0xd1
	DWORD           DataType = 0xd2
	BIT_STRING      DataType = 0xd3
	LWORD           DataType = 0xd4
	STRING2         DataType = 0xd5
	FTIME           DataType = 0xd6
	LTIME           DataType = 0xd7
	ITIME           DataType = 0xd8
	STRINGN         DataType = 0xd9
	SHORT_STRING    DataType = 0xda
	TIME            DataType = 0xdb
	EPATH           DataType = 0xdc
	ENGUNIT         DataType = 0xdd
	STRINGI         DataType = 0xde
	STRUCT          DataType = 0x2a0
	STRINGAB        DataType = 0xfce
)

func GetByteCount(dataType DataType) uint16 {
	switch dataType {
	case NULL:
		return 0
	case SINT, USINT, BOOL:
		return 1
	case INT, UINT:
		return 2
	case DINT, UDINT, REAL, BIT_STRING:
		return 4
	case LINT, ULINT, LREAL:
		return 8
	case STRUCT:
		return 88
	default:
		return 0
	}
}

func GetTypeValue(reader io.Reader, dataType DataType) (interface{}, uint32, error) {
	switch dataType {
	case NULL:
		return nil, 0, errors.New("返回类型为NULL，读取失败")
	case BOOL:
		result := int8(0)
		lib.ReadByte(reader, &result)
		bVal := result&1 > 0
		return bVal, 1, nil
	case SINT:
		result := int8(0)
		lib.ReadByte(reader, &result)
		return result, 1, nil
	case USINT:
		result := uint8(0)
		lib.ReadByte(reader, &result)
		return result, 1, nil
	case INT:
		result := int16(0)
		lib.ReadByte(reader, &result)
		return result, 2, nil
	case UINT:
		result := uint16(0)
		lib.ReadByte(reader, &result)
		return result, 2, nil
	case DINT:
		result := int32(0)
		lib.ReadByte(reader, &result)
		return result, 4, nil
	case UDINT, BIT_STRING:
		result := uint32(0)
		lib.ReadByte(reader, &result)
		return result, 4, nil
	case LINT:
		result := int64(0)
		lib.ReadByte(reader, &result)
		return result, 8, nil
	case ULINT:
		result := uint64(0)
		lib.ReadByte(reader, &result)
		return result, 8, nil
	case REAL:
		result := float32(0)
		lib.ReadByte(reader, &result)
		return result, 4, nil
	case LREAL:
		result := float64(0)
		lib.ReadByte(reader, &result)
		return result, 8, nil
	case STRUCT: //160
		offset := uint32(0)
		_tp1 := uint16(0)
		lib.ReadByte(reader, &_tp1)
		offset += 2
		if _tp1 == 0xfce {
			_len := uint32(0)
			lib.ReadByte(reader, &_len)
			offset += 4
			buf := make([]byte, _len)
			lib.ReadByte(reader, buf)
			offset += _len
			return string(buf), offset, nil
		} else {
			return nil, offset, errors.New("读取结构体信息失败")
		}
	case SHORT_STRING: //218
		offset := uint32(0)
		_len := uint8(0)
		lib.ReadByte(reader, &_len)
		offset += 1
		buf := make([]byte, _len)
		lib.ReadByte(reader, buf)
		offset += uint32(_len)
		return string(buf), offset, nil
	default:
		return nil, 0, errors.New("没有找到正确的数据类型")
	}
}

func GetBitOfWord(tagName string, word interface{}) (bool, error) {
	_, indexs := lib.ParseTagName(tagName)
	bitPos := 0
	if strings.HasSuffix(tagName, "]") {
		bitPos = indexs[0] & 0x1f
	} else {
		bitPos = indexs[0]
	}
	ret := make([]bool, 0)
	switch word.(type) {
	case uint8:
		for i := uint8(0); i < 8; i++ {
			val := (word.(uint8) & (1 << i)) > 0
			ret = append(ret, val)
		}
		break
	case uint16:
		for i := uint16(0); i < 16; i++ {
			val := (word.(uint16) & (1 << i)) > 0
			ret = append(ret, val)
		}
		break
	case uint32:
		for i := uint32(0); i < 32; i++ {
			val := (word.(uint32) & (1 << i)) > 0
			ret = append(ret, val)
		}
		break
	case uint64:
		for i := uint64(0); i < 64; i++ {
			val := (word.(uint64) & (1 << i)) > 0
			ret = append(ret, val)
		}
		break
	case int8:
		buffer := new(bytes.Buffer)
		lib.WriteByte(buffer, word)
		var newWord uint8
		lib.ReadByte(buffer, &newWord)
		for i := uint8(0); i < 8; i++ {
			val := (newWord & (1 << i)) > 0
			ret = append(ret, val)
		}
		break
	case int16:
		buffer := new(bytes.Buffer)
		lib.WriteByte(buffer, word)
		var newWord uint16
		lib.ReadByte(buffer, &newWord)
		for i := uint16(0); i < 16; i++ {
			val := (newWord & (1 << i)) > 0
			ret = append(ret, val)
		}
		break
	case int32:
		buffer := new(bytes.Buffer)
		lib.WriteByte(buffer, word)
		var newWord uint32
		lib.ReadByte(buffer, &newWord)
		for i := uint32(0); i < 32; i++ {
			val := (newWord & (1 << i)) > 0
			ret = append(ret, val)
		}
		break
	case int64:
		buffer := new(bytes.Buffer)
		lib.WriteByte(buffer, word)
		var newWord uint64
		lib.ReadByte(buffer, &newWord)
		for i := uint64(0); i < 64; i++ {
			val := (newWord & (1 << i)) > 0
			ret = append(ret, val)
		}
		break
	case float32:
		buffer := new(bytes.Buffer)
		lib.WriteByte(buffer, word)
		var newWord uint32
		lib.ReadByte(buffer, &newWord)
		for i := uint32(0); i < 64; i++ {
			val := (newWord & (1 << i)) > 0
			ret = append(ret, val)
		}
		break
	case float64:
		buffer := new(bytes.Buffer)
		lib.WriteByte(buffer, word)
		var newWord uint64
		lib.ReadByte(buffer, &newWord)
		for i := uint64(0); i < 64; i++ {
			val := (newWord & (1 << i)) > 0
			ret = append(ret, val)
		}
		break
	}
	if bitPos >= len(ret) {
		return false, errors.New("超出数据范围")
	}
	return ret[bitPos], nil
}

func WordsToBits(words []interface{}, elements uint16, dataType DataType, index int) []interface{} {
	bitPos := 0
	if dataType == BIT_STRING {
		bitPos = index % 32
	} else {
		bitPos = index
	}
	ret := make([]interface{}, 0)
	for _, word := range words {
		switch word.(type) {
		case bool:
			ret = append(ret, word.(bool))
			break
		case uint8:
			for i := uint8(0); i < 8; i++ {
				val := (word.(uint8) & (1 << i)) > 0
				ret = append(ret, val)
			}
			break
		case uint16:
			for i := uint16(0); i < 16; i++ {
				val := (word.(uint16) & (1 << i)) > 0
				ret = append(ret, val)
			}
			break
		case uint32:
			for i := uint32(0); i < 32; i++ {
				val := (word.(uint32) & (1 << i)) > 0
				ret = append(ret, val)
			}
			break
		case uint64:
			for i := uint64(0); i < 64; i++ {
				val := (word.(uint64) & (1 << i)) > 0
				ret = append(ret, val)
			}
			break
		case int8:
			buffer := new(bytes.Buffer)
			lib.WriteByte(buffer, word)
			var newWord uint8
			lib.ReadByte(buffer, &newWord)
			for i := uint8(0); i < 8; i++ {
				val := (newWord & (1 << i)) > 0
				ret = append(ret, val)
			}
			break
		case int16:
			buffer := new(bytes.Buffer)
			lib.WriteByte(buffer, word)
			var newWord uint16
			lib.ReadByte(buffer, &newWord)
			for i := uint16(0); i < 16; i++ {
				val := (newWord & (1 << i)) > 0
				ret = append(ret, val)
			}
			break
		case int32:
			buffer := new(bytes.Buffer)
			lib.WriteByte(buffer, word)
			var newWord uint32
			lib.ReadByte(buffer, &newWord)
			for i := uint32(0); i < 32; i++ {
				val := (newWord & (1 << i)) > 0
				ret = append(ret, val)
			}
			break
		case int64:
			buffer := new(bytes.Buffer)
			lib.WriteByte(buffer, word)
			var newWord uint64
			lib.ReadByte(buffer, &newWord)
			for i := uint64(0); i < 64; i++ {
				val := (newWord & (1 << i)) > 0
				ret = append(ret, val)
			}
			break
		case float32:
			buffer := new(bytes.Buffer)
			lib.WriteByte(buffer, word)
			var newWord uint32
			lib.ReadByte(buffer, &newWord)
			for i := uint32(0); i < 64; i++ {
				val := (newWord & (1 << i)) > 0
				ret = append(ret, val)
			}
			break
		case float64:
			buffer := new(bytes.Buffer)
			lib.WriteByte(buffer, word)
			var newWord uint64
			lib.ReadByte(buffer, &newWord)
			for i := uint64(0); i < 64; i++ {
				val := (newWord & (1 << i)) > 0
				ret = append(ret, val)
			}
			break
		}
	}
	ret = ret[bitPos : bitPos+int(elements)]
	return ret
}
