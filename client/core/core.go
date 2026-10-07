package core

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"geography/protocol"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

/*
   Client Core module

   WARNING THIS CLIENT IS A TESTING TOOL
   THE UI FOR IT SUCKS
   AND IT HONESTLY IS HORRIBLE TO USE
   I am working on a text based client
   as I also want one of those, that will
   probably be the first one I make since it
   will be simpiler.
   Until then use this at  your own risk
   
*/




type ClientT struct{
	ServerConn net.Conn
	UserState protocol.LoginOkT
	ClientApp fyne.App
	ClientWindow fyne.Window
	login_form LoginFormT
	MessagingUi MessagingUiT
	NewConvoPopup NewConvoPopUpT
	CurrentConvo protocol.SizeT
	Usernames map[protocol.SizeT]string
}


func (c *ClientT)SendMessage(payload string,convoId protocol.SizeT){
	send_packet := protocol.Packet{
		Type:protocol.PacketSend,
		Payload:protocol.SendT{
			SenderId:c.UserState.UserID,
			ConvoId:convoId,
			Payload:[]byte(payload),
			Token: c.UserState.Token,
			Time:time.Now().Format(time.DateTime),
		},
	}
	protocol.SendPacket(send_packet,c.ServerConn)
}

type NewConvoPopUpT struct{
	PopUp *widget.PopUp
	UserSearch *widget.SelectEntry
	Cancelbutton *widget.Button
	CreateButton *widget.Button
	SelectedUsers *widget.Entry
	ConvoName *widget.Entry
 	MainCont *fyne.Container
	Showing bool
	SelectedUserIds []protocol.SizeT
	SelectedUsernames []string
}

func (popup *NewConvoPopUpT)Init(c *ClientT){
	convo_name_default_text := "Enter Convo Name, Clear to have server auto generate name."
	popup.UserSearch = widget.NewSelectEntry(nil)
	popup.UserSearch.OnSubmitted = func(pattern string){
		search_packet := protocol.Packet{
			Type:protocol.PacketSearchForUser,
			Payload:protocol.SearchUserT{
				SenderId:c.UserState.UserID,
				SearchStr:pattern + "%",
				AuthToken:c.UserState.Token,
			},
		}
		//reader thread will update the PopUp options
		protocol.SendPacket(search_packet,c.ServerConn)
	}
	popup.ConvoName = widget.NewEntry()
	popup.ConvoName.Text = convo_name_default_text
	popup.UserSearch.OnChanged = func(selected string){
		if(!strings.Contains(selected,":")){
			return
		}
		split_str := strings.Split(selected,":")
		popup.SelectedUsernames = append(popup.SelectedUsernames,split_str[0])
		id,_ := strconv.Atoi(split_str[1])
		popup.SelectedUserIds = append(popup.SelectedUserIds,protocol.SizeT(id))
		popup.SelectedUsers.Text += selected
		popup.SelectedUsers.Refresh()
	}
	popup.SelectedUsers = widget.NewEntry()
	entry_boxes := container.NewVBox(popup.UserSearch,popup.SelectedUsers,popup.ConvoName)
	popup.CreateButton = widget.NewButton("Create Convo",func(){
		popup.SelectedUserIds = append(popup.SelectedUserIds,c.UserState.UserID)
		convo_packet := protocol.Packet{
			Type:protocol.PacketNewConvo,
			Payload:protocol.NewConvoT{
				OwnerId: c.UserState.UserID,
				UserIds:popup.SelectedUserIds,
				AuthToken:c.UserState.Token,
				ConvoName:popup.ConvoName.Text,
			},
		}
		protocol.SendPacket(convo_packet,c.ServerConn)
		popup.SelectedUsernames = nil
		popup.SelectedUserIds = nil
		popup.SelectedUsers.Text = ""
		popup.ConvoName.Text = convo_name_default_text
		popup.PopUp.Hide()
	})
	popup.Cancelbutton = widget.NewButton("Cancel",func(){
		popup.SelectedUsernames = nil
		popup.SelectedUserIds = nil
		popup.PopUp.Hide()
	})
	boxes_cont := container.NewGridWithRows(1,popup.CreateButton,popup.Cancelbutton)
	popup.MainCont = container.NewBorder(nil,boxes_cont,nil,nil,entry_boxes)
	popup.PopUp  = widget.NewPopUp(popup.MainCont,c.ClientWindow.Canvas())
	popup.PopUp.Hide()
}

type MessagingUiT struct{
	MainContainer *fyne.Container
	MsgText *widget.RichText
	ConvoList *widget.List
	SendEntry *widget.Entry
	SendBtn *widget.Button
}

