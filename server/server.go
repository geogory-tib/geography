package server

import (
	"crypto/tls"
	"database/sql"
	"strings"

	gerrors "geography/errors"
	"geography/protocol"
	"geography/srvconfig"
	"log"
	"net"
	"os"
	"slices"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)
var AckPacketTimeoutDuration int = 60



type UserIdT uint32


type User struct{
	id UserIdT
	username string
	token []byte
	conn net.Conn

}

type usrmngr struct{
	mtx sync.RWMutex
	active map[UserIdT]*User
}

func (mngr *usrmngr)get_user(id UserIdT)(*User,bool){
	mngr.mtx.RLock()
	defer mngr.mtx.RUnlock()
	user,found := mngr.active[id]
	return user,found
}

func (mngr *usrmngr)add_user(user *User){
	mngr.mtx.Lock()
	defer mngr.mtx.Unlock()
	mngr.active[user.id] = user
}

func (mngr *usrmngr)del_user(id UserIdT){
	mngr.mtx.Lock()
	defer mngr.mtx.Unlock()
	delete(mngr.active,id)
}


type Server struct{
	listener net.Listener
	Db *sql.DB
	DbMtx sync.Mutex
	users usrmngr
	Tls_Confg *tls.Config
	Config *srvconfig.ConfigT
	log_file os.File
}

func (srv *Server)configure_tls()(error){
	srv.Tls_Confg = new(tls.Config)
	cert,err := tls.LoadX509KeyPair(srv.Config.TlsCertPath,srv.Config.TlsKeyPath)
	if(err != nil){
		return err
	}
	srv.Tls_Confg.Certificates = append(srv.Tls_Confg.Certificates,cert)
	srv.Tls_Confg.MinVersion = tls.VersionTLS12
	//split_host := strings.Split(srv.Config.ServerAdressAndPort,":")
	//srv.Tls_Confg.ServerName = split_host[0]
	return nil
}

func (srv *Server)InitServer(){
	var err error
	srv.Config,err = srvconfig.LoadConfigFile()
	if(err != nil){
		log.Println(err)
	}
	log_file,err := os.Open(srv.Config.LogFilePath)
	if(err == nil){
		
		log.SetOutput(log_file)
	}else{
		log.Printf("Failed to Open Logfile '%s' logs will go to stdout\n",err.Error())
	}
	srv.listener,err = net.Listen("tcp",srv.Config.ServerAdressAndPort)
	if(err != nil){
		log.Panicf("Failed to open listener on port %s\n",srv.listener.Addr().String())
	}
	if(srv.Config.EnableTLS){
		srv.configure_tls()
	}
	srv.Db,err = sql.Open("sqlite3","./geo.db")
	if(err != nil){
		log.Panicf("Failed to open DB %s",err.Error())
	}
	_,err = srv.Db.Exec(`CREATE TABLE IF NOT EXISTS conversations(
                 id INTEGER PRIMARY KEY,
                 next_message_id INTEGER DEFAULT 0,
                 name STRING,
                 owner_id INTEGER);`)
	if(err != nil){
			log.Panic(err)
	}
	_,err = srv.Db.Exec(`CREATE TABLE IF NOT EXISTS conversation_users(
                 id INTEGER,
                 usr_id INTEGER);`)
	if(err != nil){
		log.Panic(err)
	}
	_,err = srv.Db.Exec(`CREATE TABLE IF NOT EXISTS messages(
                 msg_id INTEGER,
                 sender_id INTEGER,
                 convo_id INTEGER,
                 payload BLOB,
                 time STRING,
                 PRIMARY KEY(msg_id,convo_id));`)
	if(err != nil){
		log.Panic(err)
	}
	_,err = srv.Db.Exec(`CREATE TABLE IF NOT EXISTS users(
                 id INTEGER PRIMARY KEY AUTOINCREMENT,
                 username STRING,
                 hash BLOB,
                 UNIQUE(id,username));`)
	if(err != nil){
		log.Panic(err)
	}
	srv.users.active = make(map[UserIdT]*User,100)
}




func (srv *Server)ServerMain(){
	defer srv.listener.Close()
	for{
		conn, err := srv.listener.Accept()
		if(err != nil){
			log.Println(err)
			continue
		}
		go srv.handle_conn(conn)
	}
}


func (srv *Server)create_user(username string,password string)(){
	password_hash,err := bcrypt.GenerateFromPassword([]byte(password),bcrypt.DefaultCost)
	if(err != nil){
		log.Println(err)
		return
	}
	_,err = srv.Db.Exec("INSERT INTO users (username,hash) VALUES (?,?);",username,password_hash)
	if(err != nil){
		log.Println(err)
	}
}


