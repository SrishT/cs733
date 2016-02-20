package main

//import "fmt"

type Send struct {
	id    int
	event interface{}
}

type LogStore struct {
	index int
	term  int
	data  int
}

type SaveState struct {
	curTerm  int
	votedFor int
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
	log         []int
	logTerm     int
	logIndex    int
	nextIndex   []int
}

type Commit struct {
	id    int
	index int
	term  int
	err   string
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
	electionTimeout int64
}

type AppendEv struct {
	data int
}

type AppendEntriesReqEv struct {
	term         int
	lid          int
	prevLogIndex int
	prevLogTerm  int
	data         int
	leaderCommit int
}

type AppendEntriesRespEv struct {
	id      int
	index   int
	term    int
	success bool
}

func (s *SM) ProcessEvent(ev interface{}) []interface{} {
	actions := make([]interface{}, 10)
	switch ev.(type) {
	case AppendEv:
		cmd := ev.(AppendEv)
		actions := s.appnd(cmd.data)
		return actions
	case AppendEntriesReqEv:
		//fmt.Println("NP 1")
		cmd := ev.(AppendEntriesReqEv)
		actions := s.appendEntriesReq(cmd.term, cmd.lid, cmd.prevLogIndex, cmd.prevLogTerm, cmd.data, cmd.leaderCommit, s.log)
		//fmt.Println("NP 3")
		return actions
	case AppendEntriesRespEv:
		cmd := ev.(AppendEntriesRespEv)
		actions := s.appendEntriesResp(cmd.id, cmd.index, cmd.term, cmd.success)
		return actions
	case Timeout:
		//cmd := ev.(Timeout)
		actions := s.timeout()
		return actions
	case VoteReqEv:
		cmd := ev.(VoteReqEv)
		//fmt.Println("log data",s.id," ",s.logIndex," ",s.logTerm)
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

func (s *SM) appnd(dat int) []interface{} {
	actions := make([]interface{}, 10)
	j := 0
	switch s.status {
	case 1: //follower
		actions[0] = Send{s.lid, AppendEv{dat}}
	case 2: //candidate
		err := "No Leader"
		actions[0] = Commit{s.id, -1, 0, err}
	case 3: //leader
		actions[j] = LogStore{index: s.logIndex + 1, term: s.curTerm, data: dat}
		j++
		s.log[s.logIndex+1] = s.curTerm
		s.majority[s.id-1] = 1
		for i := 0; i < len(s.peers); i++ {
			if s.peers[i] != s.id {
				actions[j] = Send{s.peers[i], AppendEntriesReqEv{term: s.curTerm, lid: s.id, prevLogIndex: s.logIndex, prevLogTerm: s.logTerm, data: dat, leaderCommit: s.commitIndex}}
				j++
			}
		}
		s.logIndex++
		s.logTerm = s.curTerm
	}
	return actions
}

func (s *SM) appendEntriesReq(trm int, lid int, prevLogInd int, prevLogTrm int, cmd int, lC int, log []int) []interface{} {
	actions := make([]interface{}, 10)
	actions[0] = Alarm{electionTimeout: 400}
	//fmt.Println("in append outside switch")
	switch s.status {
	case 1: //follower
		//fmt.Println("SP 3")
		if trm < s.curTerm {
			actions[1] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex, term: s.curTerm, success: false}}
		} else {
			s.lid = lid
			//fmt.Println("in append 1, data =",cmd)
			if s.logTerm == prevLogTrm && s.logIndex == prevLogInd {
				//fmt.Println("in append 2, data =",cmd)
				if cmd == 1 {
					//fmt.Println("here",cmd)
					actions[1] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex + 1,
						term: s.curTerm, success: true}}
					actions[2] = LogStore{index: prevLogInd + 1, term: trm, data: cmd}
					s.logIndex++
					s.logTerm = trm
					log[s.logIndex] = trm
				}
				//commit
				//fmt.Println("to commit 1",s.commitIndex,lC)
				if s.commitIndex < lC {
					//fmt.Println("to commit 2",s.commitIndex,lC)
					if s.logIndex < lC {
						s.commitIndex = s.logIndex
					} else {
						s.commitIndex = lC
					}
					actions[3] = Commit{id: s.id, index: s.commitIndex, term: trm, err: ""}
				}
			} else if s.logIndex == prevLogInd+1 && s.logTerm != trm {
				for i := s.logIndex; i < len(log); i++ {
					log[i] = 0
				}
			} else {
				actions[1] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex, term: s.curTerm, success: false}}
			}
		}
	case 2: //candidate
		//fmt.Println("Reached here")
		if s.curTerm < trm {
			s.status = 1
			s.curTerm = trm
			s.votedFor = 0
			actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[2] = Send{s.id, AppendEntriesReqEv{term: trm, lid: lid, prevLogIndex: prevLogInd, prevLogTerm: prevLogTrm, data: cmd, leaderCommit: lC}}
		} else {
			//fmt.Println("forwarding to leader")
			actions[1] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex, term: s.curTerm, success: false}}
		}
	case 3: //leader
		if s.curTerm < trm {
			s.status = 1
			s.curTerm = trm
			s.votedFor = 0
			actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[2] = Send{s.id, AppendEntriesReqEv{term: trm, lid: lid, prevLogIndex: prevLogInd, prevLogTerm: prevLogTrm, data: cmd, leaderCommit: lC}}
		} else {
			actions[1] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex, term: s.curTerm, success: false}}
		}

	}
	return actions
}

