package raft

import (
	"fmt"
	"time"
	"strconv"
	"io/ioutil"
	//"reflect"
	"math/rand"
	"github.com/cs733-iitb/log"
	"github.com/cs733-iitb/cluster"
	"github.com/cs733-iitb/cluster/mock"
	//"github.com/SrishT/cs733/assignment3"
	//"github.com/SrishT/cs733/assignment3/sm_rn"
)

const val = 5
var leaderId int

type CommitInfo struct {
	Data  []byte
	Index int // or int .. whatever you have in your code
	Err   error // Err can be errred
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
	rand.Seed(time.Now().UTC().UnixNano()*int64(Id))
	cc := make(chan *CommitInfo)
	t := time.NewTimer(time.Duration(config.ElectionTimeout)*time.Millisecond)
	rn := &RaftNode{id:Id, lid:-1, timeoutCh:t, commitChannel:cc, StateDir:"StateStore"+strconv.Itoa(Id), LogDir:"LogFile"+strconv.Itoa(Id)}
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

func (rn *RaftNode) Append(data []byte) {
	//fmt.Println("********************* Client Append ************************ ",data)
	//fmt.Println("********************* Client Append lid ******************** ",rn.lid," ",rn.sm.lid," ",rn.sm.id)
	//fmt.Println("********************* Client Append sm status ************** ",rn.sm.status)
	actions := rn.sm.ProcessEvent(AppendEv{data,1})
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
					//fmt.Println("Commit entry at: ",rn.id," by ",rn.id," term ",rn.sm.curTerm)
					rn.CommitChannel() <- &CommitInfo{Data:cmd.data, Index:cmd.index, Err:cmd.err}
				case LogStore: cmd := actions[i].(LogStore)
					//fmt.Println("Log Store Event, filename: ",rn.LogDir)
					rn.sm.log[rn.sm.logIndex+1].data = cmd.data
					rn.sm.log[rn.sm.logIndex+1].term = rn.sm.curTerm
					lg, err := log.Open(rn.LogDir)
					if err!=nil {
						fmt.Println("Error opening Log File")
					}else {
						lg.Append(cmd.data)
					}
					//fmt.Println("Log Store at: ",rn.id," Data: ",cmd.data," by ",rn.id," term ",rn.sm.curTerm)
				case SaveState: cmd := actions[i].(SaveState)
					temp := strconv.Itoa(cmd.curTerm)+", "+strconv.Itoa(cmd.votedFor)
					//fmt.Println("Save State Event, filename: ",rn.StateDir," temp: ",temp)
					err := ioutil.WriteFile(rn.StateDir, []byte(temp), 0644)
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

	// init the raft node layer
	nodes := make([]*RaftNode, len(clconfig.Peers))

	raftConfig := ConfigRN{
		ElectionTimeout: 1200,
		HeartbeatTimeout: 500,
	}

	// Create a raft node, and give the corresponding "Server" object from the
	// cluster to help it communicate with the others.
	for i := 1; i <= 3; i++ {
		c := 0
		raftNode := NewRN(i, raftConfig)
		srv := cl.Servers[i]
		p := srv.Peers()
		m := make([]int,3)
		ni := make([]int,3)
		peer := make([]int,3)
		lg := make([]LogEntries,100)
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
		raftNode.sm = &SM{id:srv.Pid(), lid:-1,peers:peer,status:1, curTerm:0, votedFor:0, majority:m, commitIndex:-1, log:lg, logTerm:0, logIndex:0, nextIndex:ni, electionTimeout:raftConfig.ElectionTimeout, heartbeatTimeout:raftConfig.HeartbeatTimeout}
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
