package raft

import (
	"fmt"
	"time"
	"testing"
	"os"
	//"github.com/cs733-iitb/log"
	//"github.com/cs733-iitb/cluster"
	//"github.com/cs733-iitb/cluster/mock"
	//"github.com/SrishT/cs733/assignment3/r"
)

func Test_start(t *testing.T) {
	os.RemoveAll("LogDir1")
	os.RemoveAll("LogDir2")
	os.RemoveAll("LogDir3")
	os.RemoveAll("StateStore1.json")
	os.RemoveAll("StateStore2.json")
	os.RemoveAll("StateStore3.json")
}

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
	time.Sleep(time.Second*2)
	//fmt.Println("*******************************************")
	//fmt.Println("*******************************************")
	//fmt.Println("*************** Leader is *****************",nodes[0].sm.status,nodes[1].sm.status,nodes[2].sm.status)
	//fmt.Println("*******************************************")
	//fmt.Println("*******************************************")
	l:=LeaderId(nodes)
	println("**** leader Id = ", l)
	n1:=nodes[l-1].sm.lg.GetLastIndex()
	
	nodes[l-1].Append("hello")
	//time.Sleep(time.Second*10)
	//fmt.Println("************** Monitoring Commit Channel ******************")
	//for i:=0;i<3;i++ {
	ev := <-nodes[l-1].CommitChannel()
	if ev.Err != nil {t.Fatal(ev.Err)}
	fmt.Println("Data commit at node ",l," : ",string(ev.Data))
	n2:=nodes[l-1].sm.lg.GetLastIndex()
	l=LeaderId(nodes)
	nodes[l-1].Append("user")
	ev = <-nodes[l-1].CommitChannel()
	if ev.Err != nil {t.Fatal(ev.Err)}
	fmt.Println("Data commit at node ",l," : ",string(ev.Data))
	n3:=nodes[l-1].sm.lg.GetLastIndex()
	time.Sleep(time.Second*10)
	for i:=0;i<3;i++ {
		fmt.Println("value of i",i+1)
		data,_ := nodes[i].sm.lg.Get(n2)
		fmt.Println("Node ",i+1," Data: ",string(data.(LogEntries).Data)," Term: ",data.(LogEntries).Term)
		data,_ = nodes[i].sm.lg.Get(n3)
		fmt.Println("Node ",i+1," Data: ",string(data.(LogEntries).Data)," Term: ",data.(LogEntries).Term)
	}
	fmt.Println("Indices are ",n1," : ",n2," : ",n3)
	//}
	//ev := <-nodes[l-1].CommitChannel()
	
	//fmt.Println("************** Monitored Commit Channel ******************* ",ev)
	//fmt.Println("Errors : ",ev.Err," ",ev.Err.Error())
	//if ev.Err == nil {
	//	fmt.Println(string(ev.Data))
	//	if string(ev.Data) != "hello" {
	//		t.Fatal("Got different data")
	//	}
	//}else {
	//	t.Fatal("Expected commit info")
	//}
}