func (s *SM) appendEntriesResp(id int, index int, term int, success bool) []interface{} {
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
			actions[0] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		}
	case 3: //leader
		if s.curTerm < term {
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
					actions[0] = Send{id, AppendEntriesReqEv{term: s.log[s.nextIndex[id-1]], lid: s.id, prevLogIndex: s.nextIndex[id-1] - 1, prevLogTerm: s.log[s.nextIndex[id-1]-1], data: 1, leaderCommit: s.commitIndex}}
				}
				for i := 0; i < len(s.peers); i++ {
					if s.majority[i] == 1 {
						vr++
					}
				}
				if vr >= (len(s.peers)/2)+1 {
					actions[0] = Commit{id: id, index: index, term: term, err: ""}
				}
			} else if success == false {
				s.nextIndex[id-1] = s.nextIndex[id-1] - 1
				actions[0] = Send{id, AppendEntriesReqEv{term: s.log[s.nextIndex[id-1]], lid: s.id, prevLogIndex: s.nextIndex[id-1] - 1, prevLogTerm: s.log[s.nextIndex[id-1]-1], data: 1, leaderCommit: s.commitIndex}}
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
				s.curTerm = term
				s.votedFor = 0
				if s.logTerm < logTerm {
					s.votedFor = id
					actions[0] = Send{id, VoteRespEv{id: s.id, term: s.curTerm, vote: true}}
				} else if s.logTerm == logTerm && s.logIndex <= logIndex {
					s.votedFor = id
					actions[0] = Send{id, VoteRespEv{id: s.id, term: s.curTerm, vote: true}}
				} else {
					actions[0] = Send{id, VoteRespEv{id: s.id, term: s.curTerm, vote: false}}
				}
				actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
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
			actions[0] = Send{id, VoteRespEv{id: s.id, term: term, vote: true}}
			actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
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
			actions[0] = Send{id, VoteRespEv{id: s.id, term: term, vote: true}}
			actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		} else {
			j := 0
			for i := 0; i < len(s.peers); i++ {
				if s.peers[i] != s.id {
					actions[j] = Send{s.peers[i], AppendEntriesReqEv{term: s.curTerm, lid: s.id, prevLogIndex: s.logIndex, prevLogTerm: s.logTerm, data: 0, leaderCommit: s.commitIndex}}
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
	j := 0
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
			actions[0] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
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
				for i := 0; i < len(s.nextIndex); i++ {
					s.majority[i] = 0
					s.nextIndex[i] = s.logIndex + 1
				}
				for i := 0; i < len(s.peers); i++ {
					if s.peers[i] != s.id {
						actions[j] = Send{s.peers[i], AppendEntriesReqEv{term: s.curTerm, lid: s.id, prevLogIndex: s.logIndex, prevLogTerm: s.logTerm, data: 0, leaderCommit: s.commitIndex}}
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
			actions[0] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		}

	}
	return actions
}

func (s *SM) timeout() []interface{} {
	actions := make([]interface{}, 10)
	j := 0
	actions[j] = Alarm{electionTimeout: 400}
	j++
	switch s.status {
	case 1: //follower
		s.curTerm++
		s.status = 2
		for i := 0; i < len(s.majority); i++ {
			s.majority[i] = 0
		}
		s.votedFor = s.id
		actions[j] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		j++
		s.majority[s.id] = 1
		for i := 0; i < len(s.peers); i++ {
			if s.peers[i] != s.id {
				actions[j] = Send{s.peers[i], VoteReqEv{candidateId: s.id, term: s.curTerm, logIndex: s.logIndex, logTerm: s.logTerm}}
				j++
			}
		}
	case 2: //candidate
		s.votedFor = s.id
		actions[j] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor}
		j++
		for i := 0; i < len(s.majority); i++ {
			s.majority[i] = 0
		}
		s.majority[s.id] = 1
		for i := 0; i < len(s.peers); i++ {
			if s.peers[i] != s.id {
				actions[j] = Send{s.peers[i], VoteReqEv{candidateId: s.id, term: s.curTerm, logIndex: s.logIndex, logTerm: s.logTerm}}
				j++
			}
		}
	case 3: //leader
		for i := 0; i < len(s.peers); i++ {
			if s.peers[i] != s.id {
				actions[j] = Send{s.peers[i], AppendEntriesReqEv{term: s.curTerm, lid: s.id, prevLogIndex: s.logIndex, prevLogTerm: s.logTerm, data: 0, leaderCommit: s.commitIndex}}
				j++
			}
		}
	}
	return actions
}
