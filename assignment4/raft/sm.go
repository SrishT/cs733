package raft

import (
	"fmt"
	"errors"
	"github.com/cs733-iitb/log"
)

const LEADER = 3
const CANDIDATE = 2
const FOLLOWER = 1

type Send struct {
	id    int
	event interface{}
}

type LogStore struct {
	index int64
	term  int
	data  []byte
}

type SaveState struct {
	curTerm  int
	votedFor int
	commitIndex int64
}

type LogEntry struct {
	Term int
	Data []byte
}

type SM struct {
	id          int
	lid         int
	peers       []int
	status      int
	curTerm     int
	votedFor    int
	majority    []int
	commitIndex int64
	lg	    *log.Log
	logTerm     int
	logIndex    int64
	matchIndex  []int64
	nextIndex   []int64
	electionTimeout int
	heartbeatTimeout int
}

type Commit struct {
	id    int
	index int64
	term  int
	data  []byte
	err   error
}

type VoteReqEv struct {
	candidateId int
	term        int
	logIndex    int64
	logTerm     int
}

type VoteRespEv struct {
	id   int
	term int
	vote bool
}

type Timeout struct {
}

type Alarm struct {
	flag 	int
	timeout int
}

type AppendEv struct {
	data []byte
	flag int
}

type AppendEntriesReqEv struct {
	term         int
	logterm      int
	lid          int
	prevLogIndex int64
	prevLogTerm  int
	flag	     int
	data	     []byte
	leaderCommit int64
}

type AppendEntriesRespEv struct {
	id      int
	index   int64
	term    int
	data	[]byte
	success bool
}

func (s *SM) resetTerm(term int) {
	s.status = FOLLOWER
	s.curTerm = term
	s.votedFor = 0
	s.resetMajority()
}

func (s *SM) resetMajority() {
	for i := 0; i < len(s.majority); i++ {
		s.majority[i] = 0
	}
}

func (s *SM) resetNextIndex() {
	for i := 0; i < len(s.nextIndex); i++ {
		s.nextIndex[i] = s.logIndex + 1
	}
}

func (s *SM) countMajority(index int64) int {
	vr := 0	
	for i := 0; i < len(s.peers); i++ {
		if index == -1 {
			if s.majority[i] == 1 {
				vr++
			}
		} else {
			if s.matchIndex[i] >= index {
				vr++
			}
		}
	}
	return vr
}

func (s *SM) broadCast(flag int, data []byte,actions []interface{}) []interface{} {
	var cmd interface{}
	for i := 0; i < len(s.peers); i++ {
		if s.peers[i] != 0 {
			if flag == 0 {
				cmd = AppendEntriesReqEv{s.curTerm,s.curTerm,s.id,s.logIndex,s.logTerm,flag,[]byte(""),s.commitIndex}
			} else if flag == 1 {
				cmd = AppendEntriesReqEv{s.curTerm,s.curTerm,s.id,s.logIndex,s.logTerm,flag,data,s.commitIndex}
				s.nextIndex[s.peers[i]-1]++
			} else if flag == 2 {
				cmd = VoteReqEv{s.id,s.curTerm,s.logIndex,s.logTerm}	
			}
			actions = append(actions, Send{s.peers[i],cmd})
		}
	}
	return actions
}

func (s *SM) ProcessEvent(ev interface{}) []interface{} {
	actions := make([]interface{},10)
	switch ev.(type) {
	case AppendEv:
		cmd := ev.(AppendEv)
		actions := s.appnd(cmd)
		return actions
	case AppendEntriesReqEv:
		cmd := ev.(AppendEntriesReqEv)
		actions := s.appendEntriesReq(cmd)
		return actions
	case AppendEntriesRespEv:
		cmd := ev.(AppendEntriesRespEv)
		actions := s.appendEntriesResp(cmd)
		return actions
	case Timeout:
		actions := s.timeout()
		return actions
	case VoteReqEv:
		cmd := ev.(VoteReqEv)
		actions := s.voteReq(cmd)
		return actions
	case VoteRespEv:
		cmd := ev.(VoteRespEv)
		actions := s.voteResp(cmd)
		return actions
	default:
		panic("Unrecognized")
	}
	return actions
}

