package main

import (
	"os"
	"fmt"
	"net"
	"bufio"
	"strconv"
	"encoding/json"
	"github.com/cs733-iitb/cluster"
	"github.com/cs733-iitb/cluster/mock"
	"github.com/SrishT/cs733/assignment4/fs"
	"github.com/SrishT/cs733/assignment4/raft"
)

var UID = 1
var crlf = []byte{'\r', '\n'}

type CommitChan struct {
	uid int
	msg *fs.Msg
}

type ClientHandler struct {
	rn		*raft.RaftNode
	fileserv	*fs.FileServer
	address		map[int]string
	clients		map[int]chan *fs.Msg
}

func (ch *ClientHandler) ReplyChannel(uid int) chan *fs.Msg {
	return ch.clients[uid]
}

func check(obj interface{}) {
	if obj != nil {
		fmt.Println(obj)
		os.Exit(1)
	}
}

func reply(conn *net.TCPConn, msg *fs.Msg) bool {
	var err error
	write := func(data []byte) {
		if err != nil {
			return
		}
		_, err = conn.Write(data)
	}
	var resp string
	switch msg.Kind {
	case 'C': // read response
		resp = fmt.Sprintf("CONTENTS %d %d %d", msg.Version, msg.Numbytes, msg.Exptime)
	case 'O':
		resp = "OK "
		if msg.Version > 0 {
			resp += strconv.Itoa(msg.Version)
		}
	case 'F':
		resp = "ERR_FILE_NOT_FOUND"
	case 'V':
		resp = "ERR_VERSION " + strconv.Itoa(msg.Version)
	case 'M':
		resp = "ERR_CMD_ERR"
	case 'I':
		resp = "ERR_INTERNAL"
	case 'R':
		resp = "ERR_REDIRECT "+msg.Ldr
	default:
		fmt.Printf("Unknown response kind '%c'", msg.Kind)
		return false
	}
	resp += "\r\n"
	write([]byte(resp))
	if msg.Kind == 'C' {
		write(msg.Contents)
		write(crlf)
	}
	return err == nil
}

func (ch *ClientHandler) serve(conn *net.TCPConn,uid int) {
	reader := bufio.NewReader(conn)
	for {
		msg, msgerr, fatalerr := fs.GetMsg(reader)
		if fatalerr != nil || msgerr != nil {
			reply(conn, &fs.Msg{Kind: 'M'})
			conn.Close()
			break
		}
		if msgerr != nil {
			if (!reply(conn, &fs.Msg{Kind: 'M'})) {
				conn.Close()
				break
			}
		}
		data := &raft.Data{uid,msg}
		ch.rn.Append(data)
		response := <- ch.ReplyChannel(uid)
		if !reply(conn, response) {
			conn.Close()
			break
		}
	}
}

func (ch *ClientHandler) monitorRaftChan() {
	response := &fs.Msg{}
	for ch.rn.Flag == true {
		e := <- ch.rn.CommitChannel()
		var data raft.Data 
		_ = json.Unmarshal(e.Data, &data)
		if e.Err !=nil {
			response.Kind = 'R'
			response.Ldr = ch.address[ch.rn.GetLeaderId()]
		} else {
			response = ch.fileserv.ProcessMsg(data.Msg)
		}
		ch.ReplyChannel(data.Uid)<-response
	}
}

func serverMain(id int,config *raft.ConfigRN, addr map[int]string) {
	tcpaddr, err := net.ResolveTCPAddr("tcp", addr[id])
	check(err)
	tcp_acceptor, err := net.ListenTCP("tcp", tcpaddr)
	check(err)
	r := raft.MakeRaft(id,config)
	m := make(map[int]chan *fs.Msg)
	f := fs.Initialize()
	ch := &ClientHandler{rn:r,clients:m,fileserv:f,address:addr}
	go ch.monitorRaftChan()
	for ch.rn.Flag == true {
		tcp_conn, err := tcp_acceptor.AcceptTCP()
		check(err)
		uid := UID
		UID++
		replyChan := make(chan *fs.Msg,10000)
		ch.clients[uid] = replyChan
		go ch.serve(tcp_conn,uid)
	}
}

func main() {
	id,err := strconv.Atoi(os.Args[1])
	if err!=nil {
		panic(err)
	}
	clconfig := cluster.Config{Peers:[]cluster.PeerConfig {
		{Id:1}, {Id:2}, {Id:3}, {Id:4}, {Id:5},
	}}
	addr := map[int]string {1:"localhost:8081",2:"localhost:8082",3:"localhost:8083",4:"localhost:8084",5:"localhost:8085"}
	cl, _ := mock.NewCluster(clconfig)
	config := &raft.ConfigRN{Id:id,Cluster:cl,ElectionTimeout:1000,HeartbeatTimeout:500,}
	if id == 1 {
		config.ElectionTimeout = 500
	}
	serverMain(id,config,addr)
}
