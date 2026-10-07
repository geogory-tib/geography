package srvconfig

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"

)

var ConfigPaths = []string{
	"geoconfig.json",
	"/var/geoconfig.json",
	"/usr/shar/geoconfig.json",
}
var ErrConfigSyntaxError error

var ErrNoConfigFound error 

func init(){
	home_dir,err := os.UserHomeDir()
	if(err != nil){
		log.Println("GEOGRAPHY ERROR: Failed to get User's home dir, local config will not be found")
		return
	}
	
	local_conf_dir := home_dir + "/.geoconfig.json"
	ConfigPaths = slices.Insert(ConfigPaths,0,local_conf_dir)
	ErrNoConfigFound = fmt.Errorf("No Config was found in config Directories %q. Using Default Config\n",ConfigPaths)
}



type ConfigT struct{
	ServerAdressAndPort string `json:"addr_and_port"`
	AllowAccountCreationFromFailedLogin bool `json:"allow_account_creation_from_failed_login"`
	EnableHttpSignUpServer  bool `json:"enable_http_signup_server"`
	HttpServerAddrAndPort string
	EnableTLS bool `json:"enable_tls"`
	TlsCertPath string `json:"tls_cert_path"`
	TlsKeyPath string `json:"tls_key_path"`
	DropConnectionAfterXLogins int `json:"drop_connection_after_x_failed_logins"`
	LogFilePath string `json:"LogFilePath"`
}

var DefaultConfig = ConfigT{
	ServerAdressAndPort:"localhost:6969",
	AllowAccountCreationFromFailedLogin:true,
	EnableHttpSignUpServer:false,
	HttpServerAddrAndPort:"localhost:6970",
	EnableTLS:false,
	TlsCertPath:"",
	DropConnectionAfterXLogins:4,
	LogFilePath:"",
}

func LoadConfigFile()(*ConfigT,error){
	conf := new(ConfigT)
	config_json := []byte{}
	config_found := false
	config_found_ind := 0
	for ind,path := range ConfigPaths{
		data,err := os.ReadFile(path)
		if err == nil{
			config_found = true
			config_json = data
			config_found_ind = ind
			break
		}
	}
	if(!config_found){
		def_config,_ := json.MarshalIndent(&DefaultConfig,"","  ")
		os.WriteFile("geoconfig.json",def_config,0777)
		return &DefaultConfig,ErrNoConfigFound
	}
	err := json.Unmarshal(config_json,conf)
	if(err != nil){
		ErrConfigSyntaxError = fmt.Errorf("Config Syntax error '%s' for config file '%s'. Using Default Config",
			err.Error(),
			ConfigPaths[config_found_ind])
		return &DefaultConfig,ErrConfigSyntaxError
	}
	return conf,nil
}


