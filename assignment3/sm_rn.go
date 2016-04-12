package raft

import (
	"fmt"
	"time"
	"strconv"
	"math/rand"
	"io/ioutil"
	"encoding/json"
	"github.com/cs733-iitb/cluster"
	"github.com/cs733-iitb/cluster/mock"
)

const val = 5
var leaderId int

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
	cluster 	 []*NetConfig
	Id 		 int
	ElectionTimeout  int
	HeartbeatTimeout int
}

type NetConfig struct {
	Id   int
	Host string
	Port int
}

type RaftNode struct {
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
	// sriram - Why randomize? Esp. when you are debugging
	//rand.Seed(time.Now().UTC().UnixNano()*int64(Id))
	cc := make(chan *CommitInfo,10000)
	t := time.NewTimer(time.Duration(config.ElectionTimeout)*time.Millisecond)
	rn := &RaftNode{id:Id, lid:-1, timeoutCh:t, commitChannel:cc, StateDir:"StateStore"+strconv.Itoa(Id)+".json", LogDir:"LogDir"+strconv.Itoa(Id)}
	return rn
}

func (rn *RaftNode) Id() int {
	return rn.id
}

func LeaderId(rn []*RaftNode) int {
	lid := -1
	for i:=0;i<len(rn);i++ {
		if rn[i].sm.status == 3 {
			lid = rn[i].sm.id
		}
	}
	return lid
}

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
					_=rn.timeoutCh.Reset(time.Duration(cmd.timeout+rand.Intn(val))*time.Millisecond)
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
