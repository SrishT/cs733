FILE SERVER

This implementation of the file server has the following properties :-

1. The write operation creates a new file with the given name, and sets the expiry time to the default value 0 (i.e. the file will never expire) if that field is not specified. It reads as many bytes of contents as specified in the write command. If lesser input is given, the file server will wait till the required number of bytes are provided. If the file does not already exist, a map entry is created for the file and its version is set to 1, otherwise the contents of the file are updated and its version is incremented by 1.

2. The read operation first checks whether a file with the given name exists or not. If it does, the file server returns the required details about the file along with its contents, else it simply returns an error saying that the file does not exist.

3. The cas operation first checks whether a file with the given name exists or not. It sets the expiry time to the default value 0 (i.e. the file will never expire) if that field is not specified. If the version number specified in this command is same as the current version of the given filename, the contents of the file are swapped with the newly specified contents, and the map entry of the file is also updated appropriately, else an error is returned.

4. The delete operation first checks whether a file with the given name exists or not. If it does, the file and its corresponding map entry is deleted, else n error is returned saying that the file does not exist.

This file server implements concurrency, expiry and read write locks to ensure consistency.
