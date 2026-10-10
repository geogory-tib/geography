package errors

import "errors"

// Contains error defines for errors the server will send to a client

const (
	SrvErrorJsonSyntaxError = iota
	SrvErrorInvaildPacketType
	SrvErrorExpectedLoginPacket
	SrvErrorInvaildAuthToken
	SrvErrorUserIdNotActive
	SrvErrorInvaildConvoId
	SrvErrorInvaildUserId
	SrvErrorInvaildPassword
	SrvErrorFailureToCreateUserToken
	SrvErrorRejectDueToTooManyFailedLogins
	SrvErrorUnkownUsername
	SrvErrorExpectedConnEstabOkPacket
	SrvErrorTlsNotEnabled
	SrvErrorUserNotInConvo
)


var ErrInvaildAuthToken error = errors.New("Authtoken from request does not match UserId's\r\n")
var ErrUserIdNotActive error = errors.New("UserID in request is not active\r\n")
var ErrInvaildConvoId error = errors.New("Invaild Convo ID\r\n")
var ErrInvaildUserId error = errors.New("Invaild User ID\r\n")
var ErrExpectedLoginPacket error = errors.New("Expected Login Packet\r\n")
var ErrExpectedConnEstabOkPacket = errors.New("Expected Connection Established OK packet\r\n")
var ErrInvaildPassword error = errors.New("Invaild User Password\r\n")
var ErrFailureToCreateUserToken error = errors.New("Error creating user token\r\n")
var ErrRejecDueToTooManyFailedLogins error = errors.New("Connection Dropped Due to Too Many Failed Logins\r\n")
var ErrUnknownUsername error
var ErrUserNotInConvo error = errors.New("User Id not in Conversation \r\n")
var ErrTlsNotEnabled error = errors.New("Server has TLS disabled, cert cannot be requested")
