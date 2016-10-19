/*
author: Forec
last edit date: 2016/10/19
email: forec@bupt.edu.cn
LICENSE
Copyright (c) 2015-2017, Forec <forec@bupt.edu.cn>

Permission to use, copy, modify, and/or distribute this code for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
*/


package main

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var address_h = flag.String("h", "127.0.0.1",
	"Bind server with assigned IP address, default 127.0.0.1")
var port_p = flag.Int("p", 8080,
	"Bind server with assigned port, default 8080")
var buflen = 4096 * 1024

func dealWithArgs() (ip string, port int, ok bool) {
	ip = *address_h
	port = *port_p
	ip_ok, _ := regexp.MatchString(
		"^(25[0-5]|2[0-4]\\d|[0-1]?\\d?\\d)(\\.(25[0-5]|2[0-4]\\d|[0-1]?\\d?\\d)){3}$", ip)
	if !ip_ok {
		fmt.Println("Invalid IPv4 Address")
		flag.PrintDefaults()
		return "", 0, false
	}
	if 0 >= port || port > 65535 {
		fmt.Println("Invalid Port Value")
		flag.PrintDefaults()
		return "", 0, false
	}
	return ip, port, true
}

func getFileSize(path string) (size int64, err error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return -1, err
	}
	fileSize := fileInfo.Size()
	return fileSize, nil
}

func safeRecv(conn net.Conn, buf []byte) []byte {
	ans := ""
	length, err := conn.Read(buf)
	if err != nil {
		fmt.Println("ERROR: Connection Error.")
		return nil
	}
	totalRecvLength, err := strconv.Atoi(string(buf[7:length]))
	if err != nil {
		fmt.Println("ERROR: Header Transmission Error.")
		return nil
	}
	for {
		length, err = conn.Read(buf)
		if err != nil {
			fmt.Println("ERROR: Connection Error.")
			return nil
		}
		ans = ans + string(buf[:length])
		totalRecvLength -= length
		if totalRecvLength == 0 {
			break
		}
	}
	return []byte(ans)
}

func getDir(conn net.Conn, buf []byte) bool {
	_, err := conn.Write([]byte("ls"))
	if err != nil {
		fmt.Println("ERROR: Sending Request Error.")
		return false
	}
	fmt.Println(string(safeRecv(conn, buf)))
	return true
}

func getPWD(conn net.Conn, buf []byte) bool {
	_, err := conn.Write([]byte("pwd"))
	if err != nil {
		fmt.Println("ERROR: Sending Request Error.")
		return false
	}
	length, err := conn.Read(buf)
	if err != nil {
		fmt.Println("ERROR: Connection Error.")
		return false
	}
	fmt.Println(string(buf[:length]))
	return true
}

func chDir(conn net.Conn, command string, buf []byte) (ok bool) {
	if len(command) > buflen {
		command = command[:buflen]
	}
	_, err := conn.Write([]byte(command))
	if err != nil {
		fmt.Println("ERROR: Sending Request Error.")
		return false
	}
	_, err = conn.Read(buf)
	if err != nil {
		fmt.Println("ERROR: Connection Error.")
		return false
	}
	if string(buf[:7]) == "SUCCEED" {
		return true
	} else {
		fmt.Println("ERROR: Server Internal Error.")
		return false
	}
}

func sendFile(conn net.Conn, command string, buf []byte) (ok bool) {
	filename := string(command[4:])
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("ERROR: File Not Exist Or Cannot Open.")
		return false
	}
	fileReader := bufio.NewReader(file)
	totalFileLength, err := getFileSize(filename)
	if err != nil {
		fmt.Println("ERROR: Cannot Get File Info.")
		return false
	}
	defer file.Close()
	_, err1 := conn.Write([]byte(fmt.Sprintf("PUT %s", filename)))
	_, err2 := conn.Write([]byte(fmt.Sprintf("%d", totalFileLength)))
	if err1 != nil || err2 != nil {
		fmt.Println("ERROR: Sending Request Error.")
		return false
	}
	chRate := time.Tick(2e3)
	for {
		<-chRate
		length, err := fileReader.Read(buf)
		if err != nil {
			return false
		}
		if length == 0 {
			return true
		}
		_, err = conn.Write([]byte(buf[:length]))
		if err != nil {
			return false
		}
	}
}

