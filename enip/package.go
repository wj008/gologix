package enip

import (
	"bytes"
	"errors"
	"github.com/wj008/gologix/epath"
	"github.com/wj008/gologix/epath/segment"
	"github.com/wj008/gologix/lib"
	"github.com/wj008/gologix/types"
	"math"
	"strings"
)

type Header struct {
	Command   Command //H uint16
	Length    uint16  //H
	SessionId uint32  //I
	Status    Status  //I uint32
	ContextId uint64  //Q
	Options   uint32  //I
}

type Package struct {
	Header
	Data         []byte
	SequenceId   uint32
	ConnectionID uint32
	DataItems    []*CPFItem
}

func NewPackage(cmd Command, data []byte) *Package {
	p := &Package{}
	p.Command = cmd
	p.Data = data
	return p
}

type MessageRouterRequest struct {
	Service         CIPServType
	RequestPathSize uint8
	RequestPath     []byte
	RequestData     []byte
}

type UnconnectedSend struct {
	TimeTick       uint8
	TimeOutTicks   uint8
	MessageRequest []byte
	RouterPath     []byte
}

//Buffer 获取包数据
func (p *Package) Buffer() []byte {
	if p.Data == nil {
		p.Length = 0
	} else {
		p.Length = uint16(len(p.Data))
	}
	buffer := new(bytes.Buffer)
	lib.WriteByte(buffer, p.Header)
	if p.Length > 0 {
		lib.WriteByte(buffer, p.Data)
	}
	return buffer.Bytes()
}

//BuildRegisterSession 创建数据注册包
func BuildRegisterSession() *Package {
	buffer := new(bytes.Buffer)
	lib.WriteByte(buffer, uint16(0x01))
	lib.WriteByte(buffer, uint16(0x00))
	return NewPackage(CommandRegisterSession, buffer.Bytes())
}

//BuildUnregisterSession 创建数据注销
func BuildUnregisterSession() *Package {
	return NewPackage(CommandUnRegisterSession, nil)
}

//BuildRRData 不链接读取
func BuildRRData(data []byte, timeout uint16) *Package {
	buffer := new(bytes.Buffer)
	lib.WriteByte(buffer, uint32(0x00)) //I Interface Handle ID (Shall be 0 for CIP)
	lib.WriteByte(buffer, timeout)      //H EIPTimeout
	//内容部分
	cpf := BuildCPF([]*CPFItem{
		{TypeID: CPFTypeNull, Data: nil},
		{TypeID: CPFTypeUnconnectedMessage, Data: data},
	})
	buffer.Write(cpf)
	return NewPackage(CommandSendRRData, buffer.Bytes())
}

//BuildUnitData 链接读取
func BuildUnitData(data []byte, connectionID uint32, sequenceId uint32) *Package {
	buffer := new(bytes.Buffer)
	lib.WriteByte(buffer, uint32(0x00)) //I Interface Handle ID (Shall be 0 for CIP)
	lib.WriteByte(buffer, uint16(0x00)) //H EIPTimeout
	//内容部分
	connectBuf := new(bytes.Buffer)
	lib.WriteByte(connectBuf, connectionID)
	dataBuf := new(bytes.Buffer)
	lib.WriteByte(dataBuf, uint16(sequenceId))
	dataBuf.Write(data)
	//lib.WriteByte(seqAddrBuf, sequenceId)
	cpf := BuildCPF([]*CPFItem{
		{TypeID: CPFTypeConnectionBased, Data: connectBuf.Bytes()},
		{TypeID: CPFTypeConnectedTransportPacket, Data: dataBuf.Bytes()},
	})
	buffer.Write(cpf)
	pack := NewPackage(CommandSendUnitData, buffer.Bytes())
	pack.SequenceId = sequenceId
	pack.ConnectionID = connectionID
	return pack
}

