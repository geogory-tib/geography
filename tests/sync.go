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
	sync_packet := protocol.Packet{
		Type:protocol.PacketSync,
		Payload:protocol.SyncT{
			ConvoId:login_state.ConvoIDs[1],
			UserId:login_state.UserID,MsgId:0,
			AuthToken:[]byte{1,2,3,4},
		},
	}
	protocol.SendPacket(sync_packet,conn)
	ret,_ := protocol.ReadPacket(conn)
	if(ret.Type != protocol.PacketSyncResp){
		fmt.Printf("%+v\n",ret)
		panic("invaild packet type\n")
	}
	sync_resp := ret.Payload.(protocol.SyncRespT)
	fmt.Printf("%+v\n",sync_resp)
	for _,msg := range sync_resp.Messages{
		fmt.Printf("convo_id = %d msg_id = %d sender_id = %d, '%s'\n",msg.ConvoID,msg.MsgId,msg.SenderID,string(msg.MsgPayload))
	}
}
