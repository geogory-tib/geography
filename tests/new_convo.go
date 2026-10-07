package main

import (
	"fmt"
	"geography/protocol"
	"log"
	"net"
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
	New_Convo_Packet := protocol.NewConvoT{}
	New_Convo_Packet.AuthToken = login_state.Token
	New_Convo_Packet.OwnerId = login_state.UserID
	New_Convo_Packet.UserIds = []protocol.SizeT{login_state.UserID,login_state.UserID+1}
	protocol.SendPacket(protocol.Packet{Type:protocol.PacketNewConvo,Payload:New_Convo_Packet},conn)
	packet,err =  protocol.ReadPacket(conn)
	fmt.Printf("%+v\n", packet)
	close_conn := protocol.Packet{Type:protocol.PacketCloseConn,Payload:protocol.CloseConnT{Ok:"Bye bye"}}
	protocol.SendPacket(close_conn,conn)
}
