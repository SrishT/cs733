package main

import (
	"fmt"
	"testing"
)


func BasicTests(t *testing.T) {
	TestFollower(t)
	TestCandidate(t)
	TestLeader(t)
}

func TestFollower(t *testing.T) {
	peer := make([]int,5)
	m := make([]int,5)
	lg := make([]int,100)
	ni := make([]int,5)
	for i:=0;i<5;i++ {
		peer[i]=i+1
		m[i]=0
		lg[i]=3
		ni[i]=5
	}
	actions:=make([]interface{},10)
	sm := SM{id:1,peers:peer,status:1,curTerm:3,votedFor:0,majority:m,commitIndex:2,log:lg,logTerm:2, logIndex:4, nextIndex:ni}

	//test AppendEntriesReqEv
	//with data
	actions=sm.ProcessEvent(AppendEntriesReqEv{term:3, lid:2, prevLogIndex:4, prevLogTerm:2, data:1 , leaderCommit:2})
	a := 0
	s := 0
	l := 0
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Alarm: a++
				case Send: s++
					cmd := actions[i].(Send)
					if cmd.id !=sm.lid {
						t.Error(fmt.Sprintf("Reply not sent to leader"))
					}
					switch cmd.event.(type) {
						case AppendEntriesRespEv : c := cmd.event.(AppendEntriesRespEv)
							if c.success == false {
								t.Error(fmt.Sprintf("Log Record Not Appended"))
							}
						default : t.Error(fmt.Sprintf("Didnt find AppendEntriesRespEv"))
					}
				case LogStore: l++
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	if !(a==1 && s==1 && l==1) {
		t.Error(fmt.Sprintf("Required Events not found"))
	}

	//heartbeat with higher leader commit
	actions=sm.ProcessEvent(AppendEntriesReqEv{term:3, lid:2, prevLogIndex:5, prevLogTerm:3, data:0 , leaderCommit:4})
	a = 0
	c := 0
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Alarm: a++
				case Commit: c++
					cmd := actions[i].(Commit)
					if cmd.id != sm.id || cmd.index != sm.commitIndex {
						t.Error(fmt.Sprintf("Commit not done"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	if !(a==1 && c==1) {
		t.Error(fmt.Sprintf("Required Events not found"))
	}

	//test AppendEntriesRespEv
	actions=sm.ProcessEvent(AppendEntriesRespEv{id:3,index:4,term:5,success:true})
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case SaveState :cmd := actions[i].(SaveState)
						if cmd.curTerm!=5 || cmd.votedFor!=0 {
							t.Error(fmt.Sprintf("State not changed"))
						}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}

	//test AppendEv
	actions=sm.ProcessEvent(AppendEv{data:1})
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Send :cmd := actions[i].(Send)
					if cmd.id !=sm.lid {
						t.Error(fmt.Sprintf("Reply not sent to leader"))
					}
					switch cmd.event.(type) {
						case AppendEv : c++
						default : t.Error(fmt.Sprintf("AppendEv not forwarded to leader"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
		
	//test Timeout
	a=0
	s=0
	c=0
	actions=sm.ProcessEvent(Timeout{})
	for i:=0;i<len(actions);i++ {
		if sm.status != 2 {
			t.Error(fmt.Sprintf("Didnt change to candidate"))
		}
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Alarm: a++
				case Send: s++
					cmd := actions[i].(Send)
					switch cmd.event.(type) {
						case VoteReqEv : c := cmd.event.(VoteReqEv)
							if c.candidateId!=sm.id || c.term!=sm.curTerm {
								t.Error(fmt.Sprintf("Wrong parameters for requesting vote"))
							}
						
						default : t.Error(fmt.Sprintf("Didnt find VoteReqEv"))
					}
				case SaveState :c++
						cmd := actions[i].(SaveState)
						if cmd.curTerm!=sm.curTerm || cmd.votedFor!=sm.id {
							t.Error(fmt.Sprintf("State not changed"))
						}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	if !(a==1 && s==len(sm.peers)-1 && c==1) {
		t.Error(fmt.Sprintf("Required Events not found"))
	}

	//test VoteReqEv
	//vote deny
	actions=sm.ProcessEvent(VoteReqEv{candidateId:3,term:5,logIndex:5,logTerm:3})
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Send:
					cmd := actions[i].(Send)
					switch cmd.event.(type) {
						case VoteRespEv : c := cmd.event.(VoteRespEv)
							if c.term!=sm.curTerm || c.vote!=false {
								t.Error(fmt.Sprintf("Wrong processing of request vote"))
							}
						default : t.Error(fmt.Sprintf("Didnt find VoteRespEv"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	
	//vote given
	actions=sm.ProcessEvent(VoteReqEv{candidateId:3,term:6,logIndex:5,logTerm:4})
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Send:
					cmd := actions[i].(Send)
					switch cmd.event.(type) {
						case VoteRespEv : c := cmd.event.(VoteRespEv)
							if c.id!=sm.id || c.term!=sm.curTerm || c.vote!=true {
								t.Error(fmt.Sprintf("Wrong processing of request vote"))
							}
						default : t.Error(fmt.Sprintf("Didnt find VoteRespEv"))
					}
				case SaveState :cmd := actions[i].(SaveState)
					if cmd.votedFor!=3 {
						t.Error(fmt.Sprintf("Not Voted"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}

	//deny vote as already voted for other
	actions=sm.ProcessEvent(VoteReqEv{candidateId:4,term:6,logIndex:5,logTerm:4})
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Send:
					cmd := actions[i].(Send)
					switch cmd.event.(type) {
						case VoteRespEv : c := cmd.event.(VoteRespEv)
							if c.term!=sm.curTerm || c.vote!=false {
								t.Error(fmt.Sprintf("Wrong processing of request vote"))
							}
						default : t.Error(fmt.Sprintf("Didnt find VoteRespEv"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}

	//test VoteRespEv
	actions=sm.ProcessEvent(VoteRespEv{id:4,term:5,vote:true})
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case SaveState :cmd := actions[i].(SaveState)
					if cmd.curTerm!=sm.curTerm || cmd.votedFor!=0 {
						t.Error(fmt.Sprintf("State not changed"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
}

func TestCandidate(t *testing.T) {
	peer := make([]int,5)
	m := make([]int,5)
	lg := make([]int,100)
	ni := make([]int,5)
	for i:=0;i<5;i++ {
		peer[i]=i+1
		m[i]=0
		lg[i]=3
		ni[i]=5
	}
	actions:=make([]interface{},10)
	sm := SM{id:3,peers:peer,status:2,curTerm:3,votedFor:0,majority:m,commitIndex:2,log:lg,logTerm:2, logIndex:4, nextIndex:ni}

	//test AppendEntriesReqEv
	//greater term
	actions=sm.ProcessEvent(AppendEntriesReqEv{term:4, lid:2, prevLogIndex:4, prevLogTerm:2, data:1 , leaderCommit:2})
	a:=0	
	st:=0
	s:=0
	c:=0
	//fmt.Println(actions)
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			if sm.status!=1 {
				t.Error(fmt.Sprintf("Did not change to Follower"))
			}
			switch actions[i].(type) {
				case Alarm: a++
				case SaveState: st++
					cmd := actions[i].(SaveState)
					if sm.curTerm!=4 || cmd.votedFor!=0 {
						t.Error(fmt.Sprintf("State not changed"))
					}
				case Send: s++
					cmd := actions[i].(Send)
					if cmd.id !=sm.id {
						t.Error(fmt.Sprintf("Not forwarded back to itself"))
					}
					switch cmd.event.(type) {
						case AppendEntriesReqEv : c++
						default : t.Error(fmt.Sprintf("Didnt find AppendEntriesReqEv"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	if !(a==1 && s==1 && st==1) {
		t.Error(fmt.Sprintf("Required Events not found"))
	}

	//changing back to candidate
	sm.status=2

	//lesser term
	actions=sm.ProcessEvent(AppendEntriesReqEv{term:2, lid:2, prevLogIndex:4, prevLogTerm:2, data:1 , leaderCommit:2})
	a=0	
	s=0
	//fmt.Println(actions)
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			//fmt.Println(reflect.TypeOf(actions[i]))
			switch actions[i].(type) {
				case Alarm: a++
				case Send: s++
					cmd := actions[i].(Send)
					if cmd.id !=2 {
						t.Error(fmt.Sprintf("Reply not sent to leader"))
					}
					switch cmd.event.(type) {
						case AppendEntriesRespEv : c := cmd.event.(AppendEntriesRespEv)
							if c.id!=sm.id || c.term!=sm.curTerm || c.success!=false {
								t.Error(fmt.Sprintf("Wrong processing of request vote"))
							}
						default : t.Error(fmt.Sprintf("Didnt find AppendEntriesReqEv"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	if !(a==1 && s==1) {
		t.Error(fmt.Sprintf("Required Events not found"))
	}

	//test AppendEntriesRespEv
	actions=sm.ProcessEvent(AppendEntriesRespEv{id:3,index:4,term:3,success:true})
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case SaveState :cmd := actions[i].(SaveState)
						if cmd.curTerm!=3 || cmd.votedFor!=0 {
							t.Error(fmt.Sprintf("State not changed"))
						}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}

	//test AppendEv
	actions=sm.ProcessEvent(AppendEv{data:1})
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Commit :cmd := actions[i].(Commit)
					if cmd.err !="No Leader" {
						t.Error(fmt.Sprintf("Error not sent"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	
	//test Timeout
	a=0
	s=0
	c=0
	actions=sm.ProcessEvent(Timeout{})
	for i:=0;i<len(actions);i++ {
		if sm.status != 2 {
			t.Error(fmt.Sprintf("Didnt change to candidate"))
		}
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Alarm: a++
				case Send: s++
					cmd := actions[i].(Send)
					switch cmd.event.(type) {
						case VoteReqEv : c := cmd.event.(VoteReqEv)
							if c.candidateId!=sm.id || c.term!=sm.curTerm {
								t.Error(fmt.Sprintf("Wrong parameters for requesting vote"))
							}
						default : t.Error(fmt.Sprintf("Didnt find VoteReqEv"))
					}
				case SaveState :c++
						cmd := actions[i].(SaveState)
						if cmd.curTerm!=sm.curTerm || cmd.votedFor!=sm.id {
							t.Error(fmt.Sprintf("State not changed"))
						}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	if !(a==1 && s==len(sm.peers)-1 && c==1) {
		t.Error(fmt.Sprintf("Required Events not found"))
	}

	//fmt.Println(sm.curTerm) 4

	//test VoteReqEv
	//higher term
	actions=sm.ProcessEvent(VoteReqEv{candidateId:3,term:5,logIndex:5,logTerm:3})
	if sm.status != 1 {
		t.Error(fmt.Sprintf("Didn't change to follower"))
	}
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Send:
					cmd := actions[i].(Send)
					if cmd.id!=3 {
						t.Error(fmt.Sprintf("Vote not sent to new candidate"))
					}
					switch cmd.event.(type) {
						case VoteRespEv : c := cmd.event.(VoteRespEv)
							if c.term!=sm.curTerm || c.vote!=true {
								t.Error(fmt.Sprintf("Wrong processing of request vote"))
							}
						default : t.Error(fmt.Sprintf("Didnt find VoteRespEv"))
					}
				case SaveState :cmd := actions[i].(SaveState)
					if cmd.curTerm!=sm.curTerm || cmd.votedFor!=3 {
						t.Error(fmt.Sprintf("State not changed"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}

	//changing back to candidate
	sm.status=2

	//lower term
	actions=sm.ProcessEvent(VoteReqEv{candidateId:3,term:3,logIndex:5,logTerm:3})
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Send:
					cmd := actions[i].(Send)
					if cmd.id!=3 {
						t.Error(fmt.Sprintf("Vote Resp not sent"))
					}
					switch cmd.event.(type) {
						case VoteRespEv : c := cmd.event.(VoteRespEv)
							if c.term!=sm.curTerm || c.vote!=false {
								t.Error(fmt.Sprintf("Wrong processing of request vote"))
							}
						default : t.Error(fmt.Sprintf("Didnt find VoteRespEv"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	//fmt.Println(sm.curTerm) //5
	
	//setting majority votes
	sm.majority=[]int{1,0,1,0,0}

	//test VoteRespEv
	s = 0
	actions=sm.ProcessEvent(VoteRespEv{id:2,term:5,vote:true})
	if sm.status!=3 {
		t.Error(fmt.Sprintf("Did not change to leader"))
	}
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Send : 
					cmd := actions[i].(Send)
					switch cmd.event.(type) {
						case AppendEntriesReqEv : c := cmd.event.(AppendEntriesReqEv)
							s++
							if c.lid!=sm.id {
								t.Error(fmt.Sprintf("Leader id not set"))
							}
						default : t.Error(fmt.Sprintf("Didnt find AppendEntriesReqEv"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	if s!=len(sm.peers)-1 {
		t.Error(fmt.Sprintf("Required Events not found"))
	}
}

func TestLeader(t *testing.T) {
	peer := make([]int,5)
	m := make([]int,5)
	lg := make([]int,100)
	ni := make([]int,5)
	for i:=0;i<5;i++ {
		peer[i]=i+1
		m[i]=0
		lg[i]=3
		ni[i]=5
	}
	actions:=make([]interface{},10)
	sm := SM{id:1,peers:peer,status:3,curTerm:3,votedFor:0,majority:m,commitIndex:2,log:lg,logTerm:2, logIndex:4, nextIndex:ni}

	//test AppendEntriesReqEv
	//higher term
	actions=sm.ProcessEvent(AppendEntriesReqEv{term:4, lid:2, prevLogIndex:4, prevLogTerm:2, data:1 , leaderCommit:2})
	a := 0
	s := 0
	l := 0
	if sm.status!= 1 {
		t.Error(fmt.Sprintf("Did not change to follower"))
	}
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Alarm: a++
				case SaveState :l++
						cmd := actions[i].(SaveState)
						if cmd.curTerm!=sm.curTerm || cmd.votedFor!=0 {
							t.Error(fmt.Sprintf("State not changed"))
						}
				case Send: s++
					cmd := actions[i].(Send)
					if cmd.id !=sm.id {
						t.Error(fmt.Sprintf("Not forwarded back to itself"))
					}
					switch cmd.event.(type) {
						case AppendEntriesReqEv :
						default : t.Error(fmt.Sprintf("Didnt find AppendEntriesReqEv"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	if !(a==1 && s==1 && l==1) {
		t.Error(fmt.Sprintf("Required Events not found"))
	}

	//changing back to leader
	sm.status=3

	//lower term
	actions=sm.ProcessEvent(AppendEntriesReqEv{term:2, lid:2, prevLogIndex:4, prevLogTerm:2, data:1 , leaderCommit:2})
	a = 0
	s = 0
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Alarm: a++
				case Send: s++
					cmd := actions[i].(Send)
					if cmd.id !=2 {
						t.Error(fmt.Sprintf("Response not sent"))
					}
					switch cmd.event.(type) {
						case AppendEntriesRespEv :c:=cmd.event.(AppendEntriesRespEv)
							if c.term!=sm.curTerm || c.success!=false {
								t.Error(fmt.Sprintf("Response parameters not set"))
							}
						default : t.Error(fmt.Sprintf("Didnt find AppendEntriesRespEv"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	if !(a==1 && s==1) {
		t.Error(fmt.Sprintf("Required Events not found"))
	}

	//test AppendEntriesRespEv
	//higher term
	actions=sm.ProcessEvent(AppendEntriesRespEv{id:3,index:4,term:5,success:true})
	if sm.status!= 1 {
		t.Error(fmt.Sprintf("Did not change to follower"))
	}
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case SaveState :cmd := actions[i].(SaveState)
						if sm.curTerm!=5 || cmd.votedFor!=0 {
							t.Error(fmt.Sprintf("State not changed"))
						}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	
	//changing back to leader
	sm.status=3

	//false reply decrements nextIndex
	actions=sm.ProcessEvent(AppendEntriesRespEv{id:3,index:4,term:5,success:false})
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Send :
					cmd := actions[i].(Send)
					switch cmd.event.(type) {
						case AppendEntriesReqEv : c := cmd.event.(AppendEntriesReqEv)
							if c.lid!=sm.id {
								t.Error(fmt.Sprintf("Wrong parameters"))
							}
						
						default : t.Error(fmt.Sprintf("Didnt find AppendEntriesReqEv"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}

	//test AppendEv
	l=0
	s=0
	actions=sm.ProcessEvent(AppendEv{data:1})
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case LogStore :l++
				case Send :s++
					cmd := actions[i].(Send)
					switch cmd.event.(type) {
						case AppendEntriesReqEv : c := cmd.event.(AppendEntriesReqEv)
							if c.term!=sm.curTerm || c.lid!=sm.id {
								t.Error(fmt.Sprintf("Wrong parameters"))
							}
						
						default : t.Error(fmt.Sprintf("Didnt find AppendEntriesReqEv"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	if !(l==1 && s==len(sm.peers)-1) {
		t.Error(fmt.Sprintf("Required Events not found"))
	}
		
	//test Timeout
	a=0
	s=0
	actions=sm.ProcessEvent(Timeout{})
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Alarm: a++
				case Send: s++
					cmd := actions[i].(Send)
					switch cmd.event.(type) {
						case AppendEntriesReqEv : c := cmd.event.(AppendEntriesReqEv)
							if c.term!=sm.curTerm || c.lid!=sm.id {
								t.Error(fmt.Sprintf("Wrong parameters for heartbeat message"))
							}
						
						default : t.Error(fmt.Sprintf("Didnt find heartbeat message"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	if !(a==1 && s==len(sm.peers)-1) {
		t.Error(fmt.Sprintf("Required Events not found"))
	}
	//fmt.Println(sm.curTerm)//5

	//test VoteReqEv
	//with  lower term
	s=0
	actions=sm.ProcessEvent(VoteReqEv{candidateId:3,term:4,logIndex:5,logTerm:3})
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Send: s++
					cmd := actions[i].(Send)
					switch cmd.event.(type) {
						case AppendEntriesReqEv : c := cmd.event.(AppendEntriesReqEv)
							if c.term!=sm.curTerm || c.lid!=sm.id {
								t.Error(fmt.Sprintf("Wrong parameters for heartbeat message"))
							}
						
						default : t.Error(fmt.Sprintf("Didnt find heartbeat message"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
	if !(s==len(sm.peers)-1) {
		t.Error(fmt.Sprintf("Required Events not found"))
	}
	
	//with higher term
	actions=sm.ProcessEvent(VoteReqEv{candidateId:3,term:6,logIndex:5,logTerm:4})
	if sm.status != 1 {
		t.Error(fmt.Sprintf("Didn't change to follower"))
	}
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Send:
					cmd := actions[i].(Send)
					if cmd.id!=3 {
						t.Error(fmt.Sprintf("Vote not sent to new candidate"))
					}
					switch cmd.event.(type) {
						case VoteRespEv : c := cmd.event.(VoteRespEv)
							if c.term!=sm.curTerm || c.vote!=true {
								t.Error(fmt.Sprintf("Wrong processing of request vote"))
							}
						default : t.Error(fmt.Sprintf("Didnt find VoteRespEv"))
					}
				case SaveState :cmd := actions[i].(SaveState)
					if cmd.curTerm!=sm.curTerm || cmd.votedFor!=3 {
						t.Error(fmt.Sprintf("State not changed"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}
	}
	
	//changing back to leader
	sm.status=3

	//test VoteRespEv
	actions=sm.ProcessEvent(VoteRespEv{id:4,term:7,vote:true})
	if sm.status != 1 {
		t.Error(fmt.Sprintf("Didn't change to follower"))
	}
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case SaveState :cmd := actions[i].(SaveState)
					if cmd.curTerm!=sm.curTerm || cmd.votedFor!=0 {
						t.Error(fmt.Sprintf("State not changed"))
					}
				default : t.Error(fmt.Sprintf("Found %v", actions[i]))
			}
		}		
	}
}
