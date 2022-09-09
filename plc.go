package gologix

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/wj008/gologix/enip"
	"github.com/wj008/gologix/epath"
	"github.com/wj008/gologix/lib"
	"github.com/wj008/gologix/types"
	"log"
	"math/rand"
	"net"
	"strings"
	"time"
)

type TagResult struct {
	Status uint8
	DType  types.DataType
	Values []interface{}
}

type TagValue struct {
	Status uint8
	DType  types.DataType
	Value  interface{}
}

type PLCInfo struct {
	SerialNumber            uint32
	Name                    string
	Version                 string
	Status                  uint16
	Faulted                 bool
	MinorRecoverableFault   bool
	MinorUnrecoverableFault bool
	MajorRecoverableFault   bool
	MajorUnrecoverableFault bool
	IoFaulted               bool
}

type PLC struct {
	net.Conn
	IsConnected            bool
	IsRegistered           bool
	IsForwardOpened        bool
	Micro800               bool
	SessionId              uint32
	SequenceCounter        uint32
	OnClose                func()
	sequencePool           map[uint32]func(*enip.Package)
	contextPool            map[uint64]func(*enip.Package)
	knownTags              map[string]types.DataType
	connectionID           uint32
	ConnectionSize         uint16
	targetPath             []byte
	connectionPath         []byte
	serialId               uint16
	vendorId               uint16
	originatorSerialNumber uint32
	Info                   *PLCInfo
	Logger                 *log.Logger
}

func NewPLC() *PLC {
	return &PLC{}
}

func (p *PLC) Println(v ...interface{}) {
	if p.Logger != nil {
		p.Logger.Println(v...)
	}
}

func (p *PLC) PrintPackage(tag string, pack *enip.Package) {
	if p.Logger != nil {
		a := fmt.Sprintf("%x", pack.Data)
		temp := make([]string, 0)
		for i := 0; i < len(a)/2; i++ {
			temp = append(temp, a[i*2:i*2+2])
		}
		out := strings.Join(temp, " ")
		p.Logger.Println(tag)
		p.Logger.Println("Command", pack.Command, "Length", pack.Length, "SessionId", p.SessionId, "ContextId", pack.ContextId, "Status", pack.Status, "Options", pack.Options, "SequenceId", pack.SequenceId, "connectionID", pack.ConnectionID)
		p.Logger.Println("Data", out)
	}
}

func (p *PLC) PrintBytes(tag string, buf []byte) {
	if p.Logger != nil {
		a := fmt.Sprintf("%x", buf)
		temp := make([]string, 0)
		for i := 0; i < len(a)/2; i++ {
			temp = append(temp, a[i*2:i*2+2])
		}
		out := strings.Join(temp, " ")
		p.Logger.Println(tag, out)
	}
}

//readBytes 读取套接字字节
func (p *PLC) readBytes(length int) ([]byte, error) {
	end := length
	buffer := make([]byte, length)
	temp := buffer[0:end]
	reTry := 0
	nLen := 0
	for {
		reTry++
		if reTry > 100 {
			err := errors.New(fmt.Sprintf("Expected to read %d bytes, but only read %d", length, nLen))
			return nil, err
		}
		n, err1 := p.Read(temp)
		if err1 != nil {
			return nil, err1
		}
		nLen += n
		if n < end {
			temp = buffer[n:end]
			end = end - n
			continue
		} else {
			break
		}
	}
	return buffer, nil
}

//readPackage 读取数据包
func (p *PLC) readPackage() (*enip.Package, error) {
	if !p.IsConnected {
		return nil, errors.New("链接已经关闭，不可读取数据")
	}
	header, err := p.readBytes(24)
	if err != nil {
		return nil, err
	}
	dataReader := bytes.NewReader(header)
	reply := &enip.Package{}
	lib.ReadByte(dataReader, &reply.Header)
	length := int(reply.Length)
	if length > 0 {
		body, err2 := p.readBytes(int(reply.Length))
		if err2 != nil {
			return nil, err2
		}
		reply.Data = body
	}
	return reply, nil
}

func (p *PLC) newContextId() uint64 {
	rand.Seed(time.Now().UnixNano())
	contextId := rand.Uint64()
	return contextId
}