func (s *SM) appnd(e interface{}) []interface{} {
	actions := []interface{}{}
	ev := e.(AppendEv)
	le := LogEntry{}
	switch s.status { 
	case FOLLOWER:
		actions = append(actions, Send{s.lid,AppendEv{ev.data,ev.flag}})
	case CANDIDATE:
		var err error
		err = errors.New("No Leader")
		actions = append(actions, Commit{s.id,-1,0,ev.data,err})
	case LEADER:
		actions = append(actions, LogStore{s.logIndex+1,s.curTerm,ev.data})
		le.Data = ev.data
		le.Term = s.curTerm
		s.lg.Append(le)
		s.matchIndex[s.id-1] = s.logIndex + 1
		actions = s.broadCast(1,ev.data,actions)
		s.logIndex++
		s.logTerm = s.curTerm
	}
	return actions
}

func (s *SM) appendEntriesReq(e interface{}) []interface{} {
	actions := []interface{}{}
	ev := e.(AppendEntriesReqEv)
	var cmd interface{}
	le := LogEntry{}
	var lt int
	switch s.status {
	case FOLLOWER:
		if ev.term < s.curTerm {
			cmd = AppendEntriesRespEv{s.id,s.logIndex,s.curTerm,ev.data,false}
			actions = append(actions, Send{ev.lid,cmd})
		} else if ev.flag == 1 {
			s.lid = ev.lid
			s.curTerm = ev.term
			actions = append(actions,Alarm{1,s.electionTimeout})
			if s.logIndex >= ev.prevLogIndex {
				if ev.prevLogIndex >= 0 {
					t,err:= s.lg.Get(ev.prevLogIndex)
					if err!=nil {
						fmt.Println("error :",ev.prevLogIndex," ",s.logIndex," at ",s.id," lid ",ev.lid)
					}
					lt = t.(LogEntry).Term
				} else {
					lt = 0
				}
				if lt == ev.prevLogTerm {
					if s.commitIndex > ev.prevLogIndex {
						cmd = AppendEntriesRespEv{s.id, s.logIndex, s.curTerm, ev.data, true}
						actions = append(actions, Send{ev.lid,cmd})
					} else {
						if s.logIndex > ev.prevLogIndex {
							s.lg.TruncateToEnd(ev.prevLogIndex+1)
							s.logIndex = ev.prevLogIndex
						}
						cmd = AppendEntriesRespEv{s.id, s.logIndex+1, s.curTerm, ev.data, true}
						actions = append(actions, Send{ev.lid,cmd})
						actions = append(actions, LogStore{ev.prevLogIndex+1, ev.term, ev.data})
						s.logIndex++
						s.logTerm = ev.logterm
						le.Term = ev.logterm
						le.Data = ev.data
						e := s.lg.Append(le)
						if e!=nil {
							panic(e)
						}
					}
					if s.commitIndex < ev.leaderCommit {
						ci:=s.commitIndex
						if s.logIndex < ev.leaderCommit {
							s.commitIndex = s.logIndex
						} else {
							s.commitIndex = ev.leaderCommit
						}
						for i:= ci; i<s.commitIndex;i++ {
							i++
							t,_:= s.lg.Get(i)
							actions = append(actions,Commit{s.id,i,t.(LogEntry).Term,t.(LogEntry).Data,nil})
						}
						actions = append(actions,SaveState{s.curTerm, s.votedFor, s.commitIndex})
					}				
				} else {
					cmd = AppendEntriesRespEv{s.id,s.logIndex,s.curTerm,ev.data,false}
					actions = append(actions,Send{ev.lid,cmd})
				}
			} else {
				cmd = AppendEntriesRespEv{s.id,s.logIndex,s.curTerm,ev.data,false}
				actions = append(actions,Send{ev.lid,cmd})
			}
		} else {
			actions = append(actions,Alarm{1,s.electionTimeout})
			if s.logIndex >= ev.prevLogIndex {
				if ev.prevLogIndex >= 0 {
					t,err:= s.lg.Get(ev.prevLogIndex)
					if err!=nil {
						fmt.Println("error :",ev.prevLogIndex," ",s.logIndex)
					}
					lt = t.(LogEntry).Term
				} else {
					lt = 0
				}
				if lt == ev.prevLogTerm {
					if s.commitIndex < ev.leaderCommit {
						ci:=s.commitIndex
						if s.logIndex < ev.leaderCommit {
							s.commitIndex = s.logIndex
						} else {
							s.commitIndex = ev.leaderCommit
						}
						for i:= ci; i<s.commitIndex;i++ {
							i++
							t,_:= s.lg.Get(i)
							actions = append(actions,Commit{s.id,i,t.(LogEntry).Term,t.(LogEntry).Data,nil})
						}
						actions = append(actions,SaveState{s.curTerm, s.votedFor, s.commitIndex})
					}
				}
			}
		}
	case CANDIDATE:
		if s.curTerm < ev.term {
			s.resetTerm(ev.term)
			actions = append(actions,Alarm{1,s.electionTimeout})
			actions = append(actions,SaveState{s.curTerm, s.votedFor, s.commitIndex})
			cmd = AppendEntriesReqEv{ev.term,ev.logterm,ev.lid,ev.prevLogIndex,ev.prevLogTerm,ev.flag,ev.data,ev.leaderCommit}
			actions = append(actions, Send{s.id,cmd})
		} else {
			cmd = Send{ev.lid, AppendEntriesRespEv{s.id, s.logIndex, s.curTerm, ev.data, false}}
			actions = append(actions, cmd)
		} 
	case LEADER:
		if s.curTerm < ev.term {
			s.resetTerm(ev.term)
			actions = append(actions,Alarm{1,s.electionTimeout})
			actions = append(actions,SaveState{s.curTerm, s.votedFor, s.commitIndex})
			cmd = AppendEntriesReqEv{ev.term,ev.logterm,ev.lid,ev.prevLogIndex,ev.prevLogTerm,ev.flag,ev.data,ev.leaderCommit}
			actions = append(actions, Send{s.id,cmd})
		} else {
			cmd = AppendEntriesRespEv{s.id, s.logIndex, s.curTerm, ev.data, false}
			actions = append(actions, Send{ev.lid,cmd})
		}

	}
	return actions
}

