package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var address_h = flag.String("h", "127.0.0.1",
	"Bind server with assigned IP address, default 127.0.0.1")
var port_p = flag.Int("p", 8080,
	"Bind server with assigned port, default 8080")
var password string
var buflen = 4096 * 1024

func getCurrentDirectory() (string, bool) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
		return "", false
	}
	return strings.Replace(dir, "\\", "/", -1), true
}

func listDir(path string) (info string, ok bool) {
	dir, e := ioutil.ReadDir(path)
	if e != nil {
		return "", false
	}
	info = "ID\tFilename\tMode\tSize(byte)\n"
	for i, v := range dir {
		info += fmt.Sprintf("%d\t%s\t%s\t\t%d\n",
			i, v.Name(), v.Mode().String(), v.Size())
	}
	return info, true
}

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

func sendFile(conn net.Conn, filename string, buf []byte) (ok bool) {
	file, err := os.Open(filename)
	if err != nil {
		conn.Write([]byte("FAILED"))
		return false
	}
	fileReader := bufio.NewReader(file)
	totalFileLength, err := getFileSize(filename)
	if err != nil {
		conn.Write([]byte("FAILED"))
		return false
	}
	defer file.Close()
	_, err = conn.Write([]byte(fmt.Sprintf("SUCCEED%d", totalFileLength)))
	if err != nil {
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

func recvFile(conn net.Conn, filename string, buf []byte) (ok bool) {
	length, err := conn.Read(buf)
	if err != nil {
		fmt.Println("ERROR: Connection Error.")
		return false
	}
	totalFileLength, err := strconv.Atoi(string(buf[:length]))
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
		if length == buflen {
			outputLength, outputError := outputWriter.Write(buf)
			if outputError != nil || outputLength != length {
				fmt.Println("ERROR: File Write Error.")
				return false
			}
		} else {
			for i := 0; i < length; i++ {
				outputError := outputWriter.WriteByte(buf[i])
				if outputError != nil {
					fmt.Println("ERROR: File Write Error.")
					return false
				}
			}
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

func doFTPService(conn net.Conn) {
	curPath, suc := getCurrentDirectory()
	if !suc {
		conn.Write([]byte("SERVER INTERNAL ERROR"))
		return
	}
	buf := make([]byte, 4096)
	length, err := conn.Read(buf)
	if err != nil {
		fmt.Println("Error Verifying Client", err.Error())
		return
	}
	if strings.Compare(string(buf[:length]), password) == 0 {
		fmt.Printf("Remote Client %s Verifying Failed.\n",
			conn.RemoteAddr().String())
		return
	}
	fmt.Printf("Remote Client %s Verifying Succeed.\n",
		conn.RemoteAddr().String())
	_, err = conn.Write([]byte("CONNECTION SUCCEED"))
	if err != nil {
		return
	}
LOOP:
	for {
		length, err = conn.Read(buf)
		if err != nil {
			fmt.Println("Error Reading", err.Error())
			return
		}
		receiveCommand := string(buf[:length])
		fmt.Printf("Receving: %v\n", receiveCommand)
		switch {
		// get method
		case len(receiveCommand) > 3 && strings.ToUpper(receiveCommand[:3]) == "GET":
			filename := strings.TrimLeft(string(receiveCommand[4:]), " ")
			fmt.Printf("Transfering %v\n", filename)
			sendFile(conn, filename, buf)

		// put method
		case len(receiveCommand) > 3 && strings.ToUpper(receiveCommand[:3]) == "PUT":
			filename := strings.TrimLeft(string(receiveCommand[4:]), " ")
			fmt.Printf("Receiving %v\n", filename)
			recvFile(conn, filename, buf)

		// ls or dir method
		case strings.ToUpper(receiveCommand) == "LS" ||
			strings.ToUpper(receiveCommand) == "DIR":
			ans, suc := listDir(curPath)
			_, err := conn.Write([]byte(fmt.Sprintf("SUCCEED%d", len(ans))))
			if !suc || err != nil {
				conn.Write([]byte("SERVER INTERNAL ERROR"))
				continue LOOP
			}
			conn.Write([]byte(ans))

		// cd method
		case len(receiveCommand) > 2 && strings.ToUpper(receiveCommand[:2]) == "CD":
			newpath := strings.TrimLeft(string(receiveCommand[3:]), " ")
			err = os.Chdir(newpath)
			curPath, suc = getCurrentDirectory()
			if (!suc) || (err != nil) {
				if err != nil {
					conn.Write([]byte(err.Error()))
				} else {
					conn.Write([]byte("SERVER INTERNAL ERROR"))
				}
				continue LOOP
			}
			conn.Write([]byte("SUCCEED"))

		// pwd method
		case strings.ToUpper(receiveCommand) == "PWD":
			conn.Write([]byte(curPath))

		// test method
		case strings.ToUpper(receiveCommand) == "TRY":
			continue LOOP
		default:
			continue LOOP
		}
	}
}

func main() {
	flag.Parse()
	ip, port, ok := dealWithArgs()
	if !ok {
		return
	}
	fmt.Printf("Server starting at %s:%d ...\n", ip, port)
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		fmt.Println("Server starting with an error, break down...")
		return
	}
	fmt.Println("Please set a password for connections")
	fmt.Scanln(&password)
	fmt.Println("Password has been set, server is running...")
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting", err.Error())
			continue
		}
		fmt.Println("Rececive connection request from",
			conn.RemoteAddr().String())
		go doFTPService(conn)
	}
}