func (srv *Server)check_packet_auth_token(token []byte,user_id UserIdT)(error,int){
	user,found := srv.users.get_user(user_id)
	if(!found){
		return gerrors.ErrUserIdNotActive,gerrors.SrvErrorUserIdNotActive
	}else if(!slices.Equal(user.token,token)){
		return gerrors.ErrInvaildAuthToken,gerrors.SrvErrorInvaildAuthToken
	}
	return nil,0
}


func (srv *Server)update_msg_db(send *protocol.SendT)(int,error,int){
	srv.DbMtx.Lock()
	defer srv.DbMtx.Unlock()
	msg := protocol.MsgT{SenderID:send.SenderId,ConvoID:send.ConvoId,MsgPayload:send.Payload}
	err := srv.Db.QueryRow("SELECT next_message_id FROM conversations WHERE id = ?;",send.ConvoId).Scan(&msg.MsgId)
	if(err != nil && err == sql.ErrNoRows){
		return 0,gerrors.ErrInvaildConvoId,gerrors.SrvErrorInvaildConvoId
	}else if(err != nil){
		return 0,err,-1
	}
	_,err = srv.Db.Exec("UPDATE conversations SET next_message_id = next_message_id  + 1 WHERE id = ?;",msg.ConvoID)
	if(err != nil){
		log.Println(err)
		return 0,nil,0
	}
	_,err = srv.Db.Exec("INSERT INTO messages (msg_id,sender_id,convo_id,payload,time) VALUES (?,?,?,?,?);",
		msg.MsgId,
		msg.SenderID,
		msg.ConvoID,
		msg.MsgPayload,
		msg.Time,
	)
	if(err != nil){
		log.Println(err)
		return 0,nil,0
	}
	return int(msg.MsgId),nil,0
}

func (srv *Server)get_convo_user_ids_db(convo_id protocol.SizeT)([]UserIdT,error,int){
	srv.DbMtx.Lock()
	defer srv.DbMtx.Unlock()
	rows, err := srv.Db.Query("SELECT (usr_id) FROM conversation_users WHERE id = ?;",convo_id)
	if(err == sql.ErrNoRows){
		return nil,gerrors.ErrInvaildConvoId,gerrors.SrvErrorInvaildConvoId
	}
	user_ids := []UserIdT{}
	for rows.Next(){
		user_id := 0
		rows.Scan(&user_id)
		user_ids = append(user_ids,UserIdT(user_id))
	}
	return user_ids,nil,0
}

func (srv *Server)handle_send(send *protocol.SendT)(error,int){
	msg_id := 0
	err,code := srv.check_packet_auth_token(send.Token,UserIdT(send.SenderId))
	if(err != nil){
		return err,code
	}
	msg_id,err,code = srv.update_msg_db(send)
	if(err != nil){
		return err,code
	}
	user_ids,err,code := srv.get_convo_user_ids_db(send.ConvoId)
	for _,id := range user_ids{
		user,found := srv.users.get_user(id)
		if(!found){
			continue
		}
		protocol.SendPacket(protocol.Packet{
			Type:protocol.PacketNewMessage,
			Payload:protocol.NewMessageT{
				ConvoId:send.ConvoId,
				NewMessage:protocol.MsgT{
					MsgId: protocol.SizeT(msg_id),
					SenderID:send.SenderId,
					ConvoID: send.ConvoId,
					MsgPayload:send.Payload,
					Time:send.Time,
				},
			},
		},user.conn)
	}
	return nil,0
}

func (srv *Server)create_convo_db(new_convo *protocol.NewConvoT)(row_id int,err error,error_code int){
	srv.DbMtx.Lock()
	defer srv.DbMtx.Unlock()
	_,err = srv.Db.Exec("INSERT INTO conversations (owner_id,name) VALUES (?,?);",new_convo.OwnerId,new_convo.ConvoName)
	if(err != nil){
		log.Println(err)
	}
	err = srv.Db.QueryRow("SELECT MAX(id) AS last_id FROM conversations;").Scan(&row_id)
	if(err != nil){
		log.Println(err)
	}
	for _,user_id := range new_convo.UserIds{
		row_exists := 0 // will be one if the row exists
		srv.Db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = ?) AS value_exists;",user_id).Scan(&row_exists)
		if(row_exists != 1){
			err = gerrors.ErrInvaildUserId
			error_code = gerrors.SrvErrorInvaildUserId
			return 
		}
		_,err = srv.Db.Exec("INSERT INTO conversation_users (id,usr_id) VALUES (?,?);",row_id,user_id)
		if(err != nil){
			log.Println(err)
		}
	}
	return
}

