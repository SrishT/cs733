package raft

import (
	"fmt"
	"time"
	//"errors"
	//"reflect"
	"strconv"
	"math/rand"
	"io/ioutil"
	"encoding/json"
	"github.com/cs733-iitb/log"
	"github.com/cs733-iitb/cluster"
	"github.com/cs733-iitb/cluster/mock"
	//"github.com/SrishT/cs733/assignment3"
	//"github.com/SrishT/cs733/assignment3/sm_rn"
)

const val = 5
var leaderId int

type Message struct {
	Term int  //for saveState
	VotedFor int
	CommitIndex int64
}

type CommitInfo struct {
	Data  []byte
	Index int64
	Err   error
}

type ConfigRN struct {
	cluster 	 []*NetConfig // Information about all servers, including this
	Id 		 int // this node's id. One of the cluster's entries should match
	ElectionTimeout  int
	HeartbeatTimeout int
}

type NetConfig struct {
	Id   int
	Host string
	Port int
}

type RaftNode struct { // implements Node interface
	id int
	lid int
	sm *SM
	LogDir string
	StateDir string
	timeoutCh *time.Timer
	server *mock.MockServer
	commitChannel chan *CommitInfo
}

func NewRN(Id int, config ConfigRN) (raftNode *RaftNode) {
	//rand.Seed(time.Now().UTC().UnixNano()*int64(Id))
	cc := make(chan *CommitInfo)
	t := time.NewTimer(time.Duration(config.ElectionTimeout)*time.Millisecond)
	rn := &RaftNode{id:Id, lid:-1, timeoutCh:t, commitChannel:cc, StateDir:"StateStore"+strconv.Itoa(Id)+".json", LogDir:"LogDir"+strconv.Itoa(Id)}
	return rn
}

func (rn *RaftNode) Id() int {
	return rn.id
}

func LeaderId(rn []*RaftNode) int {
	lid := -1
	n := 0
	//------------to check error checking number of leaders
	for i:=0;i<len(rn);i++ {
		if rn[i].sm.status == 3 {
			n++
			lid = rn[i].sm.id
		}
	}
	fmt.Println("Number of leaders : ",n)
	return lid
}
/*
func (rn *RaftNode) Get(index int) (error, []byte) {
	var err error
	lg, err := log.Open(rn.LogDir)
	if err==nil {
		if index < rn.sm.logIndex && index >= 0 {
			val, e := lg.Get(int64(index))
			return e, []byte(val)
		}else{
			err = errors.New("Not a valid index")
		}
	}
	return err,[]byte("")
}
*/
func (rn *RaftNode) CommittedIndex() int64 {
	return rn.sm.commitIndex
}

func (rn *RaftNode) Append(data interface{}) {
	b,_ := json.Marshal(data)
	actions := rn.sm.ProcessEvent(AppendEv{b,1})
	rn.doActions(actions)
}

func (rn *RaftNode) CommitChannel() chan *CommitInfo {
	return rn.commitChannel
}

func (rn *RaftNode) doActions(actions []interface{}) {
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Alarm: cmd := actions[i].(Alarm)
					//fmt.Println(reflect.TypeOf(cmd).Name())
					_=rn.timeoutCh.Reset(time.Duration(cmd.timeout+rand.Intn(val))*time.Millisecond)
				case Send: 
					cmd := actions[i].(Send)
					switch cmd.event.(type) {
						case VoteReqEv : c := cmd.event.(VoteReqEv)
							//fmt.Println("VoteReqEv by ",rn.id," term ",rn.sm.curTerm)
							evn := VoteReqEv{c.candidateId, c.term, c.logIndex, c.logTerm}
							rn.server.Outbox() <- &cluster.Envelope{Pid: cmd.id, Msg: evn}
						case VoteRespEv : c := cmd.event.(VoteRespEv)
							//fmt.Println("VoteRespEv by ",rn.id," term ",rn.sm.curTerm)
							evn := VoteRespEv{c.id, c.term, c.vote}
							rn.server.Outbox() <- &cluster.Envelope{Pid: cmd.id, Msg: evn}
						case AppendEntriesReqEv : c := cmd.event.(AppendEntriesReqEv)
							//fmt.Println("AppendEntriesReqEv by ",rn.id," term ",rn.sm.curTerm)
							evn := AppendEntriesReqEv{c.term, c.lid, c.prevLogIndex, c.prevLogTerm, c.fl, c.data, c.leaderCommit}
							rn.server.Outbox() <- &cluster.Envelope{Pid: cmd.id, Msg: evn}
						case AppendEntriesRespEv : c := cmd.event.(AppendEntriesRespEv)
							//fmt.Println("AppendEntriesReqEv by ",rn.id," term ",rn.sm.curTerm)
							evn := AppendEntriesRespEv{c.id,c.index,c.term,c.data,c.success}
							rn.server.Outbox() <- &cluster.Envelope{Pid: cmd.id, Msg: evn}
					}
				case Commit: cmd := actions[i].(Commit)
					//fmt.Println(reflect.TypeOf(cmd).Name())
					fmt.Println("Commit entry at: ",rn.id," by ",rn.id," term ",rn.sm.curTerm)
					rn.CommitChannel() <- &CommitInfo{Data:cmd.data, Index:cmd.index, Err:cmd.err}
				case LogStore: _ = actions[i].(LogStore)
					//fmt.Println("Log Store Event, filename: ",rn.LogDir)
					//rn.sm.log[rn.sm.logIndex+1].data = cmd.data
					//rn.sm.log[rn.sm.logIndex+1].term = rn.sm.curTerm
					//fmt.Println("Log Store at: ",rn.id," Data: ",cmd.data," by ",rn.id," term ",rn.sm.curTerm)
				case SaveState: cmd := actions[i].(SaveState)
					m := Message{cmd.curTerm,cmd.votedFor,cmd.commitIndex}
					b, err := json.Marshal(m)
					err = ioutil.WriteFile(rn.StateDir,b, 0644)
					if err!=nil {
						fmt.Println("Error writing to file")
					}
			}
		}
	}
}

