# fs - A simple network file server

**fs** is a simple network file server. Access to the server is via a
simple telnet compatible API. Each file has a version number, and the server keeps the latest version. There are four commands, to read, write, compare-and-swap and delete the file.

**fs** files have an optional expiry time attached to them. In combination with the `cas` command, this facility can be used as a coordination service, much like Zookeeper.

## Sample Usage


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

## Command Specification

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

## Install

```
go get github.com/cs733/assignment1
go test github.com/cs733/assignment1/...
```

## Limits and Limitations

- Port is fixed at 8080
- Files are in-memory. There's no persistence.
- If the command or contents line is in error such that the server
  cannot reliably figure out the end of the command, the connection is
  shut down. Examples of such errors are incorrect command name,
  numbytes is not not  numeric, the contents line doesn't end with a
  '\r\n'.
- The first line of the command is constrained to be less than 500 characters long. If not, an error is returned and the connection is shut donw.

## Contact
Sriram Srinivasan.
cs733 _at_ gmail.com