func (ui *MessagingUiT)Init(client *ClientT){
	ui.ConvoList = widget.NewList(
		func() int{
			return len(client.UserState.Convos)
		},
		func() fyne.CanvasObject{
			label := widget.NewLabel("temp")
            label.Wrapping = fyne.TextWrapBreak
            return label
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(client.UserState.Convos[i].Name)
		},

	)
	ui.ConvoList.OnSelected = func(id widget.ListItemID){
		client.CurrentConvo = protocol.SizeT(id) + 1
		sync_resq := protocol.Packet{
			Type:protocol.PacketSync,
			Payload:protocol.SyncT{
				ConvoId:client.CurrentConvo,
				UserId:client.UserState.UserID,
				MsgId:0, //TODO -- implement data base for Local storage of messages
				AuthToken:client.UserState.Token,
			},
		}
		protocol.SendPacket(sync_resq,client.ServerConn)
	}

	ui.MsgText = widget.NewRichTextWithText("")
	ui.MsgText.Wrapping = fyne.TextWrapWord
	ui.SendEntry = widget.NewMultiLineEntry()
	ui.SendEntry.Wrapping = fyne.TextWrapBreak
    ui.SendEntry.SetMinRowsVisible(1)
	ui.SendEntry.OnChanged = func(string){
		fmt.Println(ui.SendEntry.CursorRow)
		if(ui.SendEntry.CursorRow >= 0 && ui.SendEntry.CursorRow <= 8){
			ui.SendEntry.SetMinRowsVisible(ui.SendEntry.CursorRow + 1)
		}
	}
	ui.SendBtn = widget.NewButton(
		"Send",
		func(){
		client.SendMessage(ui.SendEntry.Text,client.CurrentConvo)
		},
	)
	send_cont := container.NewBorder(nil,nil,nil,container.NewCenter(ui.SendBtn),ui.SendEntry)
	chat_cont := container.NewBorder(nil,send_cont,nil,nil,container.NewScroll(ui.MsgText))
	hsplit := container.NewHSplit(ui.ConvoList,chat_cont)
	new_convo_btn := widget.NewButton("New Convo",func(){
		client.NewConvoPopup.Showing = true
		client.NewConvoPopup.PopUp.Show()
	})
	util_bar := container.NewHBox(new_convo_btn)
	ui.MainContainer = container.NewBorder(util_bar,nil,nil,nil,hsplit)
}

type LoginFormT struct{
	Form widget.Form
	Container *fyne.Container
}
// refrence to client needed for submit button function
// That function is where the client actually logs in
func (form *LoginFormT)Init(client *ClientT){
	username := widget.NewEntry()
	password := widget.NewPasswordEntry()
	form.Form = widget.Form{
		Items: []*widget.FormItem{
			{Text:"Username",Widget:username},
			{Text:"Password",Widget:password},
		},
		OnSubmit:func(){
			login_packet := protocol.Packet{
				Type:protocol.PacketLogin,
				Payload:protocol.LoginT{
					Username:username.Text,
					Password:password.Text,
				},
			}
			protocol.SendPacket(login_packet,client.ServerConn)
			recv_packet,err := protocol.ReadPacket(client.ServerConn)
			if(err != nil){
				dialog.ShowError(err,client.ClientWindow)
				return
			}
			if(recv_packet.Type != protocol.PacketLoginOk){
				if(recv_packet.Type == protocol.PacketError){
					err_packet := recv_packet.Payload.(protocol.ErrPacketT)
					dialog.ShowInformation("Login Failure!",err_packet.Msg,client.ClientWindow)
				}else{
					dialog.ShowInformation(
						"Unexpected Packet!",
						fmt.Sprint("Recevied packet with type: '%s', expected LoginOKT",
							protocol.PacketTypeStr[recv_packet.Type]),
						client.ClientWindow,
					)
				}
				return
			}
			go client.ReadLoop()
			client.UserState = recv_packet.Payload.(protocol.LoginOkT)
			client.Usernames = make(map[protocol.SizeT]string)
			for _,convo := range client.UserState.Convos{
				for _,user := range convo.Users{
					client.Usernames[user.UserID] = user.Username
				}
			}
			client.MessagingUi.Init(client)
			client.ClientWindow.SetContent(client.MessagingUi.MainContainer)
			client.ClientWindow.Resize(fyne.NewSize(640,480))
			client.ClientWindow.SetTitle(fmt.Sprintf("geoclient %s on server %s",
				username.Text,
				client.ServerConn.RemoteAddr().String(),
			))
		},
	}
	form.Container = container.NewBorder(nil,nil,nil,nil,&form.Form)
}



