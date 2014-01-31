package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/coreos/raft"
	"github.com/gorilla/mux"
	"io/ioutil"
	"math/rand"
	"net/http"
	"path/filepath"
	"strings"
	"stripe-ctf.com/sqlcluster/command"
	"stripe-ctf.com/sqlcluster/db"
	"stripe-ctf.com/sqlcluster/log"
	"stripe-ctf.com/sqlcluster/sql"
	"stripe-ctf.com/sqlcluster/transport"
	"sync"
	"time"
)

// The raftd server is a combination of the Raft server and an HTTP
// server which acts as the transport.

type ReqBundle struct {
	query string
	resp  http.ResponseWriter
}

type Server struct {
	name       string
	listen     string
	path       string
	client     *transport.Client
	router     *mux.Router
	raftServer raft.Server
	httpServer *http.Server
	db         *db.DB
	sql        *sql.SQL
	mutex      sync.RWMutex
	buffer     chan ReqBundle
}

// Creates a new server.
func New(path string, listen string) (*Server, error) {
	sqlPath := filepath.Join(path, "storage.sql")
	log.Println("SQL path is: ", sqlPath)
	log.Println("New server created to listen on : ", listen)
	log.Println("Path is: ", path)
	s := &Server{
		listen: listen,
		path:   path,
		db:     db.New(),
		sql:    sql.NewSQL(sqlPath),
		client: transport.NewClient(),
		router: mux.NewRouter(),
	}
	s.buffer = make(chan ReqBundle, 500)
	// Read existing name or generate a new one.

	s.name = fmt.Sprintf("%07x", rand.Int())[0:7]

	return s, nil

}

// Returns the connection string.
func (s *Server) connectionString() string {
	str := transport.Decode(s.listen)
	log.Println("Requested connection string: ", str)
	return str
}

// Starts the server.
func (s *Server) ListenAndServe(leader string) error {
	var err error
	log.Printf("Initializing Raft Server: %s", s.path)
	// Initialize and start Raft server.
	transporter := raft.NewHTTPTransporter("/raft")
	transporter.Transport.Dial = transport.UnixDialer
	log.Println("Sending path into raft: ", s.path)
	s.raftServer, err = raft.NewServer(s.name, s.path, transporter, nil, s.sql, "")
	if err != nil {
		panic(err.Error())
	}
	s.raftServer.SetElectionTimeout(800 * time.Millisecond)
	//s.raftServer.SetHeartbeatTimeout(250 * time.Millisecond)
	transporter.Install(s.raftServer, s)
	s.raftServer.Start()

	if leader != "" {
		// Join to leader if specified.
		time.Sleep(5 * time.Millisecond)
		log.Println("Attempting to join leader:", leader)
		if !s.raftServer.IsLogEmpty() {
			log.Fatal("Cannot join with an existing log")
		}
		if err := s.Join(leader); err != nil {
			log.Fatal("Problem connecting to leader!!! ", err.Error())
		}

	} else if s.raftServer.IsLogEmpty() {
		// Initialize the server by joining itself.

		log.Println("Initializing new cluster")
		enc, _ := transport.Encode(s.listen)
		_, err := s.raftServer.Do(&raft.DefaultJoinCommand{
			Name:             s.raftServer.Name(),
			ConnectionString: enc,
		})
		if err != nil {
			log.Fatal(err)
		}

	} else {
		log.Println("Recovered from log")
	}
	log.Println("Initializing HTTP server")

	// Initialize and start HTTP server.
	s.httpServer = &http.Server{
		//Addr:    fmt.Sprintf(":%d", s.port),
		Handler: s.router,
	}

	s.router.HandleFunc("/join", s.joinHandler).Methods("POST")
	s.router.HandleFunc("/sql", s.sqlHandler).Methods("POST")
	s.router.HandleFunc("/sqlProxy", s.sqlProxyHandler).Methods("POST")
	l, err := transport.Listen(s.connectionString())
	if err != nil {
		log.Println(err)
	}
	go printState(s)
	return s.httpServer.Serve(l)
}

func printState(s *Server) {
	for {
		time.Sleep(5 * time.Second)

		leaderString := s.raftServer.Leader()
		log.Println("State ", " Name ", s.raftServer.Name(), s.raftServer.State(), " Term ", s.raftServer.Term(), " Leader is: ", leaderString, "Members: ", s.raftServer.MemberCount(),
			" Commit ", s.raftServer.CommitIndex())
	}
}

// This is a hack around Gorilla mux not providing the correct net/http
// HandleFunc() interface.
func (s *Server) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	s.router.HandleFunc(pattern, handler)
}

