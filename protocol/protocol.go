package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	gerrors "geography/errors"
	"io"
	"os"
)



type PacketType byte
type SizeT uint32
const SizeTSize = 4
const(
	PacketLogin PacketType = iota
	PacketLoginOk
	PacketSync
	PacketSyncResp
	PacketSend
	PacketNewConvo
	PacketNewConvoResp
	PacketNewMessage
	PacketSearchForUser
	PacketSearchForUserResp
	PacketGetUsernameFromID
	PacketGetUsernameResp
	PacketConnectionEstab
	PacketError
	PacketCloseConn
	__packet_type_end__
)

var PacketTypeStr = []string{
	"PacketLogin",
	"PacketLoginOk",
	"PacketSync",
	"PacketSyncResp",
	"PacketSend",
	"PacketNewConvo",
	"PacketNewConvoResp",
	"PacketNewMessage",
	"PacketSearchForUser",
	"PacketSearchForUserResp",
	"PacketGetUsernameFromID",
	"PacketGetUsernameResp",
	"PacketConnectionEstab",
	"PacketError",
	"PacketCloseConn",
}


/*
   Packet is made of two json object
   the  outer object  contains the object type,
   the inner object contains the actual packed data
   {
   "packet_type":0
   "payload":
       {
             "username":"john"
             "password":"1234"
       }
   }
*/


const HeaderSize SizeT = 5

var FailedToReadFullHeader = errors.New("Failed to Read Header packet")


type Packet struct{
	Type PacketType
	Payload any
}
type ErrPacketT struct{
	ErrorId  SizeT `json:"error_id"`
	Msg string `json:"error_msg"`
}

func CreateErrorPacket(err_msg string,err_id SizeT)(Packet){
	ret_packet := Packet{}
	ret_packet.Payload = ErrPacketT{Msg: err_msg,ErrorId: err_id}
	ret_packet.Type = PacketError
	return ret_packet
}


