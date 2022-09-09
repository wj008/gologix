package enip

import (
	"bytes"
	"github.com/wj008/gologix/lib"
)

type CPFItem struct {
	TypeID CPFType
	Length uint16
	Data   []byte
}

//BuildCPF 创建数据集合
func BuildCPF(dataItems []*CPFItem) []byte {
	itemCount := uint16(len(dataItems))
	buffer := new(bytes.Buffer)
	lib.WriteByte(buffer, itemCount)
	for i := 0; i < len(dataItems); i++ {
		item := dataItems[i]
		item.Length = uint16(len(item.Data))
		lib.WriteByte(buffer, item.TypeID)
		lib.WriteByte(buffer, item.Length)
		lib.WriteByte(buffer, item.Data)
	}
	return buffer.Bytes()
}

//ParserCPF 解析据集合
func ParserCPF(buf []byte) []*CPFItem {
	var itemCount uint16
	reader := bytes.NewReader(buf)
	lib.ReadByte(reader, &itemCount)
	result := make([]*CPFItem, 0)
	for i := 0; i < int(itemCount); i++ {
		item := &CPFItem{}
		lib.ReadByte(reader, &item.TypeID)
		lib.ReadByte(reader, &item.Length)
		item.Data = make([]byte, item.Length)
		lib.ReadByte(reader, &item.Data)
		result = append(result, item)
	}
	return result
}