func (p *PLC) newSequenceId() uint32 {
	p.SequenceCounter += 1
	p.SequenceCounter = p.SequenceCounter % 0x10000
	if p.SequenceCounter == 0 {
		p.SequenceCounter = 1
	}
	return p.SequenceCounter
}

//recvData 接收到新的数据包
func (p *PLC) recvData(reply *enip.Package) {
	p.PrintPackage("------------readPackage-------------", reply)
	if reply.Command == enip.CommandSendRRData || reply.Command == enip.CommandSendUnitData {
		reply.DataItems = enip.ParserCPF(reply.Data[6:])
	}
	if reply.Command == enip.CommandSendUnitData {
		if len(reply.DataItems) < 2 {
			p.Println("读取数据节点错误")
			return
		}
		addrItem := reply.DataItems[0]
		dataItem := reply.DataItems[1]
		reply.SequenceId = 0
		//取队列地址数据
		if addrItem.TypeID == enip.CPFTypeConnectionBased && addrItem.Length >= 4 {
			reply.ConnectionID = binary.LittleEndian.Uint32(addrItem.Data[0:4])
		}
		if dataItem.TypeID == enip.CPFTypeConnectedTransportPacket && dataItem.Length >= 2 {
			reply.SequenceId = uint32(binary.LittleEndian.Uint16(dataItem.Data[0:2]))
		}
		//获取到队列地址
		if reply.SequenceId != 0 {
			sequenceId := reply.SequenceId
			if callback, ok := p.sequencePool[sequenceId]; ok && callback != nil {
				callback(reply)
				delete(p.sequencePool, sequenceId)
				return
			}
		}
		return
	}

	//其余的
	contextId := reply.ContextId
	callback, ok := p.contextPool[contextId]
	if ok && callback != nil {
		callback(reply)
		delete(p.contextPool, contextId)
	}
	return
}

//Accept 开始接收数据
func (p *PLC) accept() {
	go func() {
		for {
			pack, err := p.readPackage()
			if err != nil {
				p.Println("读取包数据错误：", err)
				p.Close()
				return
			}
			if pack.Status == enip.StatusSuccess {
				p.recvData(pack)
			} else {
				p.Println("数据包返回错误", enip.ParseStatus(pack.Status))
			}
		}
	}()
}

//Connect 发起链接
func (p *PLC) Connect(addr string, slot uint8) (err error) {
	rawConn, err := net.Dial("tcp", addr)
	if err != nil {
		return
	}
	p.Conn = rawConn
	p.IsConnected = true
	p.contextPool = make(map[uint64]func(*enip.Package))
	p.sequencePool = make(map[uint32]func(*enip.Package))
	p.knownTags = make(map[string]types.DataType)

	if p.Micro800 {
		p.connectionPath = []byte{0x20, 0x02, 0x24, 0x01}
		p.vendorId = 0x01
	} else {
		p.connectionPath = []byte{0x01, slot, 0x20, 0x02, 0x24, 0x01}
		p.vendorId = 0x1337
	}
	p.targetPath = epath.PortBuild([]byte{slot}, 1, true)
	p.originatorSerialNumber = 42
	p.SequenceCounter = 0
	p.Info = &PLCInfo{}
	p.accept()
	return
}

//Close 关闭链接
func (p *PLC) Close() error {
	p.IsForwardOpened = false
	p.IsRegistered = false
	p.IsConnected = false
	if p.OnClose != nil {
		p.OnClose()
	}
	return p.Conn.Close()
}