func (srv *Server)get_username_from_id_db(id protocol.SizeT)(username string,err error,code int){
	srv.DbMtx.Lock()
	defer srv.DbMtx.Unlock()
	err = srv.Db.QueryRow("SELECT username FROM users WHERE id = ?",id).Scan(&username)
	if(err == sql.ErrNoRows){
		return "",gerrors.ErrInvaildUserId,gerrors.SrvErrorInvaildUserId
	}else if(err != nil){
		return "",err,0
	}
	return
}

func (srv *Server)gen_convo_name(new_convo *protocol.NewConvoT)(error,int){
	name_builder := strings.Builder{}
	for _,id := range new_convo.UserIds{
		username,err,code := srv.get_username_from_id_db(id)
		if(err != nil){
			return err,code
		}
		name_builder.WriteString(username + ",")
	}
	new_convo.ConvoName = name_builder.String()
	return nil,0
}

func (srv *Server)create_convo(new_convo *protocol.NewConvoT)(error,int){
	err,error_id := srv.check_packet_auth_token(new_convo.AuthToken,UserIdT(new_convo.OwnerId))
	if(err != nil){
		return err,error_id
	}
	if(new_convo.ConvoName == ""){
		err,code := srv.gen_convo_name(new_convo)
		if(err != nil){
			return err,code
		}
	}
	row_id,err,error_id := srv.create_convo_db(new_convo)
	if(err != nil){
		return err,error_id
	}
	convo := protocol.ConvoT{
		Name:new_convo.ConvoName,
		ConvoId: new_convo.OwnerId,
	}
	resp_packet := protocol.Packet{Type: protocol.PacketNewConvoResp, Payload: protocol.NewConvoRespT{ConvoData:convo}}
	for _,id := range new_convo.UserIds{
		user,ok := srv.users.get_user(UserIdT(id))
		if(!ok){
			continue
		}
		protocol.SendPacket(resp_packet,user.conn)
	}
	return nil,row_id
}

func (srv *Server)send_error_packet(err error,code int,user *User){
	log.Printf("Error packet with code %d sent to user '%s':\r\n '%s'",code,user.username,err.Error())
	error_packet := protocol.Packet{
		Type:protocol.PacketError,
		Payload: protocol.ErrPacketT {
			ErrorId:protocol.SizeT(code),
				Msg:err.Error(),
			},
	}
	protocol.SendPacket(error_packet,user.conn)
}

func (srv *Server)grab_messages_from_db(start protocol.SizeT,convo_id protocol.SizeT)([]protocol.MsgT,error,int){
	srv.DbMtx.Lock()
	defer srv.DbMtx.Unlock()
	messages := []protocol.MsgT{}
	rows,err := srv.Db.Query("SELECT msg_id,sender_id,payload,time FROM messages WHERE convo_id = ? AND msg_id >= ?;",convo_id,start)
	if(err != nil){
		if(err == sql.ErrNoRows){
			return nil,gerrors.ErrInvaildConvoId,gerrors.SrvErrorInvaildConvoId
		}
		return nil,err,-1
	}
	for rows.Next(){
		msg := protocol.MsgT{}
		err := rows.Scan(&msg.MsgId,&msg.SenderID,&msg.MsgPayload,&msg.Time)
		if(err != nil){
			log.Println(err)
			return nil,nil,0
		}
		msg.ConvoID = convo_id
		messages = append(messages,msg)
	}
	return messages,nil,0
}

func (srv *Server)handle_sync(sync *protocol.SyncT,user *User)(error,int){
	err,code := srv.check_packet_auth_token(sync.AuthToken,UserIdT(sync.UserId))
	if(err != nil){
		return err,code
	}
	messages,err,code := srv.grab_messages_from_db(sync.MsgId,sync.ConvoId)
	if(err != nil){
		return err,code
	}
	sync_resp := protocol.SyncRespT{
		ConvoId:sync.ConvoId,
		UserId: sync.UserId,
		Messages:messages,
	}
	protocol.SendPacket(protocol.Packet{Type:protocol.PacketSyncResp,Payload:sync_resp},user.conn)
	return nil,0
}

func (srv *Server)search_users_with_pattern_db(pattern string)(usernames  []string,ids []protocol.SizeT){
	srv.DbMtx.Lock()
	srv.DbMtx.Unlock()
	rows,err := srv.Db.Query("SELECT username,id FROM users WHERE username LIKE ?",pattern)
	if(err == sql.ErrNoRows){
		return nil,nil
	}else if (err != nil){
		log.Println(err)
		return nil,nil
	}
	for rows.Next(){
		username := ""
		id := 0
		err := rows.Scan(&username,&id)
		if(err != nil){
			log.Println(err)
		}
		usernames = append(usernames,username)
		ids = append(ids,protocol.SizeT(id))
	}
	return
}

