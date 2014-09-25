package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
  "http/client"

	"github.com/flynn/flynn/discoverd/client"
)

var dataDir = flag.String("data", "/data", "elasticsearch data directory")
var serviceName = flag.String("service", "elasticsearch", "discoverd service name")
var addr = ":" + os.Getenv("PORT")

func init() {
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)
}

func main() {
	flag.Parse()

	cmd, err := startMongod()
	if err != nil {
		log.Fatal(err)
	}
  
	sess := waitForElasticsearch(time.Minute)

	fatal := func(err error) {
		discoverd.UnregisterAll()
		cmd.Process.Signal(os.Interrupt)
		log.Fatal(err)
	}

	set, err := discoverd.RegisterWithSet(*serviceName, addr, nil)
  
	if err != nil {
		fatal(err)
	}

	log.Println("Registered with service discovery.")
	var self *discoverd.Service
	leaders := set.Leaders()
	for l := range leaders {
		if l.Addr == set.SelfAddr() {
			go func() {
				for _ = range leaders {
				}
			}()
			self = l
			break
		}
	}
}

func waitExit(cmd *exec.Cmd) {
	cmd.Wait()
	discoverd.UnregisterAll()
	var status int
	if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
		status = ws.ExitStatus()
	}
	os.Exit(status)
}

func buildMembers(self *discoverd.Service, services []*discoverd.Service) []replMember {
	res := make([]replMember, len(services)+1)
	res[0].Host = self.Addr
	for i, s := range services {
		res[i+1].ID = uint8(i + 1)
		res[i+1].Host = s.Addr
	}
	return res
}


func waitForElasticsearch(maxWait time.Duration) bool {
	log.Println("Waiting for elasticsearch to boot...")
  
	start := time.Now()
  
	for {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%s/_status", os.Getenv("PORT")))
		if err != nil {
			if time.Now().Sub(start) >= maxWait {
				log.Fatalf("Unable to connect to elasticsearch after %s, last error: %q", maxWait, err)
        return false
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
	}
  return true
}

func startElasticsearch() (*exec.Cmd, error) {
	log.Println("Starting elasticsearch...")

	cmd := exec.Command(
		"elasticsearch",
		"--dbpath", *dataDir,
		"--port", os.Getenv("PORT"),
		"--replSet", "rs0",
		"--noauth",
		"-v",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go handleSignals(cmd)
	go waitExit(cmd)
	return cmd, nil
}

func handleSignals(cmd *exec.Cmd) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	sig := <-c
	discoverd.UnregisterAll()
	cmd.Process.Signal(sig)
}
