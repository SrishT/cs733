package main

import (
	"sm"
	"fmt"
	"log"
	"time"
	"cluster"
)

type RaftNode struct { // implements Node interface
	id int
	leaderId int
	eventCh chan Event
	timeoutCh <-chan Time
	CommitChannel() <- chan CommitInfo
}

func (rn *RaftNode) Append(data) {
	rn.eventCh <- Append{data: data}
}

func New(config Config) Node {
	
}

func (rn *RaftNode) processActions (actions []interface{}) {
	for i:=0;i<len(actions);i++ {
		if actions[i]!=nil {
			switch actions[i].(type) {
				case Alarm: 
				case Send: cmd := actions[i].(Send)
					switch cmd.event.(type) {
						case VoteReqEv : c := cmd.event.(VoteReqEv)
							ev := VoteReqEv{c.candidateId, c.term, c.logIndex, c.logTerm}
						case VoteRespEv : c := cmd.event.(VoteRespEv)
							ev := VoteRespEv{c.id, c.term, c.vote}
						case AppendEntriesReqEv : c := cmd.event.(AppendEntriesReqEv)
							ev := sm.ProcessEvent{AppendEntriesReqEv{c.term, c.lid, c.prevLogIndex, c.prevLogTerm, c.data, c.leaderCommit}}
						case AppendEntriesRespEv : c := cmd.event.(AppendEntriesRespEv)
							ev := sm.ProcessEvent{AppendEntriesReqEv{c.id,c.index,c.term,c.success}}
					}
					rn.Outbox() <- &cluster.Envelope{Pid: cmd.id, Msg: ev}
				case Commit: CommitChannel <- actions[i]
				case LogStore: cmd := actions[i].(LogStore)
				case SaveState:
			}
		}
	}
}

func (rn *RaftNode) monitorChannel {
	for {
		var ev Event
		select {
			case ev = <- eventCh {ev = msg}
			case <- timeoutCh {ev = Timeout{}}
		}
		actions := sm.ProcessEvent(ev)
		rn.processActions(actions)
	}
}
