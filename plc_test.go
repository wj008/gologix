package gologix

import (
	"bytes"
	"github.com/wj008/gologix/enip"
	"github.com/wj008/gologix/lib"
	"log"
	"testing"
)

func TestPLC_Connect(t *testing.T) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	plc := NewPLC()
	//plc.Logger = log.Default()
	//plc.Micro800=true
	err := plc.Connect("192.168.0.100:44818", 0)
	if err != nil {
		log.Println(err.Error())
		return
	}
	plc.OnClose = func() {
		log.Println("链接已经关闭..")
	}
	plc.RegisterSession()
	plc.ForwardOpen()
	result, err := plc.ReadTag("P_REAL", 3)
	for s, i := range result.Values {
		log.Println(s, i)
	}
	result, err = plc.ReadTag("A_BOOL", 4)
	for s, i := range result.Values {
		log.Println(s, i)
	}
	values2, err := plc.MultiReadTag([]string{"DPT02.OFFSET", "PSV05.PSV_ON", "A_BOOL[1]", "A_BOOL[2]", "A_BOOL[3]", "A_BOOL[4]", "P_REAL[999]"})
	for s, i := range values2 {
		log.Println(s, i.Value, i.Status)
	}
	plc.ForwardClose()
	select {}
}

func TestPLC_UnConnect(t *testing.T) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	plc := NewPLC()
	err := plc.Connect("192.168.0.100:44818", 0)
	if err != nil {
		log.Println(err.Error())
		return
	}
	plc.OnClose = func() {
		log.Println("链接已经关闭..")
	}
	plc.RegisterSession()
	result, err := plc.ReadTag("P_REAL", 3)
	for s, i := range result.Values {
		log.Println(s, i)
	}
	result, err = plc.ReadTag("A_BOOL", 4)
	for s, i := range result.Values {
		log.Println(s, i)
	}
	values2, err := plc.MultiReadTag([]string{"DPT02.OFFSET", "PSV05.PSV_ON", "A_BOOL[1]", "A_BOOL[2]", "A_BOOL[3]", "A_BOOL[4]", "P_REAL[999]"})
	for s, i := range values2 {
		log.Println(s, i.Value)
	}
	select {}
}

func TestPLC_ReadAttributeAll(t *testing.T) {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	plc := NewPLC()
	plc.Logger = log.Default()
	//plc.Micro800=true
	err := plc.Connect("192.168.0.100:44818", 0)
	if err != nil {
		log.Println(err.Error())
		return
	}
	plc.OnClose = func() {
		log.Println("链接已经关闭..")
	}
	plc.RegisterSession()
	plc.ReadAttributeAll()
	select {}
}

func TestGenerateEncodedTimeout(t *testing.T) {
	log.Println(enip.GenerateEncodedTimeout(5000))
	buffer := new(bytes.Buffer)
	lib.WriteByte(buffer, uint16(100))
	log.Println(buffer.Bytes())
}
