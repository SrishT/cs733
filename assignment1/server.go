package main
import (
	"net"
	"os"
	"fmt"
	"strings"
	"strconv"
	"io/ioutil"
)
type file struct {
	version int
	ctime int
	exptime int
	numbytes int
	mutex int
}
var m map[string]file
func main() {
	m=make(map[string]file)
	serverMain()
}
func serverMain() {
	service := ":8080"
	tcpAddr, err := net.ResolveTCPAddr("tcp4", service)
	check(err)
	listener, err := net.ListenTCP("tcp", tcpAddr)
	check(err)
	for {
		conn, err := listener.Accept()
		check(err)	
		for {
			message:=make([]byte,1024)
			conn.Read(message)
			if message[0]!=0 {
				doParse(conn,string(message))
			}
		}
	}
}
func doParse(conn net.Conn,message string){
	var etime,nbytes,vrsn int
	var content string
	var err error
	a:=strings.SplitN(message,"\r\n",2)
	s:=strings.Fields(a[0])
	if s[0]=="write" {
		if len(s)==3 {
			etime=-1
		}else if len(s)==4 {
			etime,err=strconv.Atoi(s[3])
			check(err)
		}else {	
			fmt.Println("ERR_CMD_ERR\r\n")
			os.Exit(1)
		}
		nbytes,err=strconv.Atoi(s[2])
		check(err)
		if len(a[1])>=nbytes+2 {
			content=a[1][:nbytes]
			//if content[nbytes:nbytes+2]=="\r\n" {
				write(conn,s[1],nbytes,etime,content)
				if len(a[1])>nbytes+2 {
					newcontent:=a[1][nbytes+2:]
					nc:=strings.TrimSpace(newcontent)
					ar := []byte(nc)
					if ar[0]!=0 {
						doParse(conn,nc)
					}
				}	
			//}else {
			//	fmt.Println("ERR_INTERNAL\r\n : Number of bytes exceeding")
			//	return
			//}
		}else {
			fmt.Println("ERR_INTERNAL\r\n : Number of bytes less")
			return
		}			
	}else if s[0]=="read" {
		ar := []byte(a[1])
		if len(s)==2 {
			read(conn,s[1])
		}else {
			fmt.Println("ERR_CMD_ERR\r\n")
			os.Exit(1)
		}
		if ar[0]!=0 {
			doParse(conn,a[1])
		}
	}else if s[0]=="cas" {
		if len(s)==4 {
			etime=0
		}else if len(s)==5 {
			etime,err=strconv.Atoi(s[4])
			check(err)
		}else {
			fmt.Println("ERR_CMD_ERR\r\n")
			os.Exit(1)
		}
		vrsn,err=strconv.Atoi(s[2])
		check(err)
		nbytes,err=strconv.Atoi(s[3])
		check(err)
		if len(a[1])>=nbytes+2 {
			content=a[1][:nbytes]
			cas(conn,s[1],vrsn,nbytes,etime,content)
			if len(a[1])>nbytes+2 {
				newcontent:=a[1][nbytes+2:]
				nc:=strings.TrimSpace(newcontent)
				ar := []byte(nc)
				if ar[0]!=0 {
					doParse(conn,nc)
				}
			}
		}else {
			fmt.Println("ERR_INTERNAL\r\n : Number of bytes less")
			return
		}
	}else if s[0]=="delete" {
		ar := []byte(a[1])
		if len(s)==2 {
			del(conn,s[1])
		}else {
			fmt.Println("ERR_CMD_ERR\r\n")
			os.Exit(1)
		}
		if ar[0]!=0 {
			doParse(conn,a[1])
		}
	}
}
func write(conn net.Conn,filename string,nbytes int,etime int,content string) {
	v, ok := m[filename]
	if ok==true {
		m[filename]=file{v.version+1,1,etime,nbytes,0}
	}else {
		m[filename]=file{1,1,etime,nbytes,0}
	}
	err:= ioutil.WriteFile(filename,[]byte(content),0644)
    	check(err)
	fmt.Fprintf(conn,"OK %v\r\n",m[filename].version)
}
func read(conn net.Conn,filename string) {
	contents,err:=ioutil.ReadFile(filename)
	check(err)
	temp:=m[filename]
	fmt.Fprintf(conn,"CONTENTS %v %v %v\r\n%v\r\n",temp.version,temp.numbytes,temp.exptime,string(contents[:]))
}
func cas(conn net.Conn,filename string,vrsn int,nbytes int,etime int,content string) {
	v, ok := m[filename]	
	if ok==true {
		ver:=v.version
		if ver==vrsn {
			m[filename]=file{ver+1,1,etime,nbytes,0}
			err:= ioutil.WriteFile(filename,[]byte(content),0644)
    			check(err)
			fmt.Fprintf(conn,"OK %v\r\n",m[filename].version)
		}else {
			fmt.Println("ERR_VERSION\r\n")
		}
	}else {
		fmt.Println("ERR_FILE_NOT_FOUND\r\n")
		os.Exit(1)
	}		
}
func del(conn net.Conn,filename string) {
	err:=os.Remove(filename)
	check(err)
	delete(m,filename)
	fmt.Fprintf(conn,"OK\r\n")
}
func check(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERR_INTERNAL %s", err.Error())
		os.Exit(1)
	}
}