func (s *SM) appendEntriesResp(e interface{}) []interface{} {
	actions := []interface{}{}
	ev := e.(AppendEntriesRespEv)
	var cmd interface{}
	switch s.status {
	case FOLLOWER:
		if s.curTerm < ev.term {
			s.resetTerm(ev.term)
			actions = append(actions, SaveState{s.curTerm, s.votedFor, s.commitIndex})
		}
	case CANDIDATE:
		if s.curTerm < ev.term {
			s.resetTerm(ev.term)
			actions = append(actions, Alarm{1,s.electionTimeout})
			actions = append(actions, SaveState{s.curTerm, s.votedFor, s.commitIndex})
		}
	case LEADER:
		if s.curTerm < ev.term {
			s.resetTerm(ev.term)
			actions = append(actions, Alarm{1,s.electionTimeout})
			actions = append(actions, SaveState{s.curTerm, s.votedFor, s.commitIndex})
		} else {
			if ev.success == true && ev.term == s.curTerm {
				if s.matchIndex[ev.id-1] < ev.index {
					s.matchIndex[ev.id-1] = ev.index
				}
				if s.nextIndex[ev.id-1] <= s.logIndex {
					d,_ := s.lg.Get(s.nextIndex[ev.id-1])
					p,_ := s.lg.Get(s.nextIndex[ev.id-1]-1)
					cmd = AppendEntriesReqEv{s.curTerm,d.(LogEntry).Term,s.id,s.nextIndex[ev.id-1]-1,p.(LogEntry).Term,1, d.(LogEntry).Data, s.commitIndex}
					actions = append(actions, Send{ev.id,cmd})
					s.nextIndex[ev.id-1]++
				} else {
				}
				votes := s.countMajority(ev.index)
				if votes >= (len(s.peers)/2)+1 && ev.index > s.commitIndex {
					s.commitIndex++
					actions = append(actions, Commit{ev.id,ev.index,ev.term,ev.data,nil})
					actions = append(actions, SaveState{s.curTerm,s.votedFor,s.commitIndex})
				}
			} else if ev.success == false && ev.index > 0 {
				s.nextIndex[ev.id-1] = s.nextIndex[ev.id-1]-2
				d,_ := s.lg.Get(s.nextIndex[ev.id-1])
				p,_ := s.lg.Get(s.nextIndex[ev.id-1]-1)
				cmd = AppendEntriesReqEv{s.curTerm,d.(LogEntry).Term,s.id,s.nextIndex[ev.id-1]-1,p.(LogEntry).Term,1,d.(LogEntry).Data, s.commitIndex}
				s.nextIndex[ev.id-1]++
				actions = append(actions, Send{ev.id,cmd})
			}
		}
	}
	return actions
}
func (s *SM) voteReq(e interface{}) []interface{} {
	actions := []interface{}{}
	ev := e.(VoteReqEv)
	switch s.status {
	case FOLLOWER:
		if (s.curTerm < ev.term) || (s.curTerm == ev.term && s.lid == -1) {
			if s.votedFor == 0 || s.votedFor == ev.candidateId || s.curTerm < ev.term {
				actions = append(actions,Alarm{1,s.electionTimeout})
				s.resetTerm(ev.term)
				if s.logTerm < ev.logTerm {
					s.votedFor = ev.candidateId
					actions = append(actions,Send{ev.candidateId, VoteRespEv{s.id,s.curTerm,true}})
				} else if s.logTerm == ev.logTerm && s.logIndex <= ev.logIndex {
					s.votedFor = ev.candidateId
					actions = append(actions,Send{ev.candidateId, VoteRespEv{s.id,s.curTerm,true}})
				} else {
					actions = append(actions,Send{ev.candidateId, VoteRespEv{s.id,s.curTerm,false}})
				}
				actions = append(actions, SaveState{s.curTerm,s.votedFor,s.commitIndex})
			} else {
				actions = append(actions, Send{ev.candidateId,VoteRespEv{s.id,s.curTerm,false}})
			}
		} else {
			actions = append(actions, Send{ev.candidateId,VoteRespEv{s.id,s.curTerm,false}})
		}
	case CANDIDATE:
		if s.curTerm <= ev.term {
			s.resetTerm(ev.term)
			s.votedFor = ev.candidateId
			actions = append(actions, Alarm{1,s.electionTimeout})
			actions = append(actions, Send{ev.candidateId, VoteRespEv{s.id,ev.term,true}})
			actions = append(actions, SaveState{s.curTerm,s.votedFor,s.commitIndex})
		} else {
			actions = append(actions, Send{ev.candidateId, VoteRespEv{s.id,s.curTerm,false}})
		}
	case LEADER:
		if s.curTerm < ev.term {
			s.resetTerm(ev.term)
			s.votedFor = ev.candidateId
			actions = append(actions, Alarm{1,s.electionTimeout})
			actions = append(actions, Send{ev.candidateId, VoteRespEv{s.id,ev.term,true}})
			actions = append(actions, SaveState{s.curTerm,s.votedFor,s.commitIndex})
		} else {
			actions = s.broadCast(0,[]byte(""),actions)
			actions = append(actions, Alarm{0,s.heartbeatTimeout})
		}

	}
	return actions
}

