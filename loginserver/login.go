package loginserver

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"geography/srvconfig"
	"io"
	"log"
	mrand "math/rand"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

/*
   This is the code for the builtin HTTP/S Login (account creation) Server

   This is what is enabled when HTTP Signup server is enabled



*/
var challenge_timeout time.Duration = time.Minute * 5
var root_dir string
type PowdataT struct{
	id int
	timer *time.Timer
}

type LoginResultT struct{
	timer *time.Timer
	LoginOK bool `json:"login_ok"`
	ErrMsg string `json:"err_msg"`
}

type PowHandlerT struct{
	Seeds map[int]string
	ActiveIds map[int]PowdataT
	Mtx sync.Mutex
	Results map[int]LoginResultT
}
func new_powhandler()(handle *PowHandlerT){
	handle = new(PowHandlerT)
	handle.Seeds = make(map[int]string)
	handle.ActiveIds = make(map[int]PowdataT)
	handle.Results = make(map[int]LoginResultT)
	return handle
}
func (handler *PowHandlerT)create_challenge()(int,string){
	handler.Mtx.Lock()
	defer handler.Mtx.Unlock()
	var id int
	for{
		id = int(mrand.Int31())
		_,found := handler.ActiveIds[id]
		if(!found){
			break;
		}
	}
	salt := make([]byte,64)
	_,err := rand.Read(salt)
	if(err != nil){
		log.Println(err)
	}
	str_seed := hex.EncodeToString(salt)
	handler.Seeds[id] = str_seed
	pow_data := PowdataT{
		id: id,
		timer: time.AfterFunc(challenge_timeout,func(){
			handler.delete_challenge(id)
		},
		),
	}
	handler.ActiveIds[id] = pow_data
	return id,str_seed
}

func (handler *PowHandlerT)delete_challenge(id int){
	handler.Mtx.Lock()
	defer handler.Mtx.Unlock()
	delete(handler.Seeds,id)
	
	pow_data,ok := handler.ActiveIds[id]
	if(!ok){
		return
	}
	pow_data.timer.Stop()
	delete(handler.ActiveIds,id)
}

func (handler *PowHandlerT)create_result(id int,passed bool,msg string){
	handler.Mtx.Lock()
	defer handler.Mtx.Unlock()
	result := LoginResultT{
		timer: time.AfterFunc(challenge_timeout,func(){}),
		LoginOK: passed,
		ErrMsg: msg,
	}
	handler.Results[id] = result
}

func (handler *PowHandlerT)pull_result(id int)(*LoginResultT){
	handler.Mtx.Lock()
	defer handler.Mtx.Unlock()
	res,ok := handler.Results[id]
	if(!ok){
		return nil
	}
	res.timer.Stop()
	delete(handler.Results,id)
	return &res
}



func (handler *PowHandlerT)verify_challenge(id int,nonce string)(bool){
	var challenge = "0000"
	handler.Mtx.Lock()
	defer handler.Mtx.Unlock()
	seed,ok := handler.Seeds[id]
	if(!ok){
		return false
	}
	concat := seed + nonce
	sum := sha256.Sum256([]byte(concat))
	hex_str := hex.EncodeToString(sum[:])
	first_4 := hex_str[0:4]
	if(first_4 != challenge){
		return false
	}
	return true
}

type LoginServerT struct{
	ServerDb *sql.DB // This will just be the Servers DB
	DbMtx *sync.Mutex // this is also just the servers Mutex
	HttpServer *http.Server
	Config HttpLoginConfigT
	PowHandler *PowHandlerT
}

type HttpLoginConfigT struct{
	HttpLoginCode string `json:"http_login_code"`
	HttpEnableProofOfWork bool `json:"http_enable_pow"`
}

type LoginDataT struct{
	RequireLoginCode bool `json:"RequiresLoginCode"`
	Seed string `json:"PowSeed"`
	LoginID int `json:"LoginId"`
}

type SignupRequestT struct{
	Username string `json:"username"`
	Password string `json:"password"`
	LoginCode string `json:"login_code"`
	LoginId int `json:"login_id"`
	Seed string `json:"seed"`
	Nonce string `json:"nonce"`
}
func(req *SignupRequestT)verify_id(handler *PowHandlerT)(bool){
	handler.Mtx.Lock()
	defer handler.Mtx.Unlock()
	//check if id exists first
	_,ok := handler.ActiveIds[req.LoginId]
	if(!ok){
		return false
	}
	seed,ok := handler.Seeds[req.LoginId]
	if(!ok){
		return false
	}
	if(req.Seed != seed){
		return false
	}
	return true
}




// this is the same function from the server.go file but I just c
func (srv *LoginServerT)create_user(username string,password string)(error){
	srv.DbMtx.Lock()
	defer srv.DbMtx.Unlock()
	exists := false
	srv.ServerDb.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ?);",username).Scan(&exists)
	if(exists){
		return fmt.Errorf("User '%s' already exists on server",username)
	}
	password_hash,err := bcrypt.GenerateFromPassword([]byte(password),bcrypt.DefaultCost)
	if(err != nil){
		log.Println(err)
		return err
	}
	_,err = srv.ServerDb.Exec("INSERT INTO users (username,hash) VALUES (?,?);",username,password_hash)
	if(err != nil){
		log.Println(err)
	}
	return nil
}


func safe_read_file(rel_filepath string)([]byte,error){
	cleaned := filepath.Clean(rel_filepath)
	cleaned = strings.TrimPrefix(cleaned,"/")
	localized,err := filepath.Localize(cleaned)
	if(err != nil){
		return nil,err
	}
	fullpath := filepath.Join(root_dir,localized)
	return os.ReadFile(fullpath)
}