func makeRafts() ([]*RaftNode) {
	
	leaderId = -1

	//makeraft
	clconfig := cluster.Config{Peers:[]cluster.PeerConfig {
		{Id:1}, {Id:2}, {Id:3},
	}}
	cl, _ := mock.NewCluster(clconfig)
	//if err != nil {return nil, err}
	//fmt.Println(reflect.TypeOf(cluster))

	//init the raft node layer
	nodes := make([]*RaftNode, len(clconfig.Peers))

	raftConfig := ConfigRN{
		ElectionTimeout: 1200,
		HeartbeatTimeout: 500,
	}

	//Create a raft node, and give the corresponding "Server" object from the
	//Cluster to help it communicate with the others.
	for i := 1; i <= 3; i++ {
		c := 0
		ct := 0
		vf := 0
		lt := 0
		li := int64(-1)
		ci := int64(-1)
		raftNode := NewRN(i, raftConfig)
		srv := cl.Servers[i]
		p := srv.Peers()
		m := make([]int,3)
		ni := make([]int64,3)
		peer := make([]int,3)
		log, err := log.Open(raftNode.LogDir)
		log.RegisterSampleEntry(LogEntries{})
		if err!=nil {
			fmt.Println("Error opening Log File")
		}
		for j:=0;j<3;j++ {
			m[j]=0
			ni[j]=0
			if j==i-1 {
				peer[j]=0
			}else {
				peer[j]=p[c]
				c++
			}
		}
		raftNode.server = srv
		file, e := ioutil.ReadFile(raftNode.StateDir)
    		if e == nil {
			var m Message
			err = json.Unmarshal(file, &m)
			ct = m.Term
			vf = m.VotedFor
			ci = m.CommitIndex
			li = log.GetLastIndex()
			t,_ := log.Get(li)
			lt = t.(LogEntries).Term
			for j:=0;j<3;j++ {
				ni[j]=li+1
			}
		}
		//** DEBUG **/
		electionTimeOut := 500
		if srv.Pid() != 1 {
			electionTimeOut = 10000
		}
		//** DEBUG **/
		raftNode.sm = &SM{id:srv.Pid(), lid:-1,peers:peer,status:1, curTerm:ct, votedFor:vf, majority:m, commitIndex:ci, lg:log, logIndex:li, logTerm:lt, nextIndex:ni, electionTimeout: electionTimeOut, heartbeatTimeout:raftConfig.HeartbeatTimeout}
		//raftNode.sm.log.RegisterSampleEntry(LogEntries{})	
		nodes[i-1] = raftNode
	}
	return nodes
}

func (rn *RaftNode) runNode() {
	s:=rn.sm
	actions:=make([]interface{},10)
	for {
		select {
		case e := <-rn.server.Inbox():
			actions = s.ProcessEvent(e.Msg)
			if s.lid!=-1 && s.status==3 {
				rn.lid = s.lid
				leaderId = s.lid
			}
			rn.doActions(actions)
		case <-rn.timeoutCh.C:
			actions = s.ProcessEvent(Timeout{})
			if s.lid!=-1 && s.status==3 {
				rn.lid = s.lid
				leaderId = s.lid
			}
			
			rn.doActions(actions)
		}
		if leaderId!= -1 && s.lid !=leaderId {
			s.lid = leaderId
			rn.lid = leaderId
		}
	}
}