func (s *SM) voteResp(e interface{}) []interface{} {
	actions := []interface{}{}
	ev := e.(VoteRespEv)
	switch s.status {
	case FOLLOWER:
		if s.curTerm < ev.term {
			s.resetTerm(ev.term)
			actions = append(actions, SaveState{s.curTerm,s.votedFor,s.commitIndex})
		}
	case CANDIDATE:
		if s.curTerm < ev.term {
			s.resetTerm(ev.term)
			actions = append(actions, Alarm{1,s.electionTimeout})
			actions = append(actions, SaveState{s.curTerm,s.votedFor,s.commitIndex})
		} else {
			if ev.vote == true && ev.term == s.curTerm {
				s.majority[ev.id-1] = 1
			}
			votes := s.countMajority(-1)
			if votes >= (len(s.peers)/2)+1 {
				s.status = LEADER
				s.lid = s.id
				actions = append(actions, Alarm{0,s.heartbeatTimeout})
				s.resetMajority()
				s.resetNextIndex()
				actions = s.broadCast(0,[]byte(""),actions)
			}
		}
	case LEADER:
		if s.curTerm < ev.term {
			s.resetTerm(ev.term)
			actions = append(actions, Alarm{1,s.electionTimeout})
			actions = append(actions, SaveState{s.curTerm,s.votedFor,s.commitIndex})
		}

	}
	return actions
}

func (s *SM) timeout() []interface{} {
	actions := []interface{}{}
	switch s.status {
	case FOLLOWER:
		s.curTerm++
		s.status = CANDIDATE
		s.resetMajority()
		s.votedFor = s.id
		s.majority[s.id-1] = 1
		s.lid = -1
		actions = s.broadCast(2,[]byte(""),actions)
		actions = append(actions, Alarm{1,s.electionTimeout})
	case CANDIDATE:
		s.votedFor = s.id
		s.resetMajority()
		s.majority[s.id-1] = 1
		actions = s.broadCast(2,[]byte(""),actions)
		actions = append(actions, Alarm{1,s.electionTimeout})
		actions = append(actions, SaveState{s.curTerm, s.votedFor, s.commitIndex})
	case LEADER:
		actions = s.broadCast(0,[]byte(""),actions)
		actions = append(actions, Alarm{1,s.heartbeatTimeout})
	}
	return actions
}
