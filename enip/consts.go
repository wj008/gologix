package enip

type Command uint16
type CPFType uint16
type CIPServType uint8

const (
	ServiceGetAttributeAll        CIPServType = 0x01
	ServiceGetAttributeSingle     CIPServType = 0x0e
	ServiceReset                  CIPServType = 0x05
	ServiceStart                  CIPServType = 0x06
	ServiceStop                   CIPServType = 0x07
	ServiceCreate                 CIPServType = 0x08
	ServiceDelete                 CIPServType = 0x09
	ServiceMultipleServicePacket  CIPServType = 0x0a
	ServiceApplyAttributes        CIPServType = 0x0d
	ServiceSetAttributeSingle     CIPServType = 0x10
	ServiceFindNext               CIPServType = 0x11
	ServiceReadTag                CIPServType = 0x4c
	ServiceWriteTag               CIPServType = 0x4d
	ServiceReadTagFragmented      CIPServType = 0x52
	ServiceWriteTagFragmented     CIPServType = 0x53
	ServiceForwardOpen            CIPServType = 0x54
	ServiceForwardOpenLarge       CIPServType = 0x5b
	ServiceForwardClose           CIPServType = 0x4e
	ServiceReadModifyWriteTag     CIPServType = 0x4e
	ServiceUnconnectedSendService CIPServType = 0x52
)
const (
	CommandNOP               Command = 0x0000
	CommandListServices      Command = 0x0004
	CommandListIdentity      Command = 0x0063
	CommandListInterfaces    Command = 0x0064
	CommandRegisterSession   Command = 0x0065 // Begin Session Command
	CommandUnRegisterSession Command = 0x0066 // Close Session Command
	CommandSendRRData        Command = 0x006F // Send Unconnected Data Command
	CommandSendUnitData      Command = 0x0070 // Send Connnected Data Command
	CommandIndicateStatus    Command = 0x0072
	CommandCancel            Command = 0x0073
)

const (
	CPFTypeNull                     CPFType = 0x0000
	CPFTypeListIdentity             CPFType = 0x000c
	CPFTypeConnectionBased          CPFType = 0x00a1 //161
	CPFTypeConnectedTransportPacket CPFType = 0x00b1 //177
	CPFTypeUnconnectedMessage       CPFType = 0x00b2 //178
	CPFTypeListServices             CPFType = 0x0100
	CPFTypeSockInfoO2T              CPFType = 0x8000
	CPFTypeSockInfoT2O              CPFType = 0x8001
	CPFTypeSequencedAddrItem        CPFType = 0x8002
)
