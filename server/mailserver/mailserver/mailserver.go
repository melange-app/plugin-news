package main

import (
	"airdispat.ch/identity"
	"airdispat.ch/message"
	"airdispat.ch/server"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"github.com/coopernurse/gorp"
	_ "github.com/lib/pq"
	"melange/app/models"
	"melange/mailserver"
	"net"
	"os"
	"time"
)

var port = flag.String("port", "2048", "select the port on which to run the mail server")
var me = flag.String("me", getServerLocation(), "the location of the server that should be broadcast")

var key_file = flag.String("key", "", "the file to store keys")

var db_conn = flag.String("db", "", "the connection string for db.Open")

func getServerLocation() string {
	s, _ := os.Hostname()
	ips, _ := net.LookupHost(s)
	return ips[0] + ":" + *port
}

func main() {
	flag.Parse()

	db, err := sql.Open("postgres", *db_conn)
	if err != nil {
		fmt.Println("ERROR Opening DB Connection")
		fmt.Println(err)
		return
	}

	Dbm := &gorp.DbMap{
		Db:      db,
		Dialect: gorp.PostgresDialect{},
	}
	models.CreateTables(Dbm)

	handler := &melangeServer{
		Map: Dbm,
	}

	loadedKey, err := identity.LoadKeyFromFile(*key_file)
	if err != nil {
		loadedKey, err = identity.CreateIdentity()
		if err != nil {
			handler.HandleError(&server.ServerError{"Creating Mailserver Key", err})
			return
		}
		if *key_file != "" {
			err = loadedKey.SaveKeyToFile(*key_file)
			if err != nil {
				handler.HandleError(&server.ServerError{"Saving Mailserver Key", err})
				return
			}
		}
	}
	handler.LogMessage("Loaded Address", loadedKey.Address.String())

	mailserver.InitRouter()
	adServer := server.Server{
		LocationName: *me,
		Key:          loadedKey,
		Delegate:     handler,
		Router:       mailserver.LookupRouter,
	}
	StartServer(adServer, handler)
}

func StartServer(theServer server.Server, handler *melangeServer) {
	go func() {
		err := theServer.StartServer(*port)
		if err != nil {
			handler.HandleError(&server.ServerError{"Starting Server", err})
			os.Exit(1)
		}
	}()
	for {
		fmt.Print("ad> ")
		var prompt string
		fmt.Scan(&prompt)
		if prompt == "quit" {
			return
		}
	}
}

type melangeServer struct {
	Map *gorp.DbMap
	server.BasicServer
}

func (m *melangeServer) SaveMessageDescription(alert *server.MessageDescription) {
	a := models.CreateAlertFromDescription(alert)
	err := m.Map.Insert(a)
	if err != nil {
		m.HandleError(&server.ServerError{"Saving new alert to db.", err})
	}
}

func (m *melangeServer) IdentityForUser(addr *identity.Address) *identity.Identity {
	return models.IdentityFromFingerprint(addr.String(), m.Map)
}

func (m *melangeServer) RetrieveMessageForUser(name string, author *identity.Address, forAddr *identity.Address) *message.Mail {
	if name == "profile" {
		var id []*models.Identity
		_, err := m.Map.Select(&id, "select * from dispatch_identity where fingerprint = $1", author.String())
		if err != nil {
			m.HandleError(&server.ServerError{"Loading Profile Identity", err})
			return nil
		}
		if len(id) != 1 {
			// Couldn't find user
			m.HandleError(&server.ServerError{"Finding Profile User", errors.New(fmt.Sprintf("Found number of ids %v", len(id)))})
			return nil
		}
		u, err := m.Map.Get(&models.User{}, id[0].UserId)
		if err != nil {
			m.HandleError(&server.ServerError{"Getting Profile User", err})
			return nil
		}
		user, ok := u.(*models.User)
		if !ok {
			return nil
		}

		did, err := id[0].ToDispatch()
		if err != nil {
			m.HandleError(&server.ServerError{"Changing ID To Dispatch Profile", err})
			return nil
		}

		m := message.CreateMail(did.Address, forAddr, time.Now())
		m.Components.AddComponent(message.CreateComponent("airdispat.ch/profile/name", []byte(user.Name)))
		m.Components.AddComponent(message.CreateComponent("airdispat.ch/profile/avatar", []byte(user.GetAvatar())))

		return m
	}

	var results []*models.Message
	_, err := m.Map.Select(&results, "select * from dispatch_messages where name = $1 and \"from\" = $2", name, author.String())
	if err != nil {
		m.HandleError(&server.ServerError{"Finding Messages for User", err})
		return nil
	}
	if len(results) != 1 {
		m.HandleError(&server.ServerError{"Finding Messages for User", errors.New("Found wrong number of messages.")})
		return nil
	}

	out, err := results[0].ToDispatch(m.Map, forAddr.String())
	if err != nil {
		m.HandleError(&server.ServerError{"Creating Dispatch Message", err})
		return nil
	}
	return out
}

func (m *melangeServer) RetrieveMessageListForUser(since uint64, author *identity.Address, forAddr *identity.Address) *server.MessageList {
	var results []*models.Message
	_, err := m.Map.Select(&results, "select * from dispatch_messages where \"from\" = $1 and timestamp > $2 and \"to\" = ''", author.String(), since)
	if err != nil {
		m.HandleError(&server.ServerError{"Loading messages from DB", err})
		return nil
	}

	out := server.CreateMessageList(author, forAddr)

	for _, v := range results {
		desc, err := v.ToDescription(forAddr.String())
		if err != nil {
			m.HandleError(&server.ServerError{"Loading description.", err})
			return nil
		}
		desc.Location = *me
		out.AddMessageDescription(desc)
	}

	return out
}