func handle_root_request(writer http.ResponseWriter,request *http.Request){
	if request.URL.Path == "/" && request.Method == http.MethodGet{
		writer.WriteHeader(http.StatusOK)
		index_path := filepath.Join(root_dir,"index.html")
		index_data,err := os.ReadFile(index_path)
		if(err != nil){
			log.Printf("Failure to read index.html path with error `%s'\n",err.Error())
			http.Error(writer,"failure to read index.html",http.StatusInternalServerError)
			return
		}
		writer.Write(index_data)
	}else if request.Method == http.MethodGet{
		
		data,err := safe_read_file(request.URL.Path)
		if(err != nil){
			http.Error(writer,fmt.Sprintf("Error reading %s for reason %s",request.URL,err.Error()),http.StatusInternalServerError)
			return
		}
		ext := filepath.Ext(request.URL.Path)
		content_type := mime.TypeByExtension(ext)
		writer.Header().Set("Content-Type",content_type)
		writer.Write(data)
		
	}
}

func check_if_username_has_illegal_chars(username string)(bool){
	list := "* /\\#@!*()+=[]?><'\"`~,.|-%$^"
	return strings.ContainsAny(username,list)
}

func (srv *LoginServerT)handle_signup_post(login_contents SignupRequestT)(error){
	passed := login_contents.verify_id(srv.PowHandler)
	if(!passed){
		log.Printf("Login with id %d failed to verify id and seed\n",login_contents.LoginId)
		srv.PowHandler.delete_challenge(login_contents.LoginId)
		return fmt.Errorf("Failed to verify Id")
	}
	passed = srv.PowHandler.verify_challenge(login_contents.LoginId,login_contents.Nonce)
	if(!passed){
		log.Printf("Failure to complete proof of work failure: user %d with replied with nonce %s\n",
			login_contents.LoginId,
			login_contents.Nonce,
		)
		srv.PowHandler.delete_challenge(login_contents.LoginId)
		return fmt.Errorf("Incorrect nonce of %s",login_contents.Nonce)
	}
	srv.PowHandler.delete_challenge(login_contents.LoginId)
	if(srv.Config.HttpLoginCode != login_contents.LoginCode){
		return fmt.Errorf("Invaild Login code '%s' provided")
	}
	err := srv.create_user(login_contents.Username,login_contents.Password)
	if(err != nil){
		log.Println(err)
		return err
	}
	return nil
}

func (srv *LoginServerT)handle_signup_request(writer http.ResponseWriter,req *http.Request){
	defer req.Body.Close()
	if(req.Method == http.MethodPost){
		body_contents,err := io.ReadAll(req.Body)
		if(err != nil){
			http.Error(writer,err.Error(),http.StatusInternalServerError)
			return
		}
		signup_request := SignupRequestT{}
		err = json.Unmarshal(body_contents,&signup_request)
		if(err != nil){
			return
		}
		err = srv.handle_signup_post(signup_request)
		if(err != nil){
			srv.PowHandler.create_result(signup_request.LoginId,false,err.Error())
			return
		}
		srv.PowHandler.create_result(signup_request.LoginId,true,"")
	}else if (req.Method == http.MethodGet){
		writer.Header().Set("Content-Type","application/json")
		login_info := LoginDataT{}
		if(srv.Config.HttpLoginCode != ""){
			login_info.RequireLoginCode = true
		}
		id,seed := srv.PowHandler.create_challenge()
		login_info.LoginID = id
		login_info.Seed = seed
		login_info_json,err := json.Marshal(&login_info)
		if(err != nil){
			log.Println(err)
		}
		writer.Write(login_info_json)
	}else{
		http.Error(writer,"Invaild Method",http.StatusForbidden)
	}
}

func (srv *LoginServerT)request_login_result(w http.ResponseWriter,req *http.Request){
	id_str := req.PathValue("id")
	result_id,err := strconv.Atoi(id_str)
	if(err != nil){
		log.Printf("Failed to parse result id: %v\n",err)
	}
	result := srv.PowHandler.pull_result(result_id)
	payload,err := json.Marshal(result)
	if(err != nil){
		log.Println(err)
	}
	w.Header().Set("Content-Type","application/json")
	w.Write(payload)
}

func (srv *LoginServerT)Init(config *srvconfig.ConfigT, db *sql.DB,dbmtx *sync.Mutex){
	srv.HttpServer = new(http.Server)
	srv.HttpServer.Addr = config.HttpServerAddrAndPort
	srv.HttpServer.Protocols = new(http.Protocols)
	srv.HttpServer.Protocols.SetHTTP1(true)
	srv.HttpServer.Protocols.SetHTTP2(false)
	srv.Config.HttpEnableProofOfWork = config.HttpEnableProofOfWork
	srv.PowHandler = new_powhandler()
	srv.Config.HttpLoginCode = config.HttpLoginCode
	srv.DbMtx = dbmtx
	srv.ServerDb = db
	root_dir = filepath.Clean(config.HttpLoginDir)
	mux := http.NewServeMux()
	mux.HandleFunc("/",handle_root_request)
	mux.HandleFunc("/api/login",srv.handle_signup_request)
	mux.HandleFunc("GET /api/login/{id}",srv.request_login_result)
	srv.HttpServer.Handler = mux
	if(config.EnableTLS){
		err := srv.HttpServer.ListenAndServeTLS(config.TlsCertPath,config.TlsKeyPath)
		log.Println(err)
	}else{
		err := srv.HttpServer.ListenAndServe()
		if(err != nil){
			log.Println(err)
		}
	}
}
