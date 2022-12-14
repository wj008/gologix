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
	"strconv"
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

//readBytes ?????????????????????
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

//readPackage ???????????????
func (p *PLC) readPackage() (*enip.Package, error) {
	if !p.IsConnected {
		return nil, errors.New("???????????????????????????????????????")
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

//recvData ????????????????????????
func (p *PLC) recvData(reply *enip.Package) {
	p.PrintPackage("------------readPackage-------------", reply)
	if reply.Command == enip.CommandSendRRData || reply.Command == enip.CommandSendUnitData {
		reply.DataItems = enip.ParserCPF(reply.Data[6:])
	}
	if reply.Command == enip.CommandSendUnitData {
		if len(reply.DataItems) < 2 {
			p.Println("????????????????????????")
			return
		}
		addrItem := reply.DataItems[0]
		dataItem := reply.DataItems[1]
		reply.SequenceId = 0
		//?????????????????????
		if addrItem.TypeID == enip.CPFTypeConnectionBased && addrItem.Length >= 4 {
			reply.ConnectionID = binary.LittleEndian.Uint32(addrItem.Data[0:4])
		}
		if dataItem.TypeID == enip.CPFTypeConnectedTransportPacket && dataItem.Length >= 2 {
			reply.SequenceId = uint32(binary.LittleEndian.Uint16(dataItem.Data[0:2]))
		}
		//?????????????????????
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

	//?????????
	contextId := reply.ContextId
	callback, ok := p.contextPool[contextId]
	if ok && callback != nil {
		callback(reply)
		delete(p.contextPool, contextId)
	}
	return
}

//Accept ??????????????????
func (p *PLC) accept() {
	go func() {
		for {
			pack, err := p.readPackage()
			if err != nil {
				p.Println("????????????????????????", err)
				p.Close()
				return
			}
			if pack.Status == enip.StatusSuccess {
				p.recvData(pack)
			} else {
				p.Println("?????????????????????", enip.ParseStatus(pack.Status))
			}
		}
	}()
}

//Connect ????????????
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

//Close ????????????
func (p *PLC) Close() error {
	p.IsForwardOpened = false
	p.IsRegistered = false
	p.IsConnected = false
	if p.OnClose != nil {
		p.OnClose()
	}
	return p.Conn.Close()
}

//writePack ???????????????
func (p *PLC) writePack(pack *enip.Package) (reply *enip.Package, err error) {
	if !p.IsConnected {
		return nil, errors.New("--??????????????????--")
	}
	if pack.Command == enip.CommandSendUnitData && !p.IsRegistered {
		return nil, errors.New("?????????????????????")
	}
	timeout := enip.NewTimeOut(10 * time.Second)
	callback := func(reply *enip.Package) {
		timeout.Write(reply)
	}
	contextId := p.newContextId()
	sequenceId := uint32(0)
	pack.ContextId = contextId
	pack.SessionId = p.SessionId
	//???????????????
	if pack.Command == enip.CommandSendUnitData {
		sequenceId = pack.SequenceId
		p.sequencePool[sequenceId] = callback
	} else {
		p.contextPool[contextId] = callback
	}
	//??????????????????
	errorCall := func() {
		//????????????
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

	buffer := pack.Buffer()
	p.PrintPackage("--------writePack------------", pack)
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

//RegisterSession ????????????
func (p *PLC) RegisterSession() error {
	if p.IsRegistered {
		return errors.New("???????????????????????????????????????")
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
	p.Println("????????????????????????")
	return nil
}

//UnregisterSession ????????????
func (p *PLC) UnregisterSession() error {
	if !p.IsRegistered {
		return errors.New("??????????????????")
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

//ForwardOpen ???????????????????????????
func (p *PLC) ForwardOpen() error {
	p.Println("ForwardOpen")
	//??????????????????????????????????????????????????????
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
		return errors.New("??????????????????")
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
		return errors.New("??????????????????")
	}
	if p.ConnectionSize == 0 {
		p.ConnectionSize = connectionSize
	}
	conId := binary.LittleEndian.Uint32(reply.Data[20:24])
	p.IsForwardOpened = true
	p.connectionID = conId
	return nil
}

//ForwardClose ????????????????????????
func (p *PLC) ForwardClose() error {
	p.Println("ForwardClose")
	frameData := p.buildForwardClose()
	pack := enip.BuildRRData(frameData, 5)
	reply, err := p.writePack(pack)
	if err != nil {
		return err
	}
	if len(reply.DataItems) < 2 {
		return errors.New("??????????????????")
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
	//???????????????
	if connectionSize <= 511 {
		CIPService = uint8(0x54)
		parametersUint16 = uint16(0x4200) + connectionSize
	} else {
		//???????????????
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
	//???????????????
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

//ReadPartialTag ????????????????????????
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
		return 0, errors.New("???????????????")
	}
	p.knownTags[tagName] = res.DType
	//?????????????????????
	return res.DType, nil
}

//ReadTag ??????????????????
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
		//????????????
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
		p.Println("res.Status", res.Status)
		return nil, errors.New("???????????????")
	}
	values, err := p.ParseReply(res, tagName, elements)
	if err != nil {
		return nil, err
	}
	result := &TagResult{}
	result.Status = res.Status
	result.DType = res.DType
	result.Values = values
	return result, nil
}

func (p *PLC) MultiReadTag(tagList []string) (map[string]*TagValue, error) {
	p.Println("MultiReadTag", tagList)
	listLen := len(tagList)
	if listLen == 0 {
		return nil, errors.New("?????????????????????")
	}
	if listLen == 1 {
		tagName := tagList[0]
		result, err2 := p.ReadTag(tagName, 1)
		if err2 != nil {
			return nil, err2
		}
		values := make(map[string]*TagValue)
		tagValue := &TagValue{}
		tagValue.Value = result.Values[0]
		tagValue.Status = result.Status
		tagValue.DType = result.DType
		values[tagName] = tagValue
		return values, nil
	}

	serviceSegments := make([][]byte, 0)
	header := enip.BuildMultiServiceHeader()
	tagCount := 0
	hLen := len(header)
	dataLen := hLen + 2
	for _, tagName := range tagList {
		baseTag, _ := lib.ParseTagName(tagName)
		dataType, err := p.ReadPartialTag(baseTag)
		if err != nil {
			return nil, err
		}
		tagData := enip.BuildTagIOI(tagName, dataType)
		readRequest := enip.AddReadIOI(tagData, 1)
		dataLen += len(readRequest) + 2
		if dataLen > int(p.ConnectionSize) {
			break
		}
		serviceSegments = append(serviceSegments, readRequest)
		tagCount++
	}
	if tagCount == 0 {
		return nil, errors.New("?????????????????????")
	}
	buffer := new(bytes.Buffer)
	buffer.Write(header)
	lib.WriteByte(buffer, uint16(tagCount))
	data := new(bytes.Buffer)
	offsets := new(bytes.Buffer)
	temp := hLen
	if tagCount > 2 {
		temp += (tagCount - 2) * 2
	}
	for _, segment := range serviceSegments {
		lib.WriteByte(offsets, uint16(temp))
		data.Write(segment)
		temp += len(segment)
	}
	buffer.Write(offsets.Bytes())
	buffer.Write(data.Bytes())
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
	values, err := p.multiParser(res, tagList)
	return values, err
}

//ReadAttributeAll ??????????????????
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

//MultiParser ??????????????????
func (p *PLC) multiParser(res *enip.Response, tagList []string) (map[string]*TagValue, error) {
	values := make(map[string]*TagValue)
	dataLen := len(res.Data)
	if dataLen == 0 {
		return nil, errors.New("?????????????????????????????????")
	}
	tagList2 := make([]string, 0)
	tagCount := (binary.LittleEndian.Uint16(res.Data[0:2]) - 2) / 2
	for i, tag := range tagList {
		tagValue := &TagValue{}
		loc := i * 2
		if loc+2 > dataLen || i >= int(tagCount) {
			tagList2 = append(tagList2, tag)
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
				return nil, errors.New("?????????????????????????????????")
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
				return nil, errors.New("?????????????????????????????????")
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
	if len(tagList2) > 0 {
		values2, err4 := p.MultiReadTag(tagList2)
		if err4 != nil {
			return nil, err4
		}
		for name, value := range values2 {
			values[name] = value
		}
	}
	return values, nil
}

//ParseReply ????????????
func (p *PLC) ParseReply(res *enip.Response, tagName string, elements uint16) ([]interface{}, error) {
	_, indexs := lib.ParseTagName(tagName)
	dataType := res.DType
	if dataType == types.BIT_STRING {
		wordCount := lib.GetWordCount(uint16(indexs[0]), elements, 32)
		words, err := p.getReplyValues(res, tagName, wordCount)
		if err != nil {
			return nil, err
		}
		values := wordsToBits(words, elements, dataType, indexs[0])
		return values, nil
	} else if lib.IsBitWord(tagName) {
		bitCount := types.GetByteCount(dataType) * 8
		index := uint16(indexs[0]) % bitCount
		wordCount := lib.GetWordCount(index, elements, bitCount)
		words, err := p.getReplyValues(res, tagName, wordCount)
		if err != nil {
			return nil, err
		}
		values := wordsToBits(words, elements, dataType, int(index))
		return values, nil
	} else {
		values, err := p.getReplyValues(res, tagName, elements)
		if err != nil {
			return nil, err
		}
		return values, nil
	}
}

//getReplyValues ??????????????????
func (p *PLC) getReplyValues(res *enip.Response, tagName string, elements uint16) ([]interface{}, error) {
	dataType := res.DType
	reader := bytes.NewReader(res.Data)
	if reader.Len() == 0 {
		return nil, errors.New("?????????????????????????????????")
	}
	values := make([]interface{}, 0)
	offset := uint32(0)
	count := int(elements)
	for i := 0; i < count; i++ {
		if reader.Len() == 0 {
			elements2 := count - i
			if elements2 > 0 {
				baseTag, indexs := lib.ParseTagName(tagName)
				if dataType == types.BIT_STRING {
					start2 := indexs[0] + i*32
					tagName2 := baseTag + "[" + strconv.Itoa(start2) + "]"
					result, err3 := p.ReadTag(tagName2, uint16(elements2))
					if err3 != nil {
						return values, err3
					}
					values = append(values, result.Values...)
				} else if lib.IsBitWord(tagName) {
					bitCount := types.GetByteCount(dataType) * 8
					start2 := indexs[0] + i*int(bitCount)
					tagName2 := baseTag + "." + strconv.Itoa(start2)
					result, err3 := p.ReadTag(tagName2, uint16(elements2))
					if err3 != nil {
						return values, err3
					}
					values = append(values, result.Values...)
				} else {
					start2 := indexs[0] + i
					tagName2 := baseTag + "[" + strconv.Itoa(start2) + "]"
					result, err3 := p.ReadTag(tagName2, uint16(elements2))
					if err3 != nil {
						return values, err3
					}
					values = append(values, result.Values...)
				}
			}
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

//wordsToBits ????????????
func wordsToBits(words []interface{}, elements uint16, dataType types.DataType, index int) []interface{} {
	bitPos := 0
	if dataType == types.BIT_STRING {
		bitPos = index % 32
	} else {
		bitCount := types.GetByteCount(dataType) * 8
		bitPos = index % int(bitCount)
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
