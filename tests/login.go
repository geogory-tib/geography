package main

import (
	"fmt"
	"geography/protocol"
	"net"
	"log"
)

func main(){
	conn,err := net.Dial("tcp","localhost:6969")
	defer conn.Close()
	if(err != nil){
		log.Println(err)
	}
	packet,_ := protocol.ReadPacket(conn)
	if(packet.Type != protocol.PacketConnectionEstab){
		panic("Unexpected packet type")
	}
	conn_ok := protocol.Packet{
		Type:protocol.PacketConnectionEstabOk,
		Payload:protocol.ConnEnstabOkT{
		},
	}
	protocol.SendPacket(conn_ok,conn)
	// this packet has the wrong type for the payload
	botched_packet := protocol.Packet{
		Type:protocol.PacketLogin,
		Payload:protocol.ConnEnstabOkT{
			
		},
	}
	protocol.SendPacket(botched_packet,conn)
	packet,_ = protocol.ReadPacket(conn)
	fmt.Printf("%+v\n", packet)
}
