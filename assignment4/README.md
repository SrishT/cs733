#Distributed File System using RAFT

Given is a simple implementation of a distributed file system which makes use of the RAFT consensus protocol in order to distribute the file system across a cluster of servers (5 in this case) and ensure that each of the servers agrees on the series of actions to be performed locally in their state machines.
It is comprised of the following parts

## fs - A simple network file server

**fs** is a simple network file server. Access to the server is via a
simple telnet compatible API. Each file has a version number, and the server keeps the latest version. There are four commands, to read, write, compare-and-swap and delete the file.

**fs** files have an optional expiry time attached to them. In combination with the `cas` command, this facility can be used as a coordination service, much like Zookeeper.

### Sample Usage

```
> go run server.go & 

> telnet localhost 8080
  Connected to localhost.
  Escape character is '^]'
  read foo
  ERR_FILE_NOT_FOUND
  write foo 6
  abcdef
  OK 1
  read foo
  CONTENTS 1 6 0
  abcdef
  cas foo 1 7 0
  ghijklm
  OK 2
  read foo
  CONTENTS 2 7 0
  ghijklm
```

### Command Specification

The format for each of the four commands is shown below,  

| Command  | Success Response | Error Response
|----------|-----|----------|
|read _filename_ \r\n| CONTENTS _version_ _numbytes_ _exptime remaining_\r\n</br>_content bytes_\r\n </br>| ERR_FILE_NOT_FOUND
|write _filename_ _numbytes_ [_exptime_]\r\n</br>_content bytes_\r\n| OK _version_\r\n| |
|cas _filename_ _version_ _numbytes_ [_exptime_]\r\n</br>_content bytes_\r\n| OK _version_\r\n | ERR\_VERSION _newversion_
|delete _filename_ \r\n| OK\r\n | ERR_FILE_NOT_FOUND

In addition the to the semantic error responses in the table above, all commands can get two additional errors. `ERR_CMD_ERR` is returned on a malformed command, `ERR_INTERNAL` on, well, internal errors.

For `write` and `cas` and in the response to the `read` command, the content bytes is on a separate line. The length is given by _numbytes_ in the first line.

Files can have an optional expiry time, _exptime_, expressed in seconds. A subsequent `cas` or `write` cancels an earlier expiry time, and imposes the new time. By default, _exptime_ is 0, which represents no expiry. 


## raft - A consensus mechanism

**raft** is the layer which is responsible for command replication over each of the file servers in the cluster, and to ensure their consistency. It ensures that the command is handed over to the state machine of a given file server instance of the cluster, only if it has been successfully replicated over a majority of the servers in the cluster.

This coordination is achieved by the leader node, which is one of the nodes in the cluster, which has been voted for by a majority in the cluster.


## Install

```
go get github.com/SrishT/assignment4
go test github.com/SrishT/assignment4/...
```

## Limits and Limitations

- Files are in-memory. There's no persistence.
- If the command or contents line is in error such that the server
  cannot reliably figure out the end of the command, the connection is
  shut down. Examples of such errors are incorrect command name,
  numbytes is not not  numeric, the contents line doesn't end with a
  '\r\n'.
- The first line of the command is constrained to be less than 500 characters long. If not, an error is returned and the connection is shut donw.
