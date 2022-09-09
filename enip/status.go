package enip

import "strconv"

type Status uint32

const (
	StatusSuccess            Status = 0x0000
	StatusUnsupportedCommand Status = 0x0001
	StatusOutOfMemory        Status = 0x0002
	StatusIncorrectData      Status = 0x0003
	StatusInvalidSession     Status = 0x0064
	StatusInvalidLength      Status = 0x0065
	StatusUnSupportedVersion Status = 0x0069
)

//ParseStatus 格式化状态
func ParseStatus(status Status) string {
	switch status {
	case StatusSuccess:
		return "SUCCESS"
	case StatusUnsupportedCommand:
		return "FAIL: Sender issued an invalid ecapsulation command."
	case StatusOutOfMemory:
		return "FAIL: Insufficient memory resources to handle command."
	case StatusIncorrectData:
		return "FAIL: Poorly formed or incorrect data in encapsulation packet."
	case StatusInvalidSession:
		return "FAIL: Originator used an invalid session handle."
	case StatusInvalidLength:
		return "FAIL: Target received a message of invalid length."
	case StatusUnSupportedVersion:
		return "FAIL: Unsupported encapsulation protocol revision."
	default:
		return "FAIL: General failure " + strconv.Itoa(int(status)) + " occured."
	}
}