// Joins to the leader of an existing cluster.
func (s *Server) Join(leader string) error {
	enc, _ := transport.Encode(s.listen)
	command := &raft.DefaultJoinCommand{
		Name:             s.raftServer.Name(),
		ConnectionString: enc,
	}

	for {
		var b bytes.Buffer
		json.NewEncoder(&b).Encode(command)
		bReader := bytes.NewReader(b.Bytes())
		//resp, err := http.Post(fmt.Sprintf("http://%s/join", leader), "application/json", &b)
		encodedLeader, err := transport.Encode(leader)
		_ = err
		log.Println("Posting to: ", encodedLeader)
		resp, err := s.client.SafePost(encodedLeader, "/join", bReader)
		_ = resp
		if err != nil {
			log.Printf("Unable to join cluster: %s", err)
			//time.Sleep(1 * time.Millisecond)
			continue
		}
		return nil
	}
}

func (s *Server) joinHandler(w http.ResponseWriter, req *http.Request) {
	command := &raft.DefaultJoinCommand{}
	if err := json.NewDecoder(req.Body).Decode(&command); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := s.raftServer.Do(command); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) sqlHandler(w http.ResponseWriter, req *http.Request) {

	//http.Error(w, "Bah", http.StatusBadRequest)
	//return

	query, err := ioutil.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	//if state != "leader" {
	//	//http.Error(w, "Only the primary can service queries, but this is a "+state, http.StatusBadRequest)
	//	log.Println("Proxy: Initializing")
	//	resp, err := s.proxyToLeader(query)
	//	if err != nil {
	//		log.Println("Proxy: Error ", err.Error())
	//		http.Error(w, err.Error(), http.StatusBadRequest)
	//	} else {
	//		w.Write(resp)
	//	}
	//	return
	//}

	out, err := s.raftServer.Do(command.NewSqlQuery(string(query)))
	if err != nil {
		//http.Error(w, err.Error(), http.StatusBadRequest)
		//return
		if strings.Contains(err.Error(), "Not current leader") {
			for i := 0; i < 5000; i++ {
				leaderName := s.raftServer.Leader()
				if i > 0 {
					time.Sleep(1 * time.Millisecond)
				}
				if leaderName == "" {
					//http.Error(w, "Proxy: Tried to proxy but no leader!", http.StatusBadRequest)
					//http.Error(w, err.Error(), http.StatusBadRequest)
					//return
					continue
				}
				var leaderEncoded string
				if val, ok := s.raftServer.Peers()[leaderName]; ok {
					leaderEncoded = val.ConnectionString
				} else {
					//http.Error(w, err.Error(), http.StatusBadRequest)
					//return
					continue
				}

				proxyReader := bytes.NewReader(query)
				//log.Println("Proxy: SafePost starting")
				resp, err := s.client.SafePost(leaderEncoded, "/sql", proxyReader)
				if err != nil {
					//log.Println("Proxy failed: ", err.Error())
					//http.Error(w, err.Error(), http.StatusBadRequest)
					//return
					continue
				}
				respBytes, err := ioutil.ReadAll(resp)
				if err != nil {
					//http.Error(w, err.Error(), http.StatusBadRequest)
					//return
					continue
				} else {
					if i > 0 {
						log.Println("Proxy: Success on retry: ", i)
					}
					w.Write(respBytes)
					return
				}
			}
			http.Error(w, "Proxy: Too many retries!", http.StatusBadRequest)
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	output := out.(*sql.Output)
	formatted := fmt.Sprintf("SequenceNumber: %d\n%s",
		output.SequenceNumber, output.Stdout)
	w.Write([]byte(formatted))
}

func (s *Server) proxyToLeader(query []byte) ([]byte, error) {

	leaderName := s.raftServer.Leader()
	if leaderName == "" {
		//http.Error(w, "Proxy: Tried to proxy but no leader!", http.StatusBadRequest)
		return nil, errors.New("Proxy: Tried to proxy but no leader!")
	}
	var leaderEncoded string
	if val, ok := s.raftServer.Peers()[leaderName]; ok {
		leaderEncoded = val.ConnectionString
		log.Println("Proxy: Found leader at: ", leaderEncoded)
	} else {
		return nil, errors.New("Proxy: Tried to proxy but no leader!")
	}
	proxyReader := bytes.NewReader(query)
	log.Println("Proxy: SafePost starting")
	resp, err := s.client.SafePost(leaderEncoded, "/sql", proxyReader)
	if err != nil {
		log.Println("Proxy failed: ", err.Error())
		return nil, errors.New("Could not read response")
	}
	respBytes, err := ioutil.ReadAll(resp)
	if err != nil {
		return nil, errors.New("Could not read response")
	}
	return respBytes, nil
}

func (s *Server) sqlProxyHandler(w http.ResponseWriter, req *http.Request) {
	query, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Printf("Couldn't read body: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	log.Println("Proxy: Received proxy request: ", string(query))
	out, err := s.raftServer.Do(command.NewSqlQuery(string(query)))
	if err != nil {
		log.Println("Proxy: Error commiting proxy comman: ", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	output := out.(*sql.Output)
	formatted := fmt.Sprintf("SequenceNumber: %d\n%s",
		output.SequenceNumber, output.Stdout)
	log.Println("Proxy: Returning output: ", formatted)
	w.Write([]byte(formatted))
}