//writePack 写入数据包
func (p *PLC) writePack(pack *enip.Package) (reply *enip.Package, err error) {
	if !p.IsConnected {
		return nil, errors.New("--链接已经关闭--")
	}
	if pack.Command == enip.CommandSendUnitData && !p.IsRegistered {
		return nil, errors.New("还没有注册链接")
	}
	timeout := enip.NewTimeOut(30 * time.Second)
	callback := func(reply *enip.Package) {
		timeout.Write(reply)
	}
	contextId := p.newContextId()
	sequenceId := uint32(0)
	pack.ContextId = contextId
	pack.SessionId = p.SessionId
	//数据包写入
	if pack.Command == enip.CommandSendUnitData {
		sequenceId = pack.SequenceId
		p.sequencePool[sequenceId] = callback
	} else {
		p.contextPool[contextId] = callback
	}
	//如果发生错误
	errorCall := func() {
		//写入错误
		if pack.Command == enip.CommandSendUnitData {
			if _, ok := p.sequencePool[sequenceId]; ok {
				delete(p.sequencePool, sequenceId)
			}
		} else {
			if _, ok := p.contextPool[contextId]; ok {
				delete(p.contextPool, contextId)
			}
		}
		timeout.Close()
		p.Close()
	}
	p.PrintPackage("--------writePack------------", pack)
	buffer := pack.Buffer()
	if _, err = p.Write(buffer); err != nil {
		errorCall()
		return
	}
	reply, err = timeout.Read()
	if err != nil {
		errorCall()
		return
	}
	return
}

//RegisterSession 注册链接
func (p *PLC) RegisterSession() error {
	if p.IsRegistered {
		return errors.New("链接已经注册，不可重复注册")
	}
	p.Println("RegisterSession")
	pack := enip.BuildRegisterSession()
	reply, err := p.writePack(pack)
	if err != nil {
		return err
	}
	p.SessionId = reply.SessionId
	p.SequenceCounter = 0
	p.IsRegistered = true
	err3 := p.ReadAttributeAll()
	if err3 != nil {
		p.Close()
		return err3
	}
	p.Println("链接已经注册成功")
	return nil
}

//UnregisterSession 退出链接
func (p *PLC) UnregisterSession() error {
	if !p.IsRegistered {
		return errors.New("链接尚未注册")
	}
	p.Println("UnregisterSession")
	pack := enip.BuildUnregisterSession()
	pack.SessionId = p.SessionId
	buffer := pack.Buffer()
	if _, err := p.Write(buffer); err != nil {
		p.Close()
		return err
	}
	p.SessionId = 0
	p.IsRegistered = false
	ch := enip.NewTimeOut(1 * time.Second)
	ch.Read()
	return nil
}

//ForwardOpen 打开小数据读取通道
func (p *PLC) ForwardOpen() error {
	p.Println("ForwardOpen")
	//如果没有设置链接大小，尝试打开大链接
	connectionSize := p.ConnectionSize
	testLarge := false
	if p.ConnectionSize == 0 {
		testLarge = true
		connectionSize = 4002
	}
sendData:
	frameData := p.buildForwardOpen(connectionSize)
	pack := enip.BuildRRData(frameData, 5)
	reply, err := p.writePack(pack)
	if err != nil {
		return err
	}
	if len(reply.DataItems) < 2 {
		return errors.New("数据状态不符")
	}
	dataItem := reply.DataItems[1]
	status := dataItem.Data[2]
	if status != 0 {
		if testLarge {
			testLarge = false
			connectionSize = 508
			goto sendData
		}
		p.Close()
		return errors.New("数据状态不符")
	}
	if p.ConnectionSize == 0 {
		p.ConnectionSize = connectionSize
	}
	conId := binary.LittleEndian.Uint32(reply.Data[20:24])
	p.IsForwardOpened = true
	p.connectionID = conId
	return nil
}

//ForwardClose 关闭数据读取通道
func (p *PLC) ForwardClose() error {
	p.Println("ForwardClose")
	frameData := p.buildForwardClose()
	pack := enip.BuildRRData(frameData, 5)
	reply, err := p.writePack(pack)
	if err != nil {
		return err
	}
	if len(reply.DataItems) < 2 {
		return errors.New("数据状态不符")
	}
	dataItem := reply.DataItems[1]
	if dataItem.TypeID == enip.CPFTypeUnconnectedMessage {
		p.IsForwardOpened = false
	}
	ch := enip.NewTimeOut(1 * time.Second)
	ch.Read()
	return nil
}