func (c *ClientT)handle_sync_resp(packet *protocol.SyncRespT){
	builder := strings.Builder{}
	for _,msg := range packet.Messages{
		name,ok := c.Usernames[msg.SenderID]
		if(!ok){
			name = fmt.Sprint("UserId: %d",msg.SenderID)
		}
		frmt_str := fmt.Sprintf("%s:\n\n%s\n\n%s\n",name,string(msg.MsgPayload),msg.Time)
		builder.WriteString(frmt_str)
	}
	fyne.Do(func(){
		c.MessagingUi.MsgText.Segments[0].(*widget.TextSegment).Text = builder.String()
		c.MessagingUi.MsgText.Refresh()
	})
}

func (c *ClientT)handle_search(packet *protocol.SearchUserRespT){
	if(!c.NewConvoPopup.Showing){
		return
	}
	user_options := make([]string,0,len(packet.Ids))
	for i := range len(packet.Ids){
		user_options = append(user_options,fmt.Sprintf("%s:%d",packet.Usernames[i],packet.Ids[i]))
	}
	fyne.Do(func(){
		c.NewConvoPopup.UserSearch.SetOptions(user_options)
	})
}

func (client *ClientT)ReadLoop(){
	for{
		recv_packet,err := protocol.ReadPacket(client.ServerConn)
		if(err != nil){
			log.Print(err)
		}
		switch(recv_packet.Type){
			case protocol.PacketError:
			{
				err_packet := recv_packet.Payload.(protocol.ErrPacketT)
				dialog.ShowInformation("Server Error!",err_packet.Msg,client.ClientWindow)
			}
			case protocol.PacketSyncResp:
			{
				sync_packet := recv_packet.Payload.(protocol.SyncRespT)
				// TODO -- When local Database is implemented
				if(sync_packet.ConvoId != client.CurrentConvo){
					continue
				}
				client.handle_sync_resp(&sync_packet)
			}
			case protocol.PacketSearchForUserResp:
			{
				resp_packet := recv_packet.Payload.(protocol.SearchUserRespT)
				client.handle_search(&resp_packet)
			}
			case protocol.PacketNewConvoResp:
			{
				resp_packet := recv_packet.Payload.(protocol.NewConvoRespT)
				client.UserState.Convos = append(client.UserState.Convos,resp_packet.ConvoData)
			}
			case protocol.PacketNewMessage:
			{
				new_msg := recv_packet.Payload.(protocol.NewMessageT)
				sync_req := protocol.Packet{
					Type:protocol.PacketSync,
					Payload:protocol.SyncT{
						ConvoId:new_msg.ConvoId,
						UserId:client.UserState.UserID,
						MsgId:0,
						AuthToken:client.UserState.Token,
					},
				}
				protocol.SendPacket(sync_req,client.ServerConn)
			}
		}
	}
}

func (c *ClientT)handle_server_state(state *protocol.ConnectEnstabT){
	if(state.TlsEnabled){
		cert_file,err := os.Open(".geogory.space.crt")
		cert_data := []byte{}
		defer cert_file.Close()
		if(err != nil){
			if(errors.Is(err,os.ErrNotExist)){
				cert_file,err = os.Create(".geogory.space.crt")
				cert_file.Write(state.TlsCert)
				cert_data = state.TlsCert
			}
		}else{
			cert_data,_ = io.ReadAll(cert_file)
		}
		cert_pool,_ := x509.SystemCertPool()
		parsed_ca,err := x509.ParseCertificate(cert_data)
		cert_pool.AddCert(parsed_ca)
		conf := tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: "geogory.space",
			RootCAs: cert_pool,
		}
		tls_conn := tls.Client(c.ServerConn,&conf)
		err = tls_conn.Handshake()
		if(err != nil){
			log.Panic(err)
		}
		c.ServerConn = tls_conn
	}
}

func CreateClient()(*ClientT){
	client := new(ClientT)
	var err error
	client.ServerConn,err = net.Dial("tcp","geogory.space:2020")
	if(err != nil){
		log.Println(err)
	}
	server_state_packet,err := protocol.ReadPacket(client.ServerConn)
	server_state := server_state_packet.Payload.(protocol.ConnectEnstabT)
	client.handle_server_state(&server_state)
	client.ClientApp = app.New()
	client.ClientWindow = client.ClientApp.NewWindow("geoclient")
	client.login_form.Init(client)
	client.ClientWindow.SetOnClosed(func(){
		close_conn_packet := protocol.Packet{
			Type:protocol.PacketCloseConn,
			Payload:protocol.CloseConnT{
				Ok:"GoodBye!",
			},
		}
		protocol.SendPacket(close_conn_packet,client.ServerConn)
	})
	client.ClientWindow.SetContent(client.login_form.Container)
	client.NewConvoPopup.Init(client)
	return client
}
func (client *ClientT)ClientMain(){
	client.ClientWindow.ShowAndRun()
}
