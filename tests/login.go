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
	login_packet := protocol.LoginT{}
	login_packet.Username = "Jane"
	login_packet.Password = "password1"
	protocol.SendPacket(protocol.Packet{Type:protocol.PacketLogin,Payload:login_packet,},conn)
	packet, err := protocol.ReadPacket(conn)
	if(err != nil){
		log.Println(err)
	}
	if(packet.Type != protocol.PacketLoginOk){
		log.Printf("Packet Received was not loginOK packet '%+v' aborting\n", packet) // Output: {Name:Alice Age:30}
		panic("bye")
	}
	login_state := packet.Payload.(protocol.LoginOkT)
	fmt.Printf("%+v\n",login_state)
}