func recvFile(conn net.Conn, command string, buf []byte) (ok bool) {
	filename := strings.TrimLeft(string(command[4:]), " ")
	length, err := conn.Write([]byte(command))
	if err != nil {
		fmt.Println("ERROR: Sending Request Error.")
		return false
	}
	length, err = conn.Read(buf)
	if err != nil {
		fmt.Println("ERROR: Connection Error.")
		return false
	}
	if string(buf[:7]) != "SUCCEED" {
		fmt.Printf("ERROR: No file named %v in server.\n", filename)
		return false
	}
	totalFileLength, err := strconv.Atoi(string(buf[7:length]))
	if err != nil {
		fmt.Println("ERROR: Header Transmission Error.")
		return false
	}
	outputFile, err := os.OpenFile(filename,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		fmt.Println("ERROR: File Write Error.")
		return false
	}
	defer outputFile.Close()
	outputWriter := bufio.NewWriter(outputFile)
	chRate := time.Tick(1e3)
	percent := 0
	fileLength := 0
	for {
		<-chRate
		length, err = conn.Read(buf)
		if err != nil {
			fmt.Println("ERROR: Transmission Error.")
			return false
		}
		outputLength, outputError := outputWriter.Write(buf[:length])
		if outputError != nil || outputLength != length {
			fmt.Println("ERROR: File Write Error.")
			return false
		}
		fileLength = fileLength + length
		if 100*fileLength/totalFileLength > percent {
			percent = 100 * fileLength / totalFileLength
			fmt.Printf("Received: %v%%...\n", percent)
		}
		if fileLength == totalFileLength {
			outputWriter.Flush()
			fmt.Println("File Transimission Complete.")
			return true
		}
	}
}

func dealWithInputCommand(inputReader *bufio.Reader) (command string, ok bool) {
	fullCommand := ""
	for {
		input, err := inputReader.ReadString('\n')
		if err != nil {
			fmt.Println("ERROR: Failed to get your command.\n")
			return "", false
		}
		input = strings.Trim(input, "\r\n")
		fullCommand += input
		if input[len(input)-1] != '\\' {
			return fullCommand, true
		}
	}
}

func main() {
	flag.Parse()
	ip, port, ok := dealWithArgs()
	if !ok {
		return
	}
	fmt.Println("Building connection...")
	conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		fmt.Println("ERROR: Error dialing", err.Error())
		return
	}
	//conn.SetDeadline(time.Now().Add(3 * time.Second))
	inputReader := bufio.NewReader(os.Stdin)
	fmt.Printf("Input password please: ")
	var password string
	for {
		password, err = inputReader.ReadString('\n')
		if err == nil {
			break
		}
		fmt.Println("ERROR: Input Error. Please Input Again.")
	}
	fmt.Println("Verifying password...")
	password = strings.Trim(password, "\r\n")
	chRate := time.Tick(1e9)
	tryTimes := 0
	for {
		_, err = conn.Write([]byte(password[:len(password)-1]))
		if err == nil {
			break
		} else if tryTimes > 5 {
			fmt.Println("ERROR: Cannot contact with remote server, break down...")
			return
		}
		<-chRate
		tryTimes++
	}
	buf := make([]byte, 4096)
	length, err := conn.Read(buf)
	if err != nil {
		fmt.Println("ERROR: Cannot Receive Authentication From Server, break down...")
		return
	}
	if !strings.EqualFold(string(buf[:length]), "CONNECTION SUCCEED") {
		fmt.Println("ERROR: Password Not Correct! break down...")
		return
	}
	fmt.Println("Receive Authentication...")
LOOP:
	for {
		fmt.Print("> ")
		input, ok := dealWithInputCommand(inputReader)
		switch {
		case !ok:
			continue LOOP
		case strings.ToUpper(input) == "QUIT":
			fallthrough
		case strings.ToUpper(input) == "BYE":
			fallthrough
		case strings.ToUpper(input) == "EXIT":
			conn.Close()
			return

		case len(input) > 2 && strings.ToUpper(input[:2]) == "CD":
			chDir(conn, input, buf)
		case strings.ToUpper(input) == "LS" || strings.ToUpper(input) == "DIR":
			getDir(conn, buf)
		case strings.ToUpper(input) == "PWD":
			getPWD(conn, buf)
		case strings.ToUpper(input) == "TRY":
			conn.Write([]byte("TRY"))
		case len(input) > 3 && strings.ToUpper(input[:3]) == "GET":
			recvFile(conn, input, buf)
		case len(input) > 3 && strings.ToUpper(input[:3]) == "PUT":
			sendFile(conn, input, buf)
		case len(input) > 6 && strings.ToUpper(input[:6]) == "DELETE":
			//deleteFile(conn, input, buf)

		default:
			fmt.Println("ERROR: Invalid Command.")
		}
	}
}
