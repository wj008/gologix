package lib

import (
	"encoding/binary"
	"io"
)

func WriteByte(writer io.Writer, target interface{}) {
	e := binary.Write(writer, binary.LittleEndian, target)
	if e != nil {
		panic(e)
	}
}

func ReadByte(reader io.Reader, target interface{}) {
	e := binary.Read(reader, binary.LittleEndian, target)
	if e != nil {
		panic(e)
	}
}
