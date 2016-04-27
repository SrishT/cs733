package raft

import (
	"os"
	"fmt"
	"time"
	"testing"
	"strconv"
	"encoding/json"
	"github.com/cs733-iitb/cluster"
	"github.com/cs733-iitb/cluster/mock"
)

type LogData struct {
	Data int
}

func Test_start(t *testing.T) {
	cleanup()
}

func cleanup() {
	for i:=1; i<= NUMRAFTS; i++ {
		os.RemoveAll("LogDir"+strconv.Itoa(i))
		os.RemoveAll("StateStore"+strconv.Itoa(i)+".json")
	}
}

func getData(data []byte) int {
	var ld LogData
	_ = json.Unmarshal(data, &ld)
	d := ld.Data
	return d
}

func sendData(nodes []*RaftNode,l int,data int) {
	if l!=-1 {
		d := LogData{data}
		ldr := nodes[l-1]
		ldr.Append(d)
	} else {
		fmt.Println("Cannot append as no leader")
	}
}

func Test_basic(t *testing.T) {
	clconfig := cluster.Config{Peers:[]cluster.PeerConfig {
		{Id:1}, {Id:2}, {Id:3}, {Id:4}, {Id:5},
	}}
	cl, _ := mock.NewCluster(clconfig)
	nodes := make([]*RaftNode,5)
	for i:=0;i<NUMRAFTS;i++ {
		config := &ConfigRN{Id:i+1,Cluster:cl,ElectionTimeout:1000,HeartbeatTimeout:100,}
		if i==0 {
			config.ElectionTimeout=500
		}
		nodes[i] = MakeRaft(i+1,config)
	}
	time.Sleep(time.Second*1)
	l := LeaderId(nodes)
	ldr := nodes[l-1]

	for i:=0;i<1000;i++ {
		sendData(nodes,l,i)
	}

	time.Sleep(time.Second*2)
	
	for i:=0;i<1000;i++ {
		CheckCommit(t,nodes,i)
	}

	
	/* Testing Partition */
			
	cl.Partition([]int{1, 2}, []int{3, 4, 5})
	time.Sleep(time.Second*2)
	l1 := LeaderId(nodes[:2])
	l2 := LeaderId(nodes[2:])
	sendData(nodes,l1,12)
	select {
		case _ = <-ldr.CommitChannel():
			t.Fatal("Incorrectly commited at minority")
		case <-ldr.timeoutCh.C:
	}
	sendData(nodes,l2,10)
	CheckCommit(t,nodes[2:],10)

	cl.Heal()
	time.Sleep(time.Second*2)
	l1 = LeaderId(nodes)
	sendData(nodes,l1,11)
	time.Sleep(time.Second*2)
	CheckCommit(t,nodes[:2],10)
	CheckCommit(t,nodes,11)
	
	
	/* Testing Shutdown */
	
	time.Sleep(time.Second*2)
	l1 = LeaderId(nodes)
	nodes[l1-1].Shutdown()
	time.Sleep(time.Second*2)
	l1 = LeaderId(nodes)
	
}

func CheckCommit(t *testing.T,nodes []*RaftNode,data int) {
	for i:=0;i<len(nodes);i++ {
		select {
		case e := <-nodes[i].CommitChannel():
			if e.Err != nil {t.Fatal(e.Err)}
			d := getData(e.Data)
			if d != data {t.Fatal("Wrong data")}
		}
	}
}

func Test_end(t *testing.T) {
	cleanup()
}
