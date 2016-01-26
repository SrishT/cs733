package main
import (
	"net"
	"os"
	"fmt"
	"time"
	"sync"
	"bufio"	
	"strings"
	"strconv"
	"io/ioutil"
)
type file struct {
	version int
	ctime int64
	exptime int64
	numbytes int
}
type concurrency struct {
	sync.RWMutex
}
var m map[string]file
func main() {
	serverMain()
}
func serverMain() {
	m=make(map[string]file)
	service := ":8080"
	tcpAddr, _ := net.ResolveTCPAddr("tcp4", service)
	listener, _ := net.ListenTCP("tcp", tcpAddr)
	go testTime()
	for {
		conn, err := listener.Accept()
		check(conn,err)
		go handleClient(conn)	
	}
}
func testTime() {
	for {
		ct:=time.Now()
		c:=ct.Unix()
		for k,v:=range m{
			if v.exptime != 0 {
				if v.ctime+v.exptime>c {
					_=os.Remove(k)
					delete(m,k)
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
}
func handleClient(conn net.Conn) {
	r:=bufio.NewReader(conn)
	buf := make([]byte, 1024)
	doParse(conn,r,buf)
}
func doParse(conn net.Conn,r *bufio.Reader,buf []byte) {
	var nbytes,vrsn,j int
	var etime int64
	var content string
	var err error
	var c concurrency
	var x,y byte
	//fmt.Println("Back Again")
	//fmt.Println(buf)
	//fmt.Println(string(buf))
	for i:=0; i < 1024; i++ {
		//fmt.Println("Start Parsing")
		x,err=r.ReadByte()
		//fmt.Println(x)
		//fmt.Println(string(x))
		buf[i]=x
		//fmt.Println(buf[i])
		//fmt.Println(string(buf[i]))
		if buf[0]==0 {
			return
		}
		//fmt.Println(string(buf[:i]))
		if i>0 && buf[i]=='\n' && buf[i-1]=='\r' {
			message:=string(buf[:i])
			s:=strings.Fields(message)
			if s[0]=="write" {
				if len(s)==3 {
					etime=0
				}else if len(s)==4 {
					etime,err=strconv.ParseInt(s[3], 10, 64)
					check(conn,err)
				}else {	
					fmt.Fprintf(conn,"ERR_CMD_ERR\r\n")
					conn.Close()
				}
				//fmt.Println("Writing")
				nbytes,err=strconv.Atoi(s[2])
				//fmt.Println(nbytes)
				check(conn,err)
				msg := make([]byte, 1024)
				for j = 0; j < nbytes; j++ {
					y,err=r.ReadByte()
					msg[j]=y
					if msg[0]==0 {
						fmt.Println("Content is 0")
						etime=0
					}
					//fmt.Println(j)
					//fmt.Println(string(msg[:j]))
				}
				content=string(msg[:j])
				//if content == "0" {
				//	fmt.Println("Content is 0")
				//	etime=0
				//}
				//fmt.Println(content)
				x,err=r.ReadByte()
				y,err=r.ReadByte()
				if x=='\r' && y=='\n' {
					c.write(conn,s[1],nbytes,etime,content)
					//fmt.Println("Continue parsing")
					//fmt.Println(string(buf))
					doParse(conn,r,buf)
				}else {
					fmt.Fprintf(conn,"ERR_CMD_ERR\r\n")
					conn.Close()
				}
			}else if s[0]=="read" {
				if len(s)==2 {
					c.read(conn,s[1])
					doParse(conn,r,buf)
				}else {
					fmt.Fprintf(conn,"ERR_CMD_ERR\r\n")
					conn.Close()
				}
			}else if s[0]=="cas" {
				if len(s)==4 {
					etime=0
				}else if len(s)==5 {
					etime,err=strconv.ParseInt(s[4], 10, 64)
					check(conn,err)
				}else {
					fmt.Fprintf(conn,"ERR_CMD_ERR\r\n")
					conn.Close()
				}
				vrsn,err=strconv.Atoi(s[2])
				check(conn,err)
				nbytes,err=strconv.Atoi(s[3])
				check(conn,err)
				msg := make([]byte, 1024)
				for j = 0; j < nbytes; j++ {
					y,err=r.ReadByte()
					msg[j]=y
					if msg[0]==0 {
						fmt.Println("Content is 0")
						etime=0
					}
				}
				content=string(msg[:j])
				//if content == "0" {
				//	fmt.Println("Content is 0")
				//	etime=0
				//}
				//fmt.Println(content)
				x,err=r.ReadByte()
				y,err=r.ReadByte()
				if x=='\r' && y=='\n' {
					c.cas(conn,s[1],vrsn,nbytes,etime,content)
					//fmt.Println("Continue parsing")
					//fmt.Println(string(buf))
					doParse(conn,r,buf)
				}else {
					fmt.Fprintf(conn,"ERR_CMD_ERR\r\n")
					conn.Close()
				}
			}else if s[0]=="delete" {
				if len(s)==2 {
					c.del(conn,s[1])
				}else {
					fmt.Fprintf(conn,"ERR_CMD_ERR\r\n")
					conn.Close()
				}
			}else {
				fmt.Fprintf(conn,"ERR_CMD_ERR\r\n")
				conn.Close()
			}
		}
	}	
}
func (c *concurrency) write(conn net.Conn,filename string,nbytes int,etime int64,content string) {
	c.Lock()
  	defer c.Unlock()
	ct:=time.Now()
	cur:=ct.Unix()
	v, ok := m[filename]
	if ok==true {
		m[filename]=file{v.version+1,cur,etime,nbytes}
	}else {
		//fmt.Printf("m = %v\n", m)
		m[filename]=file{1,cur,etime,nbytes}
	}
	//fmt.Println("Before write")
	//fmt.Println(content)
	err:= ioutil.WriteFile(filename,[]byte(content),0644)
	//fmt.Println("After write")
    	check(conn,err)
	fmt.Fprintf(conn,"OK %v\r\n",m[filename].version)
	//fmt.Println("Write Complete")
}
func (c *concurrency) read(conn net.Conn,filename string) {
	c.RLock()
  	defer c.RUnlock()
	_, ok := m[filename]	
	if ok==true {
		contents,err:=ioutil.ReadFile(filename)
		check(conn,err)
		temp:=m[filename]
		fmt.Fprintf(conn,"CONTENTS %v %v %v\r\n%v\r\n",temp.version,temp.numbytes,temp.exptime,string(contents[:]))
	}else {
		fmt.Fprintf(conn,"ERR_FILE_NOT_FOUND\r\n")
	}
}
func (c *concurrency) cas(conn net.Conn,filename string,vrsn int,nbytes int,etime int64,content string) {
	c.Lock()
  	defer c.Unlock()
	ct:=time.Now()
	cur:=ct.Unix()
	v, ok := m[filename]	
	if ok==true {
		ver:=v.version
		if ver==vrsn {
			m[filename]=file{ver+1,cur,etime,nbytes}
			err:= ioutil.WriteFile(filename,[]byte(content),0644)
			check(conn,err)
			fmt.Fprintf(conn,"OK %v\r\n",m[filename].version)
		}else {
			fmt.Fprintf(conn,"ERR_VERSION\r\n")
		}
	}else {
		fmt.Fprintf(conn,"ERR_FILE_NOT_FOUND\r\n")
		return
	}		
}
func (c *concurrency) del(conn net.Conn,filename string) {
	c.Lock()
  	defer c.Unlock()
	_, ok := m[filename]	
	if ok==true {
		err:=os.Remove(filename)
		check(conn,err)
		delete(m,filename)
		fmt.Fprintf(conn,"OK\r\n")
	}else {
		fmt.Fprintf(conn,"ERR_FILE_NOT_FOUND\r\n")
		return
	}
}
func check(conn net.Conn,err error) {
	if err != nil {
		fmt.Fprintf(conn, "ERR_INTERNAL %s", err.Error())
		conn.Close()
	}
}
