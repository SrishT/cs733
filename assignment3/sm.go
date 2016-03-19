package raft

import (
	//"fmt"
	"errors"
	//"reflect"
)

type Send struct {
	id    int
	event interface{}
}

type LogStore struct {
	index int
	term  int
	data  []byte
}

type SaveState struct {
	curTerm  int
	votedFor int
}

type LogEntries struct {
	term int
	data []byte
}

type SM struct {
	id          int
	lid         int
	peers       []int
	status      int
	curTerm     int
	votedFor    int
	majority    []int
	commitIndex int
	log         []LogEntries
	logTerm     int
	logIndex    int
	nextIndex   []int
	electionTimeout int
	heartbeatTimeout int
}

type Commit struct {
	id    int
	index int
	term  int
	data  []byte
	err   error
}

type VoteReqEv struct {
	candidateId int
	term        int
	logIndex    int
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
	timeout int
}

type AppendEv struct {
	data []byte
	flag int
}

type AppendEntriesReqEv struct {
	term         int
	lid          int
	prevLogIndex int
	prevLogTerm  int
	fl	     int
	data	     []byte
	leaderCommit int
}

type AppendEntriesRespEv struct {
	id      int
	index   int
	term    int
	data	[]byte
	success bool
}

func (s *SM) ProcessEvent(ev interface{}) []interface{} {
	actions := make([]interface{}, 10)
	switch ev.(type) {
	case AppendEv:
		cmd := ev.(AppendEv)
		actions := s.appnd(cmd.data, cmd.flag)
		return actions
	case AppendEntriesReqEv:
		cmd := ev.(AppendEntriesReqEv)
		actions := s.appendEntriesReq(cmd.term, cmd.lid, cmd.prevLogIndex, cmd.prevLogTerm, cmd.fl, cmd.data, cmd.leaderCommit, s.log)
		return actions
	case AppendEntriesRespEv:
		cmd := ev.(AppendEntriesRespEv)
		actions := s.appendEntriesResp(cmd.id, cmd.index, cmd.term, cmd.data, cmd.success)
		return actions
	case Timeout:
		actions := s.timeout()
		return actions
	case VoteReqEv:
		cmd := ev.(VoteReqEv)
		actions := s.voteReq(cmd.candidateId, cmd.term, cmd.logIndex, cmd.logTerm)
		return actions
	case VoteRespEv:
		cmd := ev.(VoteRespEv)
		actions := s.voteResp(cmd.id, cmd.term, cmd.vote)
		return actions
	default:
		println("Unrecognized")
	}
	return actions
}

func (s *SM) appnd(dat []byte,flag int) []interface{} {
	actions := make([]interface{}, 10)
	j := 0
	switch s.status { 
	case 1: //follower
		actions[0] = Send{s.lid, AppendEv{dat,flag}}
	case 2: //candidate
		var err error
		err = errors.New("No Leader")
		actions[0] = Commit{s.id, -1, 0, dat, err}
	case 3: //leader
		actions[j] = LogStore{index: s.logIndex + 1, term: s.curTerm, data: dat}
		j++
		s.log[s.logIndex+1].data = dat
		s.log[s.logIndex+1].term = s.curTerm
		s.majority[s.id-1] = 1
		for i := 0; i < len(s.peers); i++ {
			if s.peers[i] != 0 {
				actions[j] = Send{s.peers[i], AppendEntriesReqEv{term: s.curTerm, lid: s.id, prevLogIndex: s.logIndex, prevLogTerm: s.logTerm, fl: flag, data:dat, leaderCommit: s.commitIndex}}
				j++
			}
		}
		s.logIndex++
		s.logTerm = s.curTerm
	}
	return actions
}

