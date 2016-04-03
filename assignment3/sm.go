package raft

import (
	"fmt"
	"errors"
	//"reflect"
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

//sriram - LogEntries is plural, whereas the struct represents a single log entry
//         Names really matter. Misuse of them can give rise to a lot of errors.
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
	timeout int
}

type AppendEv struct {
	data []byte
	flag int
}

type AppendEntriesReqEv struct {
	term         int
	lid          int
	prevLogIndex int64
	prevLogTerm  int
	fl	     int
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

func (s *SM) ProcessEvent(ev interface{}) []interface{} {
	//fmt.Println("Processing event ",reflect.TypeOf(ev))
	actions := make([]interface{}, 10)
	switch ev.(type) {
	case AppendEv:
		cmd := ev.(AppendEv)
		//fmt.Println("***************** In Append Process Event **************** ",cmd.data," ",cmd.flag)
		actions := s.appnd(cmd.data, cmd.flag)
		return actions
	case AppendEntriesReqEv:
		//fmt.Println("NP 1")
		cmd := ev.(AppendEntriesReqEv)
		actions := s.appendEntriesReq(cmd.term, cmd.lid, cmd.prevLogIndex, cmd.prevLogTerm, cmd.fl, cmd.data, cmd.leaderCommit)
		//fmt.Println("NP 3")
		return actions
	case AppendEntriesRespEv:
		cmd := ev.(AppendEntriesRespEv)
		actions := s.appendEntriesResp(cmd.id, cmd.index, cmd.term, cmd.data, cmd.success)
		return actions
	case Timeout:
		//fmt.Println("Processing Timeout Event")
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
		// sriram -- panic("Unrecognized"). If control comes here, it is indicative of a bigger problem; don't just print it and continue.
		println("Unrecognized")
	}
	return actions
}

func (s *SM) appnd(dat []byte,flag int) []interface{} {
	actions := make([]interface{}, 10)
	le := LogEntry{}
	j := 0
	switch s.status { 
	case FOLLOWER:
		//fmt.Println("---------In SM Follower Client Append--------------")
		actions[0] = Send{s.lid, AppendEv{dat,flag}}
	case CANDIDATE:
		//fmt.Println("----------In SM Candidate Client Append------------")
		var err error
		err = errors.New("No Leader")
		actions[0] = Commit{s.id, -1, 0, dat, err}
	case LEADER:
		//fmt.Println("----------In SM Leader Client Append----------------")
		actions[j] = LogStore{index: s.logIndex + 1, term: s.curTerm, data: dat} /*----------------correct data------------*/
		j++
		le.Data = dat
		le.Term = s.curTerm
		s.lg.Append(le)
		//fmt.Println("Storing data in log for node ",s.id," data: ",string(dat),"!")
		//d, err:=s.lg.Get(s.lg.GetLastIndex())
		//if err != nil {
		//     panic(err)
		//}
		// fmt.Println("-----------In SM Leader Client Append Sent Log Store------------")
		s.majority[s.id-1] = 1
		for i := 0; i < len(s.peers); i++ {
			if s.peers[i] != 0 {
				// sriram - Bad form for printing.
				//     1. Have a standard schema for printing: (date, node id, data)
				//     2. Print for a single node always (in the case of a follower)
				//fmt.Println("Sending Append Req Event to ",s.peers[i])
				actions[j] = Send{s.peers[i], AppendEntriesReqEv{term: s.curTerm, lid: s.id, prevLogIndex: s.logIndex, prevLogTerm: s.logTerm, fl: flag, data:dat, leaderCommit: s.commitIndex}}
				j++
			}
		}
		//fmt.Println(reflect.TypeOf(actions[0]).Name()," ",reflect.TypeOf(actions[1]).Name()," ",reflect.TypeOf(actions[2]).Name())
		s.logIndex++
		s.logTerm = s.curTerm
		//fmt.Println("Log indices after storing data : ",s.lg.GetLastIndex()," : ",s.logIndex)
		//fmt.Println("Data : ",string(d.(LogEntry).Data)," Term : ",d.(LogEntry).Term," at node ",s.id)
	}
	//fmt.Println("---------Returning actions from logStore--------------")
	return actions
}

func (s *SM) appendEntriesReq(trm int, lid int, prevLogInd int64, prevLogTrm int, flag int, dat []byte, lC int64) []interface{} {
	actions := make([]interface{}, 10)
	le := LogEntry{}
	switch s.status {
	case FOLLOWER:
		//fmt.Println("SP 3")
		if trm < s.curTerm {
			actions[0] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex, term: s.curTerm, data: dat, success: false}}
		} else {
			//fmt.Println("AppEntReq at ",s.id," ",prevLogTrm," ",s.logTerm," ",prevLogInd," ",s.logIndex," ",lC," ",s.commitIndex," ",flag)
			s.lid = lid
			actions[0] = Alarm{timeout:s.electionTimeout}
			//fmt.Println("in append 1, data =",cmd)
			if s.logTerm == prevLogTrm && s.logIndex == prevLogInd {
				//fmt.Println("in append 2, data =",cmd)
				if flag == 1 {
					//fmt.Println("here",cmd)
					s.curTerm = trm /////----------------------------check!! (just added)
					actions[1] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex + 1,
						term: s.curTerm, data: dat, success: true}}
					actions[2] = LogStore{index: prevLogInd + 1, term: trm, data:dat}
					//fmt.Println("Node : ",s.id," LogIndex before : ",s.logIndex)
					s.logIndex++
					//fmt.Println("Node : ",s.id," LogIndex after : ",s.logIndex)
					s.logTerm = trm
					le.Term = trm
					le.Data = dat

					// sriram **** DEBUG ****
					if s.id == 2 {
						fmt.Println("Before storing, term = ",le.Term," data = ",string(le.Data))
					}
					e:=s.lg.Append(le)
					if e!=nil {
						panic(e)
					}

				        // sriram *** DEBUG ****
					if s.id == 2 {
						println("@@@@@@@@ CHECKING APPEND @ 2 ")
						d, err:=s.lg.Get(s.lg.GetLastIndex())
						if err != nil {
							panic(err)
						}
						fmt.Println("Log indices after storing data : ",s.lg.GetLastIndex()," : ",s.logIndex)
						fmt.Println("Data : ",string(d.(LogEntry).Data)," Term : ",d.(LogEntry).Term," at node ",s.id)
					}
				}
				//commit
				//fmt.Println("to commit 1",s.commitIndex,lC)
				if s.commitIndex < lC {
					// fmt.Println("commiting entries at follower ",s.commitIndex," : ",lC)
					if s.logIndex < lC {
						s.commitIndex = s.logIndex
					} else {
						s.commitIndex = lC
					}
					//fmt.Println("modified commit indices ",s.commitIndex," : ",lC)
					actions[3] = Commit{id: s.id, index: s.commitIndex, term: trm, data: dat, err:nil}
					actions[4] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
				}
			} else if s.logIndex > prevLogInd && flag == 1 {
				t,_:= s.lg.Get(prevLogInd)
				if t.(LogEntry).Term == prevLogTrm {
					s.lg.TruncateToEnd(prevLogInd) //-----------------------------------handle
					s.curTerm = trm /////-------------------------------check!! (just added)
					actions[1] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex + 1,
						term: s.curTerm, data: dat, success: true}}
					actions[2] = LogStore{index: prevLogInd + 1, term: trm, data:dat}
					s.logIndex++
					s.logTerm = trm
					le.Term = trm
					le.Data = dat
					s.lg.Append(le)
					if s.commitIndex < lC {
						if s.logIndex < lC {
							s.commitIndex = s.logIndex
						} else {
							s.commitIndex = lC
						}
						actions[3] = Commit{id: s.id, index: s.commitIndex, term: trm, data: dat, err:nil}
						actions[4] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
					}	
				}
			} else {
				actions[1] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex, term: s.curTerm,  data: dat, success: false}}
			}
		}
	case CANDIDATE:
		//fmt.Println("Reached here")
		if s.curTerm < trm {
			actions[0] = Alarm{timeout:s.electionTimeout}
			s.status = 1
			s.curTerm = trm
			s.votedFor = 0
			actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[2] = Send{s.id, AppendEntriesReqEv{term: trm, lid: lid, prevLogIndex: prevLogInd, prevLogTerm: prevLogTrm, fl:flag, data:dat, leaderCommit: lC}}
		} else {
			actions[0] = Send{lid, AppendEntriesRespEv{id: s.id, index: s.logIndex, term: s.curTerm, data: dat, success: false}}
		} 
	case LEADER:
		if s.curTerm < trm {
			actions[0] = Alarm{timeout:s.electionTimeout}
			s.status = 1
			s.curTerm = trm
			s.votedFor = 0
			actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
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

func (s *SM) appendEntriesResp(id int, index int64, term int, dat []byte, success bool) []interface{} {
	actions := make([]interface{}, 10)
	switch s.status {
	case FOLLOWER:
		if s.curTerm < term {
			s.curTerm = term
			s.votedFor = 0
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[0] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
		}
	case CANDIDATE:
		if s.curTerm < term {
			s.status = 1
			s.curTerm = term
			s.votedFor = 0
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[0] = Alarm{timeout:s.electionTimeout}
			actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
		}
	case LEADER:
		if s.curTerm < term {
			actions[0] = Alarm{timeout:s.electionTimeout}
			s.status = 1
			s.curTerm = term
			s.votedFor = 0
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[0] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
		} else {
			vr := 0
			//actions[0] = Alarm{timeout:s.heartbeatTimeout}------------------check!!!
			if success == true && term == s.curTerm {
				s.majority[id-1] = 1
				//handle successive entries
				if index < s.logIndex {
					s.nextIndex[id-1] = s.nextIndex[id-1] + 1
					trm,_ := s.lg.Get(s.nextIndex[id-1])
					ptrm,_ := s.lg.Get(s.nextIndex[id-1]-1)
					d,_ := s.lg.Get(s.nextIndex[id-1])
					actions[0] = Send{id, AppendEntriesReqEv{term:trm.(LogEntry).Term, lid: s.id, prevLogIndex:s.nextIndex[id-1]-1, prevLogTerm:ptrm.(LogEntry).Term, fl:1, data:d.(LogEntry).Data, leaderCommit:s.commitIndex}}
				}
				for i := 0; i < len(s.peers); i++ {
					if s.majority[i] == 1 {
						vr++
					}
				}
				//fmt.Println("Votes Recv = ",vr)
				if vr >= (len(s.peers)/2)+1 {
					s.commitIndex++
					// fmt.Println("Updating Leader Commit")
					actions[0] = Commit{id: id, index: index, term: term, data: dat, err: nil}
					actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
				}
			} else if success == false && index>0 {
				s.nextIndex[id-1] = s.nextIndex[id-1]-1
				trm,_ := s.lg.Get(s.nextIndex[id-1])
				ptrm,_ := s.lg.Get(s.nextIndex[id-1]-1)
				d,_ := s.lg.Get(s.nextIndex[id-1])
				actions[0] = Send{id, AppendEntriesReqEv{term:trm.(LogEntry).Term, lid:s.id, prevLogIndex:s.nextIndex[id-1]-1, prevLogTerm:ptrm.(LogEntry).Term, fl:1, data:d.(LogEntry).Data, leaderCommit: s.commitIndex}}
			}
		}
	}
	return actions
}
func (s *SM) voteReq(id int, term int, logIndex int64, logTerm int) []interface{} {
	actions := make([]interface{}, 10)
        //fmt.Println("VoteReq received")
	switch s.status {
	case FOLLOWER:
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
				actions[2] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
			} else {
				actions[0] = Send{id, VoteRespEv{id: s.id, term: s.curTerm, vote: false}}
			}
		} else {
			actions[0] = Send{id, VoteRespEv{id: s.id, term: s.curTerm, vote: false}}
		}
	case CANDIDATE:
		if s.curTerm <= term {
			s.status = 1
			s.curTerm = term
			s.votedFor = id
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[0] = Alarm{timeout:s.electionTimeout}
			actions[1] = Send{id, VoteRespEv{id: s.id, term: term, vote: true}}
			actions[2] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
		} else {
			actions[0] = Send{id, VoteRespEv{id: s.id, term: s.curTerm, vote: false}}
		}
	case LEADER:
		if s.curTerm < term {
			s.status = 1
			s.curTerm = term
			s.votedFor = id
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[0] = Alarm{timeout:s.electionTimeout}
			actions[1] = Send{id, VoteRespEv{id: s.id, term: term, vote: true}}
			actions[2] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
		} else {
			actions[0] = Alarm{timeout:s.heartbeatTimeout}
			j := 1
			//fmt.Println("Sending heartbeat")
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
	case FOLLOWER:
		if s.curTerm < term {
			s.curTerm = term
			s.votedFor = 0
			actions[0] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
		}
	case CANDIDATE:
		if s.curTerm < term {
			s.status = 1
			s.curTerm = term
			s.votedFor = 0
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			// sriram - always use actions = append(actions, Alarm...)
			// That way you don't have to track indices. In general, be wary of hard numbers in code. They 
			// are always subject to change.
			actions[0] = Alarm{timeout:s.electionTimeout}
			actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
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
				// fmt.Println("---------- Leader Id ",s.lid,"--------")
				actions[0] = Alarm{timeout:s.heartbeatTimeout}
				for i := 0; i < len(s.nextIndex); i++ {
					s.majority[i] = 0
					s.nextIndex[i] = s.logIndex + 1
				}
				//fmt.Println("Sending heartbeat")
				for i := 0; i < len(s.peers); i++ {
					if s.peers[i] != 0 {
						actions[j] = Send{s.peers[i], AppendEntriesReqEv{term: s.curTerm, lid: s.id, prevLogIndex: s.logIndex, prevLogTerm: s.logTerm, fl: 0, data:[]byte(""), leaderCommit: s.commitIndex}}
						j++
					}
				}
			}
		}
	case LEADER:
		if s.curTerm < term {
			s.status = 1
			s.curTerm = term
			s.votedFor = 0
			for i := 0; i < len(s.majority); i++ {
				s.majority[i] = 0
			}
			actions[0] = Alarm{timeout:s.electionTimeout}
			actions[1] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
		}

	}
	return actions
}

