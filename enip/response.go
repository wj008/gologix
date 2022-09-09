package enip

import (
	"bytes"
	"github.com/wj008/gologix/lib"
	"github.com/wj008/gologix/types"
)

type Response struct {
	Sequence               uint16
	Service                uint8
	Reserved               uint8
	Status                 uint8
	SizeOfAdditionalStatus uint8
	AdditionalStatus       []byte
	DType                  types.DataType
	Data                   []byte
}

func ParserResponse(data []byte, readSeq bool) *Response {
	res := &Response{}
	reader := bytes.NewReader(data)
	if readSeq {
		lib.ReadByte(reader, &res.Sequence)
	}
	lib.ReadByte(reader, &res.Service)
	lib.ReadByte(reader, &res.Reserved)
	lib.ReadByte(reader, &res.Status)
	lib.ReadByte(reader, &res.SizeOfAdditionalStatus)
	if res.SizeOfAdditionalStatus > 0 {
		res.AdditionalStatus = make([]byte, res.SizeOfAdditionalStatus)
		lib.ReadByte(reader, res.AdditionalStatus)
	}
	if reader.Len() > 0 && (res.Status == 0 || res.Status == 6) {
		lib.ReadByte(reader, &res.DType)
		res.Data = make([]byte, reader.Len())
		lib.ReadByte(reader, res.Data)
	}
	return res
}