func (p *PLC) buildForwardOpen(connectionSize uint16) []byte {
	buffer := new(bytes.Buffer)
	p.serialId = uint16(lib.RandInt64(65000))
	var CIPService uint8
	var parametersUint16 uint16
	var parametersUint32 uint32
	//读取小数据
	if connectionSize <= 511 {
		CIPService = uint8(0x54)
		parametersUint16 = uint16(0x4200) + connectionSize
	} else {
		//读取大数据
		CIPService = uint8(0x5b)
		parametersUint32 = uint32(0x42000000) + uint32(connectionSize)
	}
	lib.WriteByte(buffer, CIPService)               //B
	lib.WriteByte(buffer, uint8(0x02))              //B CIPPathSize
	lib.WriteByte(buffer, uint8(0x20))              //B CIPClassType
	lib.WriteByte(buffer, uint8(0x06))              //B CIPClass
	lib.WriteByte(buffer, uint8(0x24))              //B CIPInstanceType
	lib.WriteByte(buffer, uint8(0x01))              //B CIPInstance
	lib.WriteByte(buffer, uint8(0x0a))              //B CIPPriority
	lib.WriteByte(buffer, uint8(0x0e))              //B CIPTimeoutTicks
	lib.WriteByte(buffer, uint32(0x20000002))       //I CIPOTConnectionID
	lib.WriteByte(buffer, uint32(0x20000001))       //I CIPTOConnectionID
	lib.WriteByte(buffer, p.serialId)               //H
	lib.WriteByte(buffer, p.vendorId)               //H
	lib.WriteByte(buffer, p.originatorSerialNumber) //I
	lib.WriteByte(buffer, uint32(0x03))             //I CIPMultiplier
	lib.WriteByte(buffer, uint32(0x00201234))       //I CIPOTRPI
	//小数据读取
	if CIPService == 0x54 {
		lib.WriteByte(buffer, parametersUint16)
		lib.WriteByte(buffer, uint32(0x00204001))
		lib.WriteByte(buffer, parametersUint16)
	} else {
		lib.WriteByte(buffer, parametersUint32)
		lib.WriteByte(buffer, uint32(0x00204001))
		lib.WriteByte(buffer, parametersUint32)
	}

	lib.WriteByte(buffer, uint8(0xa3))
	lib.WriteByte(buffer, uint8(len(p.connectionPath)/2))
	buffer.Write(p.connectionPath)
	return buffer.Bytes()
}

func (p *PLC) buildForwardClose() []byte {
	buffer := new(bytes.Buffer)
	var CIPService uint8 = 0x4e
	lib.WriteByte(buffer, CIPService)               //B
	lib.WriteByte(buffer, uint8(0x02))              //B CIPPathSize
	lib.WriteByte(buffer, uint8(0x20))              //B CIPClassType
	lib.WriteByte(buffer, uint8(0x06))              //B CIPClass
	lib.WriteByte(buffer, uint8(0x24))              //B CIPInstanceType
	lib.WriteByte(buffer, uint8(0x01))              //B CIPInstance
	lib.WriteByte(buffer, uint8(0x0a))              //B CIPPriority
	lib.WriteByte(buffer, uint8(0x0e))              //B CIPTimeoutTicks
	lib.WriteByte(buffer, p.serialId)               //H
	lib.WriteByte(buffer, p.vendorId)               //H
	lib.WriteByte(buffer, p.originatorSerialNumber) //I
	lib.WriteByte(buffer, uint8(len(p.connectionPath)/2))
	buffer.Write(p.connectionPath)
	return buffer.Bytes()
}

//ReadPartialTag 读取节点数据类型
func (p *PLC) ReadPartialTag(tagName string) (types.DataType, error) {
	if tagType, ok := p.knownTags[tagName]; ok {
		return tagType, nil
	}
	p.Println("ReadPartialTag", tagName)
	tagData := enip.BuildTagIOI(tagName, 0)
	readRequest := enip.AddPartialReadIOI(tagData, 1, 0)
	var pack *enip.Package
	IsForwardOpened := p.IsForwardOpened
	if IsForwardOpened {
		pack = enip.BuildUnitData(readRequest, p.connectionID, p.newSequenceId())
	} else {
		pack = enip.BuildUnconnectedSend(p.targetPath, readRequest)
	}
	reply, err := p.writePack(pack)
	if err != nil {
		return 0, err
	}
	dataItem := reply.DataItems[1]
	res := enip.ParserResponse(dataItem.Data, IsForwardOpened)
	if res.Status != 0 && res.Status != 6 {
		return 0, errors.New("状态不正确")
	}
	p.knownTags[tagName] = res.DType
	//创建上下文关联
	return res.DType, nil
}

