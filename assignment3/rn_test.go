package raft

import (
	"fmt"
	"time"
	"testing"
	//"github.com/cs733-iitb/cluster"
	//"github.com/cs733-iitb/cluster/mock"
	//"github.com/SrishT/cs733/assignment3/r"
)

func Test_basic(t *testing.T) {
	nodes:=makeRafts()
	go func() {
		nodes[0].runNode()
	}()
	go func() {
		nodes[1].runNode()
	}()
	go func() {
		nodes[2].runNode()
	}()
	time.Sleep(time.Second*10)
	fmt.Println("*******************************************")
	fmt.Println("*******************************************")
	fmt.Println("*************** Leader is *****************",nodes[0].sm.status,nodes[1].sm.status,nodes[2].sm.status)
	fmt.Println("*******************************************")
	fmt.Println("*******************************************")
	l:=LeaderId(nodes)
	nodes[l-1].Append([]byte("hello"))
	time.Sleep(time.Second*10)
	//fmt.Println("************** Monitoring Commit Channel ******************")
	ev := <-nodes[l-1].CommitChannel()
	//fmt.Println("************** Monitored Commit Channel ******************* ",ev)
	//fmt.Println("Errors : ",ev.Err," ",ev.Err.Error())
	if ev.Err == nil {
		if string(ev.Data) != "hello" {
			t.Fatal("Got different data")
		}
	}else {
		t.Fatal("Expected commit info")
	}
}