func (srv *Server)handle_search_for_user(search *protocol.SearchUserT)(error,int){
	user,found := srv.users.get_user(UserIdT(search.SenderId))
	if(!found){
		return gerrors.ErrUserIdNotActive,gerrors.SrvErrorUserIdNotActive
	}
	err,code := srv.check_packet_auth_token(search.AuthToken,UserIdT(search.SenderId))
	if(err != nil){
		return err,code
	}
	users,ids := srv.search_users_with_pattern_db(search.SearchStr)
	resp_packet := protocol.Packet{
		Type:protocol.PacketSearchForUserResp,
		Payload:protocol.SearchUserRespT{
			Ids:ids,
			Usernames:users,
		},
	}
	protocol.SendPacket(resp_packet,user.conn)
	return nil,0
}
func (srv *Server)user_loop(user *User){
	for{		
		packet,err := protocol.ReadPacket(user.conn)
		if(err != nil){
			
		}
		switch packet.Type{
			case protocol.PacketSend:
			{
				send_packet := packet.Payload.(protocol.SendT)
				err,error_id := srv.handle_send(&send_packet)
				if(err != nil){
					srv.send_error_packet(err,error_id,user)
				}
			}
			case protocol.PacketSync:
			{
				sync_packet := packet.Payload.(protocol.SyncT)
				err,error_id := srv.handle_sync(&sync_packet,user)
				if(err != nil){
					srv.send_error_packet(err,error_id,user)
				}
			}
			case protocol.PacketNewConvo:
			{
				newconvo_packet := packet.Payload.(protocol.NewConvoT)
				err,error_id := srv.create_convo(&newconvo_packet)
				if(err != nil){
					srv.send_error_packet(err,error_id,user)
				}
			}
			case protocol.PacketSearchForUser:
			{
				search_packet := packet.Payload.(protocol.SearchUserT)
				err,error_id := srv.handle_search_for_user(&search_packet)
				if(err != nil){
					srv.send_error_packet(err,error_id,user)
				}
			}
			case protocol.PacketError:
			{
				protocol.SendPacket(packet,user.conn)
			}
			case protocol.PacketCloseConn:
			{
				srv.users.del_user(user.id)
				return
			}
			default:

		}
	}
}