type LoginT struct{
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserT struct{
	Username string `json:"username"`
	UserID SizeT `json:"user_id"`
}

type ConvoT struct{
	ConvoId SizeT `json:"convo_id"`
	Name string   `json:"convo_name"`
	Users []UserT
}

type LoginOkT struct{
	UserID SizeT `json:"user_id"`
	Convos []ConvoT `json:"convos"`
	Token []byte `json:"auth_token"`
}

type SendT struct{
	SenderId SizeT `json:"sender_id"`
	ConvoId  SizeT `json:"convo_id"`
	Payload []byte `json:"msg_payload"`
	Token []byte   `json:"auth_token"`
	Time string    `json:"sent_time"`
}


// will just send okay
// serves no purpose, just though it'd be funny
type CloseConnT struct{
	Ok string `json:"closecon_bye"`
}




type NewConvoT struct{
	OwnerId SizeT `json:"owner_id"`
	UserIds []SizeT `json:"user_ids"`
	AuthToken []byte `json:"auth_token"`
	ConvoName string `json:"convo_name"`
}

type NewConvoRespT struct{
	ConvoData ConvoT `json:"convo_data"`
}


type SyncT struct{
	ConvoId SizeT `json:"convo_id"`
	UserId SizeT `json:"user_id"`
	MsgId SizeT `json:"msg_id"`
	AuthToken []byte `json:"auth_tokens"`
}


// In the future clients will add this to their
// local DB for messages.
type NewMessageT struct{
	ConvoId SizeT `json:"convo_id"`
	NewMessage MsgT `json:"new_msg"`
}

type MsgT struct{
	MsgId SizeT `json:"msg_id"`
	SenderID SizeT `json:"sender_id"`
	ConvoID SizeT  `json:"convo_id"`
	MsgPayload []byte `json:"msg_payload"`
	Time string `json:"msg_time"`
}

type SyncRespT struct{
	ConvoId SizeT `json:"convo_id"`
	UserId SizeT `json:"user_id"`
	Messages []MsgT `json:"messages"`
}

type SearchUserT struct{
	SenderId SizeT `json:"sender_id"`
	SearchStr string `json:"search_str"`
	AuthToken []byte `json:"auth_token"`
}

type SearchUserRespT struct{
	Ids []SizeT  `json:"found_ids"`
	Usernames []string `json:"found_usernames"`
}

type GetUsernameFromIdT struct{
	SenderId SizeT `json:"sender_id"`
	RequestId SizeT `json:"requested_id"`
	AuthToken []byte `json:"auth_token"`
}

type GetUsernameRespT struct{
	Username string `json:"found_username"`
}

type ConnectEnstabT struct{
	TlsEnabled bool `json:"tls_enabled"`
	HttpLoginSite string `json:"http_login_site"`
	TlsCert []byte`json:"tls_cert"`
}

//TODO -- HANDLE ERRORS
func SendPacket(p Packet, out io.Writer)(error){
	packet_json := json.RawMessage{}
	packet_json,_  = json.Marshal(p.Payload)
	send_packet := encapsulated_packet{Ptype:p.Type,Payload:packet_json}
	json_data,_ := json.Marshal(send_packet)
	_,err := out.Write(json_data)
	if(err != nil){
		return err
	}
	return nil
}



type encapsulated_packet struct{
	Ptype PacketType `json:"packet_type"`
	Payload json.RawMessage `json:"payload"`
}


func create_packet[T any](payload json.RawMessage)(ret T,err error){
	err = json.Unmarshal(payload,&ret)
	return
}


func ReadPacket(src io.Reader)(ret Packet,ret_err error){
	decoder := json.NewDecoder(src)
	raw_packet := encapsulated_packet{}
	err := decoder.Decode(&raw_packet)
	if(err != nil){
		if(err == io.EOF || err == io.ErrClosedPipe || err == io.ErrUnexpectedEOF || err == os.ErrDeadlineExceeded){
			return ret,err
		}
		return CreateErrorPacket(err.Error(),gerrors.SrvErrorJsonSyntaxError),nil
	}
	if(raw_packet.Ptype < 0 || raw_packet.Ptype >= __packet_type_end__){
		return CreateErrorPacket(fmt.Sprintf("Invaild packet type of '%d'\r\n",raw_packet.Ptype),gerrors.SrvErrorInvaildPacketType),nil
	}
	ret.Type = raw_packet.Ptype
	switch(raw_packet.Ptype){
		case PacketLogin:
		{
			ret.Payload,ret_err = create_packet[LoginT](raw_packet.Payload)
		}
		case PacketLoginOk:
		{
			ret.Payload,ret_err = create_packet[LoginOkT](raw_packet.Payload)
		}
		case PacketNewConvo:
		{
			ret.Payload,ret_err = create_packet[NewConvoT](raw_packet.Payload)
		}
		case PacketSend:
		{
			ret.Payload,ret_err = create_packet[SendT](raw_packet.Payload)
		}
		case PacketSync:
		{
			ret.Payload,ret_err = create_packet[SyncT](raw_packet.Payload)
		}
		case PacketCloseConn:{
			ret.Payload,ret_err = create_packet[CloseConnT](raw_packet.Payload)
		}
		case PacketError:
		{
			ret.Payload,ret_err = create_packet[ErrPacketT](raw_packet.Payload)
		}
		case PacketSyncResp:
		{
			ret.Payload,ret_err = create_packet[SyncRespT](raw_packet.Payload)
		}
		case PacketNewConvoResp:
		{
			ret.Payload,ret_err = create_packet[NewConvoRespT](raw_packet.Payload)
		}
		case PacketSearchForUser:
		{
			ret.Payload,ret_err = create_packet[SearchUserT](raw_packet.Payload)
		}
		case PacketSearchForUserResp:
		{
			ret.Payload,ret_err = create_packet[SearchUserRespT](raw_packet.Payload)
		}
		case PacketGetUsernameFromID:
		{
			ret.Payload,ret_err = create_packet[GetUsernameFromIdT](raw_packet.Payload)
		}
		case PacketGetUsernameResp:
		{
			ret.Payload,ret_err = create_packet[GetUsernameRespT](raw_packet.Payload)
		}
		case PacketNewMessage:
		{
			ret.Payload,ret_err = create_packet[NewMessageT](raw_packet.Payload)
		}
		case PacketConnectionEstab:
		{
			ret.Payload,ret_err = create_packet[ConnectEnstabT](raw_packet.Payload)
		}
		default:
		{
			//TODO -- add proper error handling here
			panic("unknown packet\r\n")
		}
	}
	return 
}
/*
   Just contains a error string for now
   I don't know what all erros I will have
   I will proably add a number for error ID or something
*/
