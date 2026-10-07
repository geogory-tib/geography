package main

import (
	"geography/server"
)



func main(){
	server := new(server.Server)
	server.InitServer()
	server.ServerMain()
}
