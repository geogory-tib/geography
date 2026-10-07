package main

import (
	//	"fmt"
	"geography/protocol"
	"log"
	"net"
	"time"
)



func main(){
	conn,err := net.Dial("tcp","localhost:6969")
	defer conn.Close()
	if(err != nil){
		log.Println(err)
	}
	login_packet := protocol.LoginT{}
	login_packet.Username = "John"
	login_packet.Password = "password"
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
	new_message := protocol.SendT{}
	new_message.ConvoId = 3
	new_message.SenderId = login_state.UserID
	new_message.Time = time.Now().String()
	new_message.Token = login_state.Token
	new_message.Payload = []byte("New Message from John sent at " + new_message.Time)
	protocol.SendPacket(protocol.Packet{Type:protocol.PacketSend,Payload:new_message},conn)
}
