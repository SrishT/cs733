package raft

import (
	"os"
	"fmt"
	"time"
	"testing"
	"strconv"
	"io/ioutil"
	"encoding/json"
	"github.com/cs733-iitb/log"
	"github.com/cs733-iitb/cluster"
	"github.com/cs733-iitb/cluster/mock"
)

var NUMRAFTS = 5

func Test_start(t *testing.T) {
	cleanup()
}

func cleanup() {
	for i:=1; i<= NUMRAFTS; i++ {
		os.RemoveAll("LogDir"+strconv.Itoa(i))
		os.RemoveAll("StateStore"+strconv.Itoa(i)+".json")
	}
}

func makeRafts() ([]*RaftNode) {
	
	leaderId = -1

	clconfig := cluster.Config{Peers:[]cluster.PeerConfig {
		{Id:1}, {Id:2}, {Id:3}, {Id:4}, {Id:5},
	}}
	cl, _ := mock.NewCluster(clconfig)

	nodes := make([]*RaftNode, len(clconfig.Peers))

	raftConfig := ConfigRN{
		ElectionTimeout: 1200,
		HeartbeatTimeout: 500,
	}

	for i := 1; i <= NUMRAFTS; i++ {
		c := 0
		ct := 0
		vf := 0
		lt := 0
		li := int64(-1)
		ci := int64(-1)
		raftNode := NewRN(i, raftConfig)
		srv := cl.Servers[i]
		p := srv.Peers()
		m := make([]int,NUMRAFTS)
		peer := make([]int,NUMRAFTS)
		ni := make([]int64,NUMRAFTS)
		mi := make([]int64,NUMRAFTS)
		log, err := log.Open(raftNode.LogDir)
		log.RegisterSampleEntry(LogEntry{})
		if err!=nil {
			fmt.Println("Error opening Log File")
		}
		for j:=0;j<NUMRAFTS;j++ {
			m[j]=0
			ni[j]=0
			mi[j]=-1
			if j==i-1 {
				peer[j]=0
			}else {
				peer[j]=p[c]
				c++
			}
		}
		raftNode.server = srv
		file, e := ioutil.ReadFile(raftNode.StateDir)
    		if e == nil {
			var m Message
			err = json.Unmarshal(file, &m)
			ct = m.Term
			vf = m.VotedFor
			ci = m.CommitIndex
			li = log.GetLastIndex()
			mi[i-1] = li
			t,_ := log.Get(li)
			lt = t.(LogEntry).Term
			for j:=0;j<NUMRAFTS;j++ {
				ni[j]=li+1
			}
		}
		// sriram ** DEBUG REMOVE AFTER DEBUGGING **/
		electionTimeOut := 500
		if srv.Pid() != 1 {
			electionTimeOut = 10000 // ensuring leader 1 is always 1
		}

		raftNode.sm = &SM{id:srv.Pid(), lid:-1,peers:peer,status:1, curTerm:ct, votedFor:vf, majority:m, commitIndex:ci, lg:log, logIndex:li, matchIndex:mi, logTerm:lt, nextIndex:ni, electionTimeout: electionTimeOut, heartbeatTimeout:raftConfig.HeartbeatTimeout}
		//raftNode.sm.log.RegisterSampleEntry(LogEntries{})	
		nodes[i-1] = raftNode
	}
	return nodes
}

func Test_basic(t *testing.T) {
	nodes:=makeRafts()
	// sriram -- why is the following not in a loop? Hardcoding array indices is almost always a sign 
	// of a potential bug or repetition
	/*
	for i:=0; i<3; i++ {
		go func() {
			nodes[i].runNode()
		}()
	}
	*/

	go func() {
		nodes[0].runNode()
	}()
	go func() {
		nodes[1].runNode()
	}()
	go func() {
		nodes[2].runNode()
	}()
	go func() {
		nodes[3].runNode()
	}()
	go func() {
		nodes[4].runNode()
	}()

	time.Sleep(time.Second*4)

	l := LeaderId(nodes)
	ldr := nodes[l-1]


	for i:=0;i<10;i++ {
		ldr.Append(strconv.Itoa(i))
	}

	for i:=0;i<10;i++ {
		data,index := CheckCommit(t,ldr)
		fmt.Println("At ",i," Data Committed = ",data," Index = ",index," i = ",i)
	}
	
	n:=ldr.sm.lg.GetLastIndex()
	fmt.Println("Last index at leader : ",n)
	data,_ := ldr.sm.lg.Get(n)
	fmt.Println("At leader, Data: ",string(data.(LogEntry).Data)," Term: ",data.(LogEntry).Term)

	n=nodes[1].sm.lg.GetLastIndex()
	fmt.Println("Last index at node 2 : ",n)
	data,_ = nodes[1].sm.lg.Get(n)
	fmt.Println("At node 2, Data: ",string(data.(LogEntry).Data)," Term: ",data.(LogEntry).Term)

	n=nodes[2].sm.lg.GetLastIndex()
	fmt.Println("Last index at node 3 : ",n)
	data,_ = nodes[2].sm.lg.Get(n)
	fmt.Println("At node 3, Data: ",string(data.(LogEntry).Data)," Term: ",data.(LogEntry).Term)

	n=nodes[3].sm.lg.GetLastIndex()
	fmt.Println("Last index at node 4 : ",n)
	data,_ = nodes[3].sm.lg.Get(n)
	fmt.Println("At node 4, Data: ",string(data.(LogEntry).Data)," Term: ",data.(LogEntry).Term)

	n=nodes[4].sm.lg.GetLastIndex()
	fmt.Println("Last index at node 5 : ",n)
	data,_ = nodes[4].sm.lg.Get(n)
	fmt.Println("At node 5, Data: ",string(data.(LogEntry).Data)," Term: ",data.(LogEntry).Term)
	
	//cl.Partition([]int{1, 2, 3}, []int{4, 5})
	
}

func CheckCommit(t *testing.T,ldr *RaftNode) (string,int64) {
	for {
		select {
		case e := <-ldr.CommitChannel():
			if e.Err != nil {t.Fatal(e.Err)}
			d:=string(e.Data)
			i:=e.Index
			return d,i
		}
	}
}

func Test_end(t *testing.T) {
	//cleanup()
}
