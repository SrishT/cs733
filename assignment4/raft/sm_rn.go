package raft

import (
	"fmt"
	"time"
	"strconv"
	"math/rand"
	"io/ioutil"
	"encoding/json"
	"github.com/cs733-iitb/log"
	"github.com/cs733-iitb/cluster"
	"github.com/cs733-iitb/cluster/mock"
	"github.com/SrishT/cs733/assignment4/fs"
)

var NUMRAFTS = 5

type Data struct { //added
	Uid int
	Msg *fs.Msg
}

type Message struct {
	Term int
	VotedFor int
	CommitIndex int64
}

type CommitInfo struct {
	Data  []byte
	Index int64
	Err   error
}

type ConfigRN struct {
	Cluster		 *mock.MockCluster
	Id 		 int
	ElectionTimeout  int
	HeartbeatTimeout int
}

type RaftNode struct {
	id int
	sm *SM
	Flag bool
	LogDir string
	StateDir string
	cluster *mock.MockCluster
	timeoutCh *time.Timer
	server *mock.MockServer
	commitChannel chan *CommitInfo
}

func MakeRaft(i int,config *ConfigRN) (*RaftNode) {
	c:=0; ct:=0; vf:=0; lt:=0
	li:=int64(-1); ci:=int64(-1)
	raftNode := NewRN(i,config)
	srv := config.Cluster.Servers[i]
	p := srv.Peers()
	m := make([]int,NUMRAFTS)
	ni := make([]int64,NUMRAFTS)
	mi := make([]int64,NUMRAFTS)
	peer := make([]int,NUMRAFTS)
	log, err := log.Open(raftNode.LogDir)
	log.RegisterSampleEntry(LogEntry{})
	if err!=nil {
		fmt.Println("Error opening Log File")
	}
	for j:=0;j<NUMRAFTS;j++ {
		m[j]=0 ; ni[j]=0 ; mi[j]=-1
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
		ct = m.Term ; vf = m.VotedFor ; ci = m.CommitIndex
		li = log.GetLastIndex()
		mi[i-1] = li
		t,_ := log.Get(li)
		lt = t.(LogEntry).Term
		for j:=0;j<NUMRAFTS;j++ {
			ni[j]=li+1
		}
	}
	raftNode.sm = &SM{id:srv.Pid(), lid:-1,peers:peer,status:1, curTerm:ct, votedFor:vf, majority:m, commitIndex:ci, lg:log, logIndex:li, matchIndex:mi, logTerm:lt, nextIndex:ni, electionTimeout:config.ElectionTimeout, heartbeatTimeout:config.HeartbeatTimeout}

	go func() {
		raftNode.runNode()
	}()
	
	return raftNode
}

func NewRN(Id int, config *ConfigRN) (raftNode *RaftNode) {
	rand.Seed(time.Now().UTC().UnixNano())
	cc := make(chan *CommitInfo,10000)
	t := time.NewTimer(time.Duration(config.ElectionTimeout)*time.Millisecond)
	rn := &RaftNode{id:Id, Flag:true, timeoutCh:t, commitChannel:cc, StateDir:"StateStore"+strconv.Itoa(Id)+".json", LogDir:"LogDir"+strconv.Itoa(Id), cluster:config.Cluster}
	return rn
}

func (rn *RaftNode) Id() int {
	return rn.id
}

func (rn *RaftNode) GetLeaderId() int {
	return rn.sm.lid
}

func LeaderId(nodes []*RaftNode) int {
	for i:=0;i<len(nodes);i++ {
		if nodes[i].sm.status == 3 {
			return nodes[i].sm.id
		}
	}
	return -1
}

func (rn *RaftNode) CommittedIndex() int64 {
	return rn.sm.commitIndex
}

func (rn *RaftNode) Append(data interface{}) {
	b,_ := json.Marshal(data)
	actions := rn.sm.ProcessEvent(AppendEv{b,1})
	rn.doActions(actions)
}

func (rn *RaftNode) Shutdown() {
	rn.Flag = false
	rn.sm.status = FOLLOWER
	rn.sm.lg.Close()
	rn.server.Close()
	rn.timeoutCh.Stop()
}

func (rn *RaftNode) CommitChannel() chan *CommitInfo {
	return rn.commitChannel
}

func (rn *RaftNode) doActions(actions []interface{}) {
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Alarm: cmd := actions[i].(Alarm)
					if cmd.flag == 1 {
						_=rn.timeoutCh.Reset(time.Duration(cmd.timeout+rand.Intn(cmd.timeout))*time.Millisecond)
					} else {
						_=rn.timeoutCh.Reset(time.Duration(cmd.timeout)*time.Millisecond)
					}
				case Send: 
					cmd := actions[i].(Send)
					switch cmd.event.(type) {
						case VoteReqEv : c := cmd.event.(VoteReqEv)
							rn.server.Outbox() <- &cluster.Envelope{Pid: cmd.id, Msg: c}
						case VoteRespEv : c := cmd.event.(VoteRespEv)
							rn.server.Outbox() <- &cluster.Envelope{Pid: cmd.id, Msg: c}
						case AppendEntriesReqEv : c := cmd.event.(AppendEntriesReqEv)
							rn.server.Outbox() <- &cluster.Envelope{Pid: cmd.id, Msg: c}
						case AppendEntriesRespEv : c := cmd.event.(AppendEntriesRespEv)
							rn.server.Outbox() <- &cluster.Envelope{Pid: cmd.id, Msg: c}
					}
				case Commit: cmd := actions[i].(Commit)				
					rn.CommitChannel() <- &CommitInfo{Data:cmd.data, Index:cmd.index, Err:cmd.err}
				case LogStore: _ = actions[i].(LogStore)
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

func (rn *RaftNode) runNode() {
	actions:=make([]interface{},10)
	for rn.Flag == true {
		select {
		case e := <-rn.server.Inbox():
			actions = rn.sm.ProcessEvent(e.Msg)
			rn.doActions(actions)
		case <-rn.timeoutCh.C:
			actions = rn.sm.ProcessEvent(Timeout{})
			rn.doActions(actions)
		}
	}
}