//ReadTag 读取节点数据
func (p *PLC) ReadTag(tagName string, elements uint16) (*TagResult, error) {
	baseTag, indexs := lib.ParseTagName(tagName)
	dataType, err := p.ReadPartialTag(baseTag)
	if err != nil {
		return nil, err
	}
	p.Println("ReadTag", tagName)
	var pack *enip.Package
	var readRequest []byte
	IsForwardOpened := p.IsForwardOpened
	if dataType == types.BIT_STRING {
		//211
		tagData := enip.BuildTagIOI(tagName, dataType)
		words := lib.GetWordCount(uint16(indexs[0]), elements, 32)
		readRequest = enip.AddReadIOI(tagData, words)
	} else if lib.IsBitWord(tagName) {
		bitCount := types.GetByteCount(dataType) * 8
		tagData := enip.BuildTagIOI(tagName, dataType)
		words := lib.GetWordCount(uint16(indexs[0]), elements, bitCount)
		readRequest = enip.AddReadIOI(tagData, words)
	} else {
		//读取数据
		tagData := enip.BuildTagIOI(tagName, dataType)
		readRequest = enip.AddReadIOI(tagData, elements)
	}
	if IsForwardOpened {
		pack = enip.BuildUnitData(readRequest, p.connectionID, p.newSequenceId())
	} else {
		pack = enip.BuildUnconnectedSend(p.targetPath, readRequest)
	}
	reply, err := p.writePack(pack)
	if err != nil {
		return nil, err
	}
	dataItem := reply.DataItems[1]
	res := enip.ParserResponse(dataItem.Data, IsForwardOpened)
	if res.Status != 0 && res.Status != 6 {
		return nil, errors.New("状态不正确")
	}
	values, err := enip.ParseReply(res, tagName, elements)
	result := &TagResult{}
	result.Status = res.Status
	result.DType = res.DType
	result.Values = values
	return result, nil
}

func (p *PLC) MultiReadTag(tagList []string) (map[string]*TagValue, error) {
	p.Println("MultiReadTag", tagList)
	serviceSegments := make([][]byte, 0)
	tagCount := len(tagList)
	header := enip.BuildMultiServiceHeader()
	for _, tagName := range tagList {
		baseTag, _ := lib.ParseTagName(tagName)
		dataType, err := p.ReadPartialTag(baseTag)
		if err != nil {
			return nil, err
		}
		tagData := enip.BuildTagIOI(tagName, dataType)
		readRequest := enip.AddReadIOI(tagData, 1)
		serviceSegments = append(serviceSegments, readRequest)
	}
	offsets := new(bytes.Buffer)
	temp := len(header)
	if tagCount > 2 {
		temp += (tagCount - 2) * 2
	}
	lib.WriteByte(offsets, uint16(temp))
	for i := 0; i < tagCount-1; i++ {
		temp += len(serviceSegments[i])
		lib.WriteByte(offsets, uint16(temp))
	}
	buffer := new(bytes.Buffer)
	buffer.Write(header)
	lib.WriteByte(buffer, uint16(tagCount))
	buffer.Write(offsets.Bytes())
	for _, segment := range serviceSegments {
		buffer.Write(segment)
	}
	var pack *enip.Package
	IsForwardOpened := p.IsForwardOpened
	if IsForwardOpened {
		pack = enip.BuildUnitData(buffer.Bytes(), p.connectionID, p.newSequenceId())
	} else {
		pack = enip.BuildUnconnectedSend(p.targetPath, buffer.Bytes())
	}
	reply, err := p.writePack(pack)
	if err != nil {
		return nil, err
	}
	dataItem := reply.DataItems[1]
	res := enip.ParserResponse(dataItem.Data, IsForwardOpened)
	values, err := multiParser(res, tagList)
	return values, err
}

