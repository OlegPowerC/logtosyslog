package main

import (
	"encoding/json"
	"fmt"
	"github.com/howeyc/fsnotify"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

//Описание JSON параметров
type params struct {
	Syslogserver string `json:"Syslogserver"`
	Syslogport   int    `json:"syslogport"`
	Filename     string `json:"filename"`
	Severity     int    `json:"Severity"`
	Facility     int    `json:"Facility"`
	Debugmode    int    `json:"debugg"`
}

type alarm struct {
	Name  string `json:"name"`
	Error int    `json:"error"`
}

var debugmode bool = false

func SendMessageToSyslogServer(Data string, Severity int, Facility int, ServerAddrAdndPort string) (sentbytes int, err error) {
	if Severity > 7 {
		return 0, fmt.Errorf("Invalid severity %d", Severity)
	}
	if Facility > 23 {
		return 0, fmt.Errorf("Invalid severity %d", Facility)
	}

	raddr, errresolv := net.ResolveUDPAddr("udp", ServerAddrAdndPort)
	if errresolv != nil {
		return 0, errresolv
	}

	var Priority int
	Priority = Facility*8 + Severity
	SyslogMessage := fmt.Sprintf("<%d> %s", Priority, Data)

	conn, err := net.DialUDP("udp", nil, raddr)
	defer conn.Close()
	if err != nil {
		return 0, err
	}
	var fmb []byte
	fmb = []byte(SyslogMessage)
	sentbytes, errsend := conn.Write(fmb)
	if errsend != nil {
		return 0, errsend
	}
	return sentbytes, nil
}

func waitfsevent(watcher *fsnotify.Watcher, fname string, SyslogServerFullUrl string, Severity int, Facility int, wg *sync.WaitGroup, debugmode bool) {
	defer wg.Done()
	var prevsize int64 = 0
	fis, err := os.Stat(fname)
	if err != nil {
		log.Fatal(err)
	}
	prevsize = fis.Size()

	for {
		select {
		case ev := <-watcher.Event:
			if ev.IsModify() {
				f, _ := os.Open(fname)
				f.Seek(prevsize, 0)
				fi, err := os.Stat(fname)
				if err != nil {
					log.Fatal(err)
				}
				// get the size
				size := fi.Size()
				if size > prevsize {
					newdata_len := size - prevsize
					buff := make([]byte, newdata_len)
					f.Read(buff)

					prevsize = size

					strfind := string(buff)
					strfind = strings.TrimLeft(strfind, "\r\n")
					strfind = strings.TrimLeft(strfind, "\n")
					if debugmode {
						fmt.Print(strfind)
					}
					strfind = strings.TrimRight(strfind, "\r\n")
					strfind = strings.TrimRight(strfind, "\n")
					if len(SyslogServerFullUrl) > 4 {
						sentbytes, senderror := SendMessageToSyslogServer(strfind, Severity, Facility, SyslogServerFullUrl)
						if senderror != nil {
							fmt.Println(senderror)
						} else {
							if debugmode {
								fmt.Println("sent:", sentbytes, "bytes")
							}
						}
					}
				}
			}
		case err := <-watcher.Error:
			log.Println("error:", err)
		}
	}
}

const JsonFileName = "params.json"

func main() {
	ossigch := make(chan os.Signal)
	signal.Notify(ossigch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ossigch
		fmt.Println("Terminate")
		os.Exit(1)
	}()
	var wg sync.WaitGroup
	var JParams params
	// Открываем файл с настройками
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	jSettingsFile, err := os.Open(exPath + "/" + JsonFileName)
	// Проверяем на ошибки
	if err != nil {
		fmt.Println("Ошибка:", err)
	}
	defer jSettingsFile.Close()

	FData, err := ioutil.ReadAll(jSettingsFile)
	if err != nil {
		fmt.Println("Ошибка:", err)
	}
	json.Unmarshal(FData, &JParams)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	if JParams.Debugmode == 1 {
		debugmode = true
		fmt.Println("Enabled debugg mode")
	} else {
		fmt.Println("Disabled debugg mode")
	}

	SyslogServerFullURL := fmt.Sprintf("%s:%d", JParams.Syslogserver, JParams.Syslogport)

	wg.Add(1)
	fmt.Println(JParams.Filename)
	go waitfsevent(watcher, JParams.Filename, SyslogServerFullURL, JParams.Severity, JParams.Facility, &wg, debugmode)

	err = watcher.Watch(JParams.Filename)
	if err != nil {
		log.Fatal(err)
	}
	wg.Wait()
	watcher.Close()
}