func (s *SM) appendEntriesReq(trm int, lid int, prevLogInd int, prevLogTrm int, flag int, dat []byte, lC int, log []LogEntries) []interface{} {
	actions := make([]interface{}, 10)
	switch s.status {
	case 1: //follower
		if trm < s.curTerm {
			actions[0] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex, term: s.curTerm, data: dat, success: false}}
		} else {
			s.lid = lid
			actions[0] = Alarm{timeout:s.electionTimeout}
			if s.logTerm == prevLogTrm && s.logIndex == prevLogInd {
				if flag == 1 {
					actions[1] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex + 1,
						term: s.curTerm, data: dat, success: true}}
					actions[2] = LogStore{index: prevLogInd + 1, term: trm, data:dat}
					s.logIndex++
					s.logTerm = trm
					log[s.logIndex].term = trm
				}
				//commit
				if s.commitIndex < lC {
					if s.logIndex < lC {
						s.commitIndex = s.logIndex
					} else {
						s.commitIndex = lC
					}
					actions[3] = Commit{id: s.id, index: s.commitIndex, term: trm, data: dat, err:nil}
				}
			} else if s.logIndex == prevLogInd+1 && s.logTerm != trm {
				for i := s.logIndex; i < len(log); i++ {
					log[i].term = 0
					log[i].data = []byte("")
				}
			} else {
				actions[1] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex, term: s.curTerm,  data: dat, success: false}}
			}
		}
	case 2: //candidate
		if s.curTerm < trm {
			actions[0] = Alarm{timeout:s.electionTimeout}
			s.status = 1
			s.curTerm = trm
			s.votedFor = 0
			actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[2] = Send{s.id, AppendEntriesReqEv{term: trm, lid: lid, prevLogIndex: prevLogInd, prevLogTerm: prevLogTrm, fl:flag, data:dat, leaderCommit: lC}}
		} else {
			actions[0] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex, term: s.curTerm, data: dat, success: false}}
		} 
	case 3: //leader
		if s.curTerm < trm {
			actions[0] = Alarm{timeout:s.electionTimeout}
			s.status = 1
			s.curTerm = trm
			s.votedFor = 0
			actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[2] = Send{s.id, AppendEntriesReqEv{term: trm, lid: lid, prevLogIndex: prevLogInd, prevLogTerm: prevLogTrm, fl:flag , data:dat, leaderCommit: lC}}
		} else {
			actions[0] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex, term: s.curTerm, data: dat, success: false}}
		}

	}
	return actions
}

func (s *SM) appendEntriesResp(id int, index int, term int, dat []byte, success bool) []interface{} {
	actions := make([]interface{}, 10)
	switch s.status {
	case 1: //follower
		if s.curTerm < term {
			s.curTerm = term
			s.votedFor = 0
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[0] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		}
	case 2: //candidate
		if s.curTerm < term {
			s.status = 1
			s.curTerm = term
			s.votedFor = 0
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[0] = Alarm{timeout:s.electionTimeout}
			actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		}
	case 3: //leader
		if s.curTerm < term {
			actions[0] = Alarm{timeout:s.electionTimeout}
			s.status = 1
			s.curTerm = term
			s.votedFor = 0
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[0] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		} else {
			vr := 0
			if success == true && term == s.curTerm {
				s.majority[id-1] = 1
				//handle successive entries
				if index < s.logIndex {
					s.nextIndex[id-1] = s.nextIndex[id-1] + 1
					actions[0] = Send{id, AppendEntriesReqEv{term: s.log[s.nextIndex[id-1]].term, lid: s.id, prevLogIndex: s.nextIndex[id-1] - 1, prevLogTerm: s.log[s.nextIndex[id-1]-1].term, fl:1, data:[]byte(""), leaderCommit: s.commitIndex}}
				}
				for i := 0; i < len(s.peers); i++ {
					if s.majority[i] == 1 {
						vr++
					}
				}
				if vr >= (len(s.peers)/2)+1 {
					actions[0] = Commit{id: id, index: index, term: term, data: dat, err:nil}
				}
			} else if success == false && id!=0 {
				//s.nextIndex[id-1] = s.nextIndex[id-1] - 1
				//actions[0] = Send{id, AppendEntriesReqEv{term: s.log[s.nextIndex[id-1]].term, lid: s.id, prevLogIndex: s.nextIndex[id-1] - 1, prevLogTerm: s.log[s.nextIndex[id-1]-1].term, fl:1, data:s.log[s.nextIndex[id-1]-1].data, leaderCommit: s.commitIndex}}
			}
		}
	}
	return actions
}

