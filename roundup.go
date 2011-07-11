package main

import (
	"exec"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
)

const (
	jLo = 1
	jHi = 16
)

var testCaseRegexp = regexp.MustCompile("it_[a-zA-Z0-9_]*")

type TaskResult struct {
	task   string
	err    os.Error
	output string
}

func printUsage() {
	fmt.Printf(`roundup [options] testplan

Options:

-jN - concurrency level. For example, -j4 means parallel execution with 4 threads.
`)
}

func execute(testPlan, task string) (res *TaskResult) {
	res = new(TaskResult)
	res.task = task
	cmd := exec.Command("/bin/sh", "-c", fmt.Sprintf("set -e; . %s; %s", testPlan, task))
	cmd.Dir, _ = path.Split(testPlan)
	data, err := cmd.CombinedOutput()
	res.err = err
	res.output = string(data)
	return
}

func worker(index int, testPlan string, taskCh chan string, resCh chan *TaskResult, doneCh chan bool) {
	for task := range taskCh {
		resCh <- execute(testPlan, task)
	}
	doneCh <- true
}

func resultWorker(resCh chan *TaskResult, doneResCh chan []*TaskResult) {
	var all []*TaskResult
	for r := range resCh {
		if r.err == nil {
			fmt.Printf("%s\tPASS\n", r.task)
		} else {
			fmt.Printf("%s\n", r.output)
			fmt.Printf("%s\tFAILED\n", r.task)
		}
		all = append(all, r)
	}
	doneResCh <- all
}

func summarize(res []*TaskResult) bool {
	passed := 0
	for _, r := range res {
		if r.err == nil {
			passed++
		}
	}
	fmt.Printf("\nTests: %3d | Passed: %3d | Failed: %3d |\n", len(res), passed, len(res)-passed)
	return passed == len(res)
}

func main() {
	if len(os.Args) == 1 {
		printUsage()
		os.Exit(1)
	}
	var j = jLo
	if strings.HasPrefix(os.Args[1], "-j") {
		var err os.Error
		j, err = strconv.Atoi(os.Args[1][2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "roundup: Couldn't parse -j option. "+
				"Valid values are like -j2, -j4, -j16, etc\n")
			os.Exit(1)
		}
		if len(os.Args) == 2 {
			printUsage()
			os.Exit(1)
		}
	}
	if j < jLo || j > jHi {
		fmt.Fprintf(os.Stderr, "roundup: -j%d is invalid option. "+
			"Concurrency level from %d to %d are supported", j, jLo, jHi)
		os.Exit(1)
	}
	testPlan := os.Args[len(os.Args)-1]
	data, err := ioutil.ReadFile(testPlan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "roundup: couldn't read %s: %v", testPlan, err)
		os.Exit(1)
	}
	taskCh := make(chan string)
	resCh := make(chan *TaskResult)
	doneCh := make(chan bool)
	doneResCh := make(chan []*TaskResult)
	for i := 0; i < j; i++ {
		go worker(i, testPlan, taskCh, resCh, doneCh)
	}
	go resultWorker(resCh, doneResCh)
	for _, line := range strings.Split(string(data), "\n") {
		m := testCaseRegexp.FindString(line)
		if m != "" {
			taskCh <- m
		}
	}
	close(taskCh)
	for i := 0; i < j; i++ {
		<-doneCh
	}
	close(doneCh)
	close(resCh)
	res := <-doneResCh
	close(doneResCh)
	if !summarize(res) {
		os.Exit(2)
	}
}