//BuildUnconnectedSend 不连接封包
func BuildUnconnectedSend(path []byte, request []byte) *Package {
	ucmm := &UnconnectedSend{}
	//2000ms  2,250
	ucmm.TimeTick = 2
	ucmm.TimeOutTicks = 250
	ucmm.MessageRequest = request
	ucmm.RouterPath = path
	mr2 := &MessageRouterRequest{}
	mr2.Service = ServiceUnconnectedSendService
	mr2.RequestPath = segment.Paths(
		epath.LogicalBuild(epath.LogicalTypeClassID, 0x06, true),
		epath.LogicalBuild(epath.LogicalTypeInstanceID, 0x01, true),
	)
	mr2.RequestData = ucmm.Buffer()
	pack := BuildRRData(mr2.Buffer(), 10)
	return pack
}

func BuildReadAttributeAll(path []byte) *Package {
	mr2 := &MessageRouterRequest{}
	mr2.Service = ServiceGetAttributeAll
	mr2.RequestPath = segment.Paths(
		epath.LogicalBuild(epath.LogicalTypeClassID, 0x01, true),
		epath.LogicalBuild(epath.LogicalTypeInstanceID, 0x01, true),
	)
	mr2.RequestData = nil
	pack := BuildUnconnectedSend(path, mr2.Buffer())
	return pack
}

//BuildTagIOI 创建单个数据
func BuildTagIOI(tagName string, dataType types.DataType) []byte {
	buffer := new(bytes.Buffer)
	tagArray := strings.Split(tagName, ".")
	for i, tag := range tagArray {
		//存在后缀
		if strings.HasSuffix(tag, "]") {
			baseTag, indexs := lib.ParseTagName(tag)
			BaseTagLen := len([]byte(baseTag))
			if BaseTagLen > 255 {
				return nil
			}
			if dataType == types.BIT_STRING && i == len(tagArray)-1 {
				indexs = []int{indexs[0] / 32}
			} else if dataType == types.NULL {
				indexs = []int{0}
			}
			lib.WriteByte(buffer, uint8(0x91))
			lib.WriteByte(buffer, uint8(BaseTagLen))
			buffer.Write([]byte(baseTag))
			//字节补0
			if BaseTagLen%2 != 0 {
				lib.WriteByte(buffer, uint8(0x00))
			}
			for _, index := range indexs {
				if index < 256 {
					lib.WriteByte(buffer, uint8(0x28))
					lib.WriteByte(buffer, uint8(index))
				} else if index >= 256 && index < 65536 {
					lib.WriteByte(buffer, uint16(0x29))
					lib.WriteByte(buffer, uint16(index))
				} else {
					lib.WriteByte(buffer, uint16(0x2a))
					lib.WriteByte(buffer, uint32(index))
				}
			}
		} else if !lib.IsInteger(tag) {
			BaseTagLen := len([]byte(tag))
			lib.WriteByte(buffer, uint8(0x91))
			lib.WriteByte(buffer, uint8(BaseTagLen))
			buffer.Write([]byte(tag))
			if BaseTagLen%2 != 0 {
				lib.WriteByte(buffer, uint8(0x00))
			}
		}
	}
	return buffer.Bytes()
}

//AddReadIOI 获取字段内容数据包
func AddReadIOI(tagIOI []byte, elements uint16) []byte {
	buffer := new(bytes.Buffer)
	size := len(tagIOI) / 2
	lib.WriteByte(buffer, uint8(ServiceReadTag)) //ServiceReadTag
	lib.WriteByte(buffer, uint8(size))
	buffer.Write(tagIOI)
	lib.WriteByte(buffer, elements)
	return buffer.Bytes()
}

func (req *MessageRouterRequest) Buffer() []byte {
	buffer := new(bytes.Buffer)
	lib.WriteByte(buffer, req.Service)
	req.RequestPathSize = uint8(len(req.RequestPath) / 2)
	lib.WriteByte(buffer, req.RequestPathSize)
	lib.WriteByte(buffer, req.RequestPath)
	lib.WriteByte(buffer, req.RequestData)
	return buffer.Bytes()
}

func (u *UnconnectedSend) Buffer() []byte {
	buffer := new(bytes.Buffer)
	lib.WriteByte(buffer, u.TimeTick)
	lib.WriteByte(buffer, u.TimeOutTicks)
	msgLen := len(u.MessageRequest)
	lib.WriteByte(buffer, uint16(msgLen))
	lib.WriteByte(buffer, u.MessageRequest)
	if msgLen%2 == 1 {
		lib.WriteByte(buffer, uint8(0))
	}
	lib.WriteByte(buffer, uint8(len(u.RouterPath)/2))
	lib.WriteByte(buffer, uint8(0))
	lib.WriteByte(buffer, u.RouterPath)
	return buffer.Bytes()
}