func (srv *Server)get_user_session(conn net.Conn,user *User)(bool,error,int){
	login_packet,err := protocol.ReadPacket(conn)
	if(err != nil){
		
	}
	password_blob := []byte{}
	if(login_packet.Type != protocol.PacketLogin){
		return false, gerrors.ErrExpectedLoginPacket,gerrors.SrvErrorExpectedLoginPacket
	}
	login,_ := login_packet.Payload.(protocol.LoginT)
	srv.DbMtx.Lock()
	defer srv.DbMtx.Unlock()
	err = srv.Db.QueryRow("SELECT hash,id FROM users WHERE username = ?;",login.Username).Scan(&password_blob,&user.id)
	if(err == sql.ErrNoRows && srv.Config.AllowAccountCreationFromFailedLogin){
		srv.create_user(login.Username,login.Password)
		srv.Db.QueryRow("SELECT id FROM users WHERE username = ?;",login.Username).Scan(&user.id)
		user.conn = conn
		time_str := time.Now().String()
		user.token,err = bcrypt.GenerateFromPassword([]byte(time_str),bcrypt.MinCost)
		if(err != nil){
			return false,gerrors.ErrFailureToCreateUserToken,gerrors.SrvErrorFailureToCreateUserToken
		}
		user.username = login.Username
		srv.users.add_user(user)
		return true,nil,0
	}
	err = bcrypt.CompareHashAndPassword(password_blob,[]byte(login.Password))
	if(err != nil){
		if(err == bcrypt.ErrMismatchedHashAndPassword){
			return false,gerrors.ErrInvaildPassword,gerrors.SrvErrorInvaildPassword
		}else{
			log.Println(err)
		}
	}
	user.conn = conn
	//token is just generated from the string of the current time and bcrypted
	time_str := time.Now().String()
	user.token,err = bcrypt.GenerateFromPassword([]byte(time_str),bcrypt.MinCost)
	if(err != nil){
		return false,gerrors.ErrFailureToCreateUserToken,gerrors.SrvErrorFailureToCreateUserToken
	}
	srv.users.add_user(user)
	return true,nil,0
}
// generates the LoginOk packet to send to the user
// contains this conversations and the users auth token
func (srv *Server)get_usernames_and_ids_from_convo_db(convo_id protocol.SizeT)([]protocol.UserT,error,int){
	srv.DbMtx.Lock()
	defer srv.DbMtx.Unlock()
	ret_users := []protocol.UserT{}
	rows,err := srv.Db.Query("SELECT usr_id FROM conversation_users WHERE id = ?;",convo_id)
	if(err != nil){
		return nil,err,0
	}
	user_ids := []protocol.SizeT{}
	for rows.Next(){
		user_id := protocol.SizeT(0)
		err = rows.Scan(&user_id)
		if(err != nil){
			log.Println(err)
		}
		user_ids = append(user_ids,user_id)
	}
	for _,id := range user_ids{
		username := ""
		err = srv.Db.QueryRow("SELECT username FROM users WHERE id = ?;",id).Scan(&username)
		if(err != nil){
			log.Println(err)
		}
		user := protocol.UserT{
			Username:username,
			UserID:id,
		}
		ret_users = append(ret_users,user)
	}
	return ret_users,nil,0
}
func (srv *Server)get_convo_ids_from_user_db(user_id protocol.SizeT)([]protocol.ConvoT){
	ret_ids := []protocol.ConvoT{}
	srv.DbMtx.Lock()
	defer srv.DbMtx.Unlock()
	rows,err := srv.Db.Query("SELECT id FROM conversation_users WHERE usr_id = ?;",user_id)
	if(err != nil){
		log.Println(err)
		return nil
	}
	for(rows.Next()){
		convo_id := protocol.SizeT(0)
		convo_name := ""
		err := rows.Scan(&convo_id)
		if(err != nil){
			log.Println(err)
			return nil
		}
		if(convo_id > 0){
			err = srv.Db.QueryRow("SELECT name FROM conversations WHERE id = ?;",convo_id).Scan(&convo_name)
		}
		
		if(err != nil){
			log.Println(err)
			return nil
		}
		convo := protocol.ConvoT{
			Name:convo_name,
			ConvoId:convo_id,
		}
		ret_ids = append(ret_ids,convo)
	}
	return ret_ids
}
func (srv *Server)create_login_state(user *User)(protocol.LoginOkT,bool){
	state := protocol.LoginOkT{}
	state.Token = user.token
	state.UserID = protocol.SizeT(user.id)
	state.Convos = srv.get_convo_ids_from_user_db(protocol.SizeT(user.id))
	for i := range len(state.Convos){
		usernames,_,_ := srv.get_usernames_and_ids_from_convo_db(state.Convos[i].ConvoId)
		state.Convos[i].Users = usernames
	}
	return state,true
}
func (srv *Server)upgrade_conn_to_tls(conn net.Conn)(*tls.Conn,error,int){
	tls_conn := tls.Server(conn,srv.Tls_Confg)
	err := tls_conn.Handshake()
	if(err != nil){
		return tls_conn,err,0
	}
	return tls_conn,nil,0
}
func (srv *Server)handle_conn(conn net.Conn){
	defer conn.Close()
	conn_state := protocol.Packet{
		Type: protocol.PacketConnectionEstab,
		Payload: protocol.ConnectEnstabT{
			TlsEnabled: srv.Config.EnableTLS,
			HttpLoginSite:srv.Config.HttpServerAddrAndPort,
			TlsCert:srv.Tls_Confg.Certificates[0].Leaf.Raw,
		},
	}
	protocol.SendPacket(conn_state,conn)
	if(srv.Config.EnableTLS){
		new_conn,err,_ := srv.upgrade_conn_to_tls(conn)
		if(err != nil){
			log.Println(err)
		}
		conn = new_conn
	}
	
	logged_in_user := new(User)
	login_ok := false
	for  _ = range srv.Config.DropConnectionAfterXLogins{
		found,err,code := srv.get_user_session(conn,logged_in_user)
		if(err != nil){
			err_packet := protocol.CreateErrorPacket(err.Error(),protocol.SizeT(code))
			protocol.SendPacket(err_packet,conn)
		}
		if(found){
			login_ok = found
			break
		}
	}
	if(!login_ok){
		protocol.SendPacket(
			protocol.CreateErrorPacket(gerrors.ErrRejecDueToTooManyFailedLogins.Error(),
				gerrors.SrvErrorRejectDueToTooManyFailedLogins),conn)
		return
	}
	login_state,found := srv.create_login_state(logged_in_user)
	if(!found){
		return
	}
	protocol.SendPacket(protocol.Packet{Type:protocol.PacketLoginOk,Payload:login_state},conn)
	srv.user_loop(logged_in_user)
}
