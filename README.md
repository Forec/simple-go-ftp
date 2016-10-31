# File Transfer Tool （文件传输工具）
> This project is a simple file transfer tool, written in Golang 1.6. **If you have any problems/ideas, please [email me](mailto:forec@bupt.edu.cn), or open your PR. I feel honored to learn from your help** .

## Platform
There are two files `server.go` and `client.go`, you should put them under different paths. To build the executable files, you need to configure the Golang  properly first. The [install guide](https://github.com/Forec/the-way-to-go_ZH_CN/blob/master/eBook/02.2.md) may help you.

## Usage
* Both `server.exe` and `client.exe` have help guide, you can use `--help` to see the parameters.
* `server.exe` has two parameters, host and port, which assigns where the server listening. You need to assign a password for you server after executing it.
* `client.exe` has two parameters, host and port, point where the client should connect. You need to input password to get authorisation after connection.

## Support Commands
version v0.1 supports the following commands temporarily. The buffer size is set to 4MB, you can resize it. The clocker for sending/receiving files is 1e3 ns, you can change it to influence the speed of transmitting. The current transmitting speed is about 2.5M/s in local machines.
* cd: change current path
* ls or dir: get file info
* put: put local file to remote sever
* get: get file from remote server
* pwd: get current path

## Update-logs
* 2016-9-15: Add project.
* 2016-10-18: Finish basic functions, version v0.1. Build repository.
* 2016-10-19: Update recv function.
* 2016-10-31：Fix help message in server and client.

# License
All codes in this repository are licensed under the terms you may find in the file named "LICENSE" in this directory.