//AddPartialReadIOI 获取字段信息
func AddPartialReadIOI(tagIOI []byte, elements uint16, offset uint32) []byte {
	buffer := new(bytes.Buffer)
	size := len(tagIOI) / 2
	lib.WriteByte(buffer, uint8(0x52))
	lib.WriteByte(buffer, uint8(size))
	buffer.Write(tagIOI)
	lib.WriteByte(buffer, elements)
	lib.WriteByte(buffer, offset)
	return buffer.Bytes()
}

func BuildMultiServiceHeader() []byte {
	buffer := new(bytes.Buffer)
	lib.WriteByte(buffer, uint8(ServiceMultipleServicePacket)) //B MultiService
	lib.WriteByte(buffer, uint8(0x02))                         //B MultiPathSize
	lib.WriteByte(buffer, uint8(0x20))                         //B MutliClassType
	lib.WriteByte(buffer, uint8(0x02))                         //B MultiClassSegment
	lib.WriteByte(buffer, uint8(0x24))                         //B MultiInstanceType
	lib.WriteByte(buffer, uint8(0x01))                         //B MultiInstanceSegment
	return buffer.Bytes()
}

func GenerateEncodedTimeout(timeout int) (uint8, uint8) {
	timeTick := uint8(0)
	ticks := uint8(0)
	first := true
	diff := 0
	for i := 0; i < 16; i++ {
		for j := 1; j < 256; j++ {
			newDiff := int(math.Abs(float64(timeout) - math.Pow(float64(2), float64(i))*float64(j)))
			if first || newDiff <= diff {
				first = false
				diff = newDiff
				timeTick = uint8(i)
				ticks = uint8(j)
				if newDiff == 0 {
					return timeTick, ticks
				}
			}
		}
	}
	return timeTick, ticks
}

//ParseReply 解析数据
func ParseReply(res *Response, tagName string, elements uint16) ([]interface{}, error) {
	_, indexs := lib.ParseTagName(tagName)
	dataType := res.DType
	if dataType == types.BIT_STRING {
		wordCount := lib.GetWordCount(uint16(indexs[0]), elements, 32)
		words, err := getReplyValues(res, wordCount)
		if err != nil {
			return nil, err
		}
		values := wordsToBits(words, elements, dataType, indexs[0])
		return values, nil
	} else if lib.IsBitWord(tagName) {
		bitCount := types.GetByteCount(dataType)
		wordCount := lib.GetWordCount(uint16(indexs[0]), elements, bitCount)
		words, err := getReplyValues(res, wordCount)
		if err != nil {
			return nil, err
		}
		values := wordsToBits(words, elements, dataType, indexs[0])
		return values, nil
	} else {
		values, err := getReplyValues(res, elements)
		if err != nil {
			return nil, err
		}
		return values, nil
	}
}

//getReplyValues 获取所有数值
func getReplyValues(res *Response, elements uint16) ([]interface{}, error) {
	dataType := res.DType
	reader := bytes.NewReader(res.Data)
	if reader.Len() == 0 {
		return nil, errors.New("返回内容为空，读取失败")
	}
	values := make([]interface{}, 0)
	offset := uint32(0)
	for i := 0; i < int(elements); i++ {
		if reader.Len() == 0 {
			return values, nil
		}
		result, pos, err2 := types.GetTypeValue(reader, dataType)
		if err2 != nil {
			return nil, err2
		}
		values = append(values, result)
		offset += pos
	}
	return values, nil
}

//wordsToBits 字节转位
func wordsToBits(words []interface{}, elements uint16, dataType types.DataType, index int) []interface{} {
	bitPos := 0
	if dataType == types.BIT_STRING {
		bitPos = index % 32
	} else {
		bitPos = index
	}
	ret := make([]interface{}, 0)
	for _, word := range words {
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
	}
	ret = ret[bitPos : bitPos+int(elements)]
	return ret
}