//ReadAttributeAll 获取设备信息
func (p *PLC) ReadAttributeAll() error {
	pack := enip.BuildReadAttributeAll(p.targetPath)
	reply, err := p.writePack(pack)
	if err != nil {
		return err
	}
	dataItem := reply.DataItems[1]
	p.Info.SerialNumber = binary.LittleEndian.Uint32(dataItem.Data[14:18])
	nLen := dataItem.Data[18]
	p.Info.Name = string(dataItem.Data[19 : 19+int(nLen)])
	major := dataItem.Data[10]
	minor := dataItem.Data[11]
	p.Info.Version = fmt.Sprintf("%d.%d", major, minor)
	status := binary.LittleEndian.Uint16(dataItem.Data[12:14])
	p.Info.Status = status
	status &= 0x0ff0
	p.Info.Faulted = (status & 0x0f00) > 0
	p.Info.MinorRecoverableFault = (status & 0x0100) > 0
	p.Info.MinorUnrecoverableFault = (status & 0x0200) > 0
	p.Info.MajorRecoverableFault = (status & 0x0400) > 0
	p.Info.MajorUnrecoverableFault = (status & 0x0800) > 0
	status &= 0x0f00
	p.Info.IoFaulted = status>>4 == 2
	if status>>4 == 2 {
		p.Info.Faulted = true
	}
	p.Println(p.Info)
	return nil
}

//MultiParser 解析批量数据
func multiParser(res *enip.Response, tagList []string) (map[string]*TagValue, error) {
	values := make(map[string]*TagValue)
	dataLen := len(res.Data)
	if dataLen == 0 {
		return nil, errors.New("返回内容为空，读取失败")
	}
	for i, tag := range tagList {
		tagValue := &TagValue{}
		loc := i * 2
		if loc+2 > dataLen {
			tagValue.Value = nil
			values[tag] = tagValue
			continue
		}
		offset := binary.LittleEndian.Uint16(res.Data[loc : loc+2])
		if int(offset+4) > dataLen {
			tagValue.Value = nil
			values[tag] = tagValue
			continue
		}
		replyStatus := res.Data[offset]
		replyExtended := res.Data[offset+1]
		if !(replyStatus == 0 && replyExtended == 0) {
			tagValue.Status = replyStatus
			if replyStatus == 0 && replyExtended != 0 {
				tagValue.Status = 100
			}
			tagValue.Value = nil
			values[tag] = tagValue
			continue
		}
		dataType := types.DataType(binary.LittleEndian.Uint16(res.Data[offset+2 : offset+4]))
		if lib.IsBitWord(tag) {
			reader := bytes.NewReader(res.Data[offset+4:])
			if reader.Len() == 0 {
				return nil, errors.New("返回内容为空，读取失败")
			}
			result, _, err2 := types.GetTypeValue(reader, dataType)
			if err2 != nil {
				tagValue.Value = nil
				tagValue.Status = 101
				values[tag] = tagValue
				continue
			}
			tagValue.Value, err2 = types.GetBitOfWord(tag, result)
			if err2 != nil {
				tagValue.Value = nil
				tagValue.Status = 102
				values[tag] = tagValue
				continue
			}
			tagValue.Status = 0
			tagValue.DType = dataType
			values[tag] = tagValue

		} else if dataType == types.BIT_STRING {
			reader := bytes.NewReader(res.Data[offset+4:])
			if reader.Len() == 0 {
				return nil, errors.New("返回内容为空，读取失败")
			}
			result, _, err2 := types.GetTypeValue(reader, dataType)
			if err2 != nil {
				tagValue.Value = nil
				tagValue.Status = 101
				values[tag] = tagValue
				continue
			}
			tagValue.Value, err2 = types.GetBitOfWord(tag, result)
			if err2 != nil {
				tagValue.Value = nil
				tagValue.Status = 102
				values[tag] = tagValue
				continue
			}
			tagValue.Status = 0
			tagValue.DType = dataType
			values[tag] = tagValue
		} else {
			reader := bytes.NewReader(res.Data[offset+4:])
			var err2 error
			tagValue.Value, _, err2 = types.GetTypeValue(reader, dataType)
			if err2 != nil {
				tagValue.Value = nil
				tagValue.Status = 101
				values[tag] = tagValue
				continue
			}
			tagValue.Status = 0
			tagValue.DType = dataType
			values[tag] = tagValue
		}
	}
	return values, nil
}
