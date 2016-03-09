package main

import (
	"fmt"
)

type RaftNode struct { // implements Node interface
	eventCh chan Event
	timeoutCh <-chan Time
}

func (rn *RaftNode) Append(data) {
	rn.eventCh <- Append{data: data}
}

func (rn *RaftNode) processEvents {
	for {
		var ev Event
		select {
			case ev = <- eventCh {ev = msg} 
			<- timeoutCh {ev = Timeout{}}
		}
		actions := sm.ProcessEvent(ev)
		doActions(actions)
	}
}
