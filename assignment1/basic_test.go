package main

import (
	"bufio"
	"fmt"
	"net"
	"time"
	"strconv"
	"strings"
	"testing"
)


// Simple serial check of getting and setting
func TestTCPSimple(t *testing.T) {
	go serverMain()
        time.Sleep(1 * time.Second) // one second is enough time for the server to start
	name1 := "hi.txt"
	contents1 := "bye"
	ncontents1 := "bye again"
	exptime1 := 3000
	nexptime1 := 4000
	version1 := 1
	name2 := "hello.txt"
	contents2 := "hi"
	exptime2 := 15
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		t.Error(err.Error()) // report error through testing framework
	}
	scanner := bufio.NewScanner(conn)
	
	// Write a file
	fmt.Fprintf(conn, "write %v %v %v\r\n%v\r\n", name1, len(contents1), exptime1, contents1)	
	scanner.Scan() // read first line
	resp := scanner.Text() // extract the text from the buffer
	arr := strings.Split(resp, " ") // split into OK and <version>
	expect(t, arr[0], "OK")
	version1, err = strconv.Atoi(arr[1]) // parse version as number
	if err != nil {
		t.Error("Non-numeric version found")
	}

	//Reading
	fmt.Fprintf(conn, "read %v\r\n", name1) // try a read now
	scanner.Scan()
	resp = scanner.Text() // extract the text from the buffer
	arr = strings.Split(scanner.Text(), " ")
	expect(t, arr[0], "CONTENTS")
	expect(t, arr[1], fmt.Sprintf("%v", version1)) // expect only accepts strings, convert int version to string
	expect(t, arr[2], fmt.Sprintf("%v", len(contents1)))	
	scanner.Scan()
	expect(t, contents1, scanner.Text())

	// CAS
	fmt.Fprintf(conn, "cas %v %v %v %v\r\n%v\r\n", name1, version1, len(ncontents1), nexptime1, ncontents1)
	scanner.Scan() // read first line
	resp = scanner.Text() // extract the text from the buffer
	arr = strings.Split(resp, " ") // split into OK and <version>
	expect(t, arr[0], "OK")
	_, err = strconv.Atoi(arr[1]) // parse version as number
	if err != nil {
		t.Error("Non-numeric version found")
	}
	version1++
	expect(t, arr[1], fmt.Sprintf("%v", version1))


	// testing after expiry
	fmt.Fprintf(conn, "write %v %v %v\r\n%v\r\n", name2, len(contents2), exptime2, contents2)	
	scanner.Scan() // read first line
	time.Sleep(16 * time.Second)
	fmt.Fprintf(conn, "read %v\r\n", name2) // try a read now
	scanner.Scan()
	resp = scanner.Text() // extract the text from the buffer
	arr = strings.Split(scanner.Text(), " ")
	errchar := strings.Split(arr[0], "_")
	expect(t, errchar[0], "ERR")

	// concurrency	
	for i:=0;i<5;i++ {
		go runConcurrent(t,i)
	}
}

func runConcurrent(t *testing.T,i int) {
	name     := "hi.txt"
	contents := "Hey"
	exptime  := 300000
	conn, err:= net.Dial("tcp", "localhost:8080")

		if err != nil {
			t.Error(err.Error()) // report error through testing framework
		}
        //fmt.Println(conn)
	scanner  := bufio.NewScanner(conn)
	fmt.Fprintf(conn, "write %v %v %v\r\n%v\r\n", name, len(contents), exptime, contents+strconv.Itoa(i))
	scanner.Scan() // read first line
	resp := scanner.Text() // extract the text from the buffer
	arr := strings.Split(resp, " ") // split into OK and <version>
	expect(t, arr[0], "OK")
	_, err = strconv.ParseInt(arr[1], 10, 64) // parse version as number
	if err != nil {
		t.Error("Non-numeric version found")
	}	
}

// Useful testing function
func expect(t *testing.T, a string, b string) {
	if a != b {
		t.Error(fmt.Sprintf("Expected %v, found %v", b, a)) // t.Error is visible when running `go test -verbose`
	}
}