func (s *SM) voteReq(id int, term int, logIndex int, logTerm int) []interface{} {
	actions := make([]interface{}, 10)
	switch s.status {
	case 1: //follower
		if s.curTerm <= term {
			if s.votedFor == 0 || s.votedFor == id {
				actions[0] = Alarm{timeout:s.electionTimeout}
				s.curTerm = term
				s.votedFor = 0
				if s.logTerm < logTerm {
					s.votedFor = id
					actions[1] = Send{id, VoteRespEv{id: s.id, term: s.curTerm, vote: true}}
				} else if s.logTerm == logTerm && s.logIndex <= logIndex {
					s.votedFor = id
					actions[1] = Send{id, VoteRespEv{id: s.id, term: s.curTerm, vote: true}}
				} else {
					actions[1] = Send{id, VoteRespEv{id: s.id, term: s.curTerm, vote: false}}
				}
				actions[2] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
			} else {
				actions[0] = Send{id, VoteRespEv{id: s.id, term: s.curTerm, vote: false}}
			}
		} else {
			actions[0] = Send{id, VoteRespEv{id: s.id, term: s.curTerm, vote: false}}
		}
	case 2: //candidate
		if s.curTerm <= term {
			s.status = 1
			s.curTerm = term
			s.votedFor = id
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[0] = Alarm{timeout:s.electionTimeout}
			actions[1] = Send{id, VoteRespEv{id: s.id, term: term, vote: true}}
			actions[2] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		} else {
			actions[0] = Send{id, VoteRespEv{id: s.id, term: s.curTerm, vote: false}}
		}
	case 3: //leader
		if s.curTerm < term {
			s.status = 1
			s.curTerm = term
			s.votedFor = id
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[0] = Alarm{timeout:s.electionTimeout}
			actions[1] = Send{id, VoteRespEv{id: s.id, term: term, vote: true}}
			actions[2] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		} else {
			actions[0] = Alarm{timeout:s.heartbeatTimeout}
			j := 1
			for i := 0; i < len(s.peers); i++ {
				if s.peers[i] != 0 {
					actions[j] = Send{s.peers[i], AppendEntriesReqEv{term: s.curTerm, lid: s.id, prevLogIndex: s.logIndex, prevLogTerm: s.logTerm, fl: 0, data:[]byte(""), leaderCommit: s.commitIndex}}
					j++
				}
			}
		}

	}
	return actions
}

func (s *SM) voteResp(id int, term int, vote bool) []interface{} {
	actions := make([]interface{}, 10)
	vr := 0
	j := 1
	switch s.status {
	case 1: //follower
		if s.curTerm < term {
			s.curTerm = term
			s.votedFor = 0
			actions[0] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		}
	case 2: //candidate
		if s.curTerm < term {
			s.status = 1
			s.curTerm = term
			s.votedFor = 0
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[0] = Alarm{timeout:s.electionTimeout}
			actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		} else {
			if vote == true && term == s.curTerm {
				s.majority[id-1] = 1
			}
			for i := 0; i < len(s.peers); i++ {
				if s.majority[i] == 1 {
					vr++
				}
			}
			if vr >= (len(s.peers)/2)+1 {
				s.status = 3
				s.lid = s.id
				actions[0] = Alarm{timeout:s.heartbeatTimeout}
				for i := 0; i < len(s.nextIndex); i++ {
					s.majority[i] = 0
					s.nextIndex[i] = s.logIndex + 1
				}
				for i := 0; i < len(s.peers); i++ {
					if s.peers[i] != 0 {
						actions[j] = Send{s.peers[i], AppendEntriesReqEv{term: s.curTerm, lid: s.id, prevLogIndex: s.logIndex, prevLogTerm: s.logTerm, fl: 0, data:[]byte(""), leaderCommit: s.commitIndex}}
						j++
					}
				}
			}
		}
	case 3: //leader
		if s.curTerm < term {
			s.status = 1
			s.curTerm = term
			s.votedFor = 0
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[0] = Alarm{timeout:s.electionTimeout}
			actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		}

	}
	return actions
}

func (s *SM) timeout() []interface{} {
	actions := make([]interface{}, 10)
	j := 1
	switch s.status {
	case 1: //follower
		s.curTerm++
		s.status = 2
		for i := 0; i < len(s.majority); i++ {
			s.majority[i] = 0
		}
		s.votedFor = s.id
		actions[0] = Alarm{timeout:s.electionTimeout}
		actions[j] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		j++
		s.majority[s.id-1] = 1
		for i := 0; i < len(s.peers); i++ {
			if s.peers[i] != 0 {
				actions[j] = Send{s.peers[i], VoteReqEv{candidateId: s.id, term: s.curTerm, logIndex: s.logIndex, logTerm: s.logTerm}}
				j++
			}
		}
	case 2: //candidate
		s.votedFor = s.id
		actions[0] = Alarm{timeout:s.electionTimeout}
		actions[j] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		j++
		for i := 0; i < len(s.majority); i++ {
			s.majority[i] = 0
		}
		s.majority[s.id-1] = 1
		for i := 0; i < len(s.peers); i++ {
			if s.peers[i] != 0 {
				actions[j] = Send{s.peers[i], VoteReqEv{candidateId: s.id, term: s.curTerm, logIndex: s.logIndex, logTerm: s.logTerm}}
				j++
			}
		}
	case 3: //leader
		actions[0] = Alarm{timeout:s.heartbeatTimeout}
		for i := 0; i < len(s.peers); i++ {
			if s.peers[i] != 0 {
				actions[j] = Send{s.peers[i], AppendEntriesReqEv{term: s.curTerm, lid: s.id, prevLogIndex: s.logIndex, prevLogTerm: s.logTerm, fl: 0, data:[]byte(""), leaderCommit: s.commitIndex}}
				j++
			}
		}
	}
	return actions
}
