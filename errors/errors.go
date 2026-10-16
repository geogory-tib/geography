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
	SrvErrorMessageNotFound
)


var ErrInvaildAuthToken error = errors.New("Authtoken from request does not match UserId's")
var ErrUserIdNotActive error = errors.New("UserID in request is not active")
var ErrInvaildConvoId error = errors.New("Invaild Convo ID")
var ErrInvaildUserId error = errors.New("Invaild User ID")
var ErrExpectedLoginPacket error = errors.New("Expected Login Packet")
var ErrExpectedConnEstabOkPacket = errors.New("Expected Connection Established OK packet")
var ErrInvaildPassword error = errors.New("Invaild User Password")
var ErrFailureToCreateUserToken error = errors.New("Error creating user token")
var ErrRejecDueToTooManyFailedLogins error = errors.New("Connection Dropped Due to Too Many Failed Logins")
var ErrUnknownUsername error
var ErrUserNotInConvo error = errors.New("User Id not in Conversation")
var ErrTlsNotEnabled error = errors.New("Server has TLS disabled, cert cannot be requested")
var ErrMessageNotFound error = errors.New("Requested message was nnot found")
