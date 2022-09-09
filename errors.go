package gologix

import "strconv"

//GetErrorCode 解析数据错误
func GetErrorCode(status uint8) string {
	switch status {
	case 0:
		return "Success"
	case 1:
		return "Connection failure"
	case 2:
		return "Resource unavailable"
	case 3:
		return "Invalid parameter value"
	case 4:
		return "Path segment error"
	case 5:
		return "Path destination unknown"
	case 6:
		return "Partial transfer"
	case 7:
		return "Connection lost"
	case 8:
		return "Service not supported"
	case 9:
		return "Invalid Attribute"
	case 10:
		return "Attribute list error"
	case 11:
		return "Already in requested mode/state"
	case 12:
		return "Object state conflict"
	case 13:
		return "Object already exists"
	case 14:
		return "Attribute not settable"
	case 15:
		return "Privilege violation"
	case 16:
		return "Device state conflict"
	case 17:
		return "Reply data too large"
	case 18:
		return "Fragmentation of a premitive value"
	case 19:
		return "Not enough data received"
	case 20:
		return "Attribute not supported"
	case 21:
		return "Too much data"
	case 22:
		return "Object does not exist"
	case 23:
		return "Service fragmentation sequence not in progress"
	case 24:
		return "No stored attribute data"
	case 25:
		return "Store operation failure"
	case 26:
		return "Routing failure, request packet too large"
	case 27:
		return "Routing failure, response packet too large"
	case 28:
		return "Missing attribute list entry data"
	case 29:
		return "Invalid attribute value list"
	case 30:
		return "Embedded service error"
	case 31:
		return "Vendor specific"
	case 32:
		return "Invalid Parameter"
	case 33:
		return "Write once value or medium already written"
	case 34:
		return "Invalid reply received"
	case 35:
		return "Buffer overflow"
	case 36:
		return "Invalid message format"
	case 37:
		return "Key failure in path"
	case 38:
		return "Path size invalid"
	case 39:
		return "Unexpected attribute in list"
	case 40:
		return "Invalid member ID"
	case 41:
		return "Member not settable"
	case 42:
		return "Group 2 only server general failure"
	case 43:
		return "Unknown Modbus error"
	case 44:
		return "Attribute not gettable"
	default:
		return "Unknown error " + strconv.Itoa(int(status))
	}
}
