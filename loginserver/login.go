package loginserver

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"geography/srvconfig"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

/*
   This is the code for the builtin HTTP/S Login (account creation) Server

   This is what is enabled when HTTP Signup server is enabled



*/

var root_dir string

type LoginServerT struct{
	ServerDb *sql.DB // This will just be the Servers DB
	DbMtx *sync.Mutex // this is also just the servers Mutex
	HttpServer *http.Server
}
// this is the same function from the server.go file but I just c
func (srv *LoginServerT)create_user(username string,password string)(){
	srv.DbMtx.Lock()
	defer srv.DbMtx.Unlock()
	password_hash,err := bcrypt.GenerateFromPassword([]byte(password),bcrypt.DefaultCost)
	if(err != nil){
		log.Println(err)
		return
	}
	_,err = srv.ServerDb.Exec("INSERT INTO users (username,hash) VALUES (?,?);",username,password_hash)
	if(err != nil){
		log.Println(err)
	}
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


func (srv *LoginServerT)handle_signup_request(writer http.ResponseWriter,req *http.Request){
	defer req.Body.Close()
	if(req.Method != http.MethodPost){
		http.Error(writer,"Invaild Request method for URL",http.StatusForbidden)
		return
	}
	body_contents,err := io.ReadAll(req.Body)
	if(err != nil){
		http.Error(writer,err.Error(),http.StatusInternalServerError)
		return
	}
	var login_contents map[string]string
	json.Unmarshal(body_contents,&login_contents)
	username,_ := login_contents["username"]
	password,_ := login_contents["password"]
	if(check_if_username_has_illegal_chars(username) || len(username) > 24){
		return
	}else if(len([]byte(password)) > 72){
		return
	}
	srv.create_user(username,password)
}

func (srv *LoginServerT)Init(config *srvconfig.ConfigT, db *sql.DB,dbmtx *sync.Mutex){
	srv.HttpServer = new(http.Server)
	srv.HttpServer.Addr = config.HttpServerAddrAndPort
	srv.HttpServer.Protocols = new(http.Protocols)
	srv.HttpServer.Protocols.SetHTTP1(true)
	srv.HttpServer.Protocols.SetHTTP2(false)
	srv.DbMtx = dbmtx
	srv.ServerDb = db
	root_dir = filepath.Clean(config.HttpLoginDir)
	mux := http.NewServeMux()
	mux.HandleFunc("/",handle_root_request)
	mux.HandleFunc("/api/login",srv.handle_signup_request)
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