func (s *SM) timeout() []interface{} {
	actions := make([]interface{}, 10)
	j := 1
	switch s.status {
	case FOLLOWER:
		s.curTerm++
		s.status = 2
		for i := 0; i < len(s.majority); i++ {
			s.majority[i] = 0
		}
		s.votedFor = s.id
		//fmt.Println("Voted for self by ",s.id)
		actions[0] = Alarm{timeout:s.electionTimeout}
		//fmt.Println("SM1 ",s.id)
		actions[j] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
		//fmt.Println("SM2 ",s.id)
		j++
		//fmt.Println("SM3 ",s.id)
		//fmt.Println("s.id", s.id)
		s.majority[s.id-1] = 1
		//fmt.Println("SM4 ",s.id)
		for i := 0; i < len(s.peers); i++ {
			if s.peers[i] != 0 {
				actions[j] = Send{s.peers[i], VoteReqEv{candidateId: s.id, term: s.curTerm, logIndex: s.logIndex, logTerm: s.logTerm}}
				j++
			}
		}
	case CANDIDATE:
		s.votedFor = s.id
		actions[0] = Alarm{timeout:s.electionTimeout}
		actions[j] = SaveState{curTerm: s.curTerm, votedFor: s.votedFor, commitIndex: s.commitIndex}
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
	case LEADER:
		actions[0] = Alarm{timeout:s.heartbeatTimeout}
		//fmt.Println("Sending heartbeat")
		for i := 0; i < len(s.peers); i++ {
			if s.peers[i] != 0 {
				actions[j] = Send{s.peers[i], AppendEntriesReqEv{term: s.curTerm, lid: s.id, prevLogIndex: s.logIndex, prevLogTerm: s.logTerm, fl: 0, data:[]byte(""), leaderCommit: s.commitIndex}}
				j++
			}
		}
	}
	return actions
}
