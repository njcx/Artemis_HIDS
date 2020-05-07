package scanner

import (
	"github.com/hillu/go-yara"
	"github.com/toolkits/slice"

	"sec_check/logger"
	"sec_check/lib"
	"sec_check/collector"
	"sec_check/vars"
	"sec_check/models"

	"os"
	"sync"
	"time"
	"strings"
)

type Scanner struct {
	Rules *yara.Rules
}

func NewScanner(rulesData string) (*Scanner, error) {
	rules, err := LoadRules(rulesData)
	return &Scanner{Rules: rules}, err
}

func LoadRules(rulesData string) (*yara.Rules, error) {
	rules, err := yara.LoadRules(rulesData)
	return rules, err
}

func (s *Scanner) ScanFile(filename string) (error, *models.FileScanResult) {
	if vars.Verbose {
		logger.Log.Debugf("checking file: %v", filename)
	}
	matches, err := s.Rules.ScanFile(filename, 0, 10)
	result := &models.FileScanResult{FileName: filename, Matches: matches}
	return err, result
}

func (s *Scanner) ScanFiles(filename string) {
	files, err := lib.GetFiles(filename)
	if err == nil {
		//var wg sync.WaitGroup
		// wg.Add(len(files))
		// go-yara不是协程安全的，并发模式不可用，改为普通的循环
		for _, f := range files {
			models.SaveFileResult(s.ScanFile(f))
			//wg.Add(1)
			//go func(filename string) {
			//	defer wg.Done()
			//	models.SaveFileResult(s.ScanFile(filename))
			//}(f)
			//waitTimeout(&wg, 60)
		}
	}
}

func (s *Scanner) ScanProcess(pid int) (error, *models.ProcessScanResult) {
	if vars.Verbose {
		logger.Log.Debugf("checking pid: %v", pid)
	}
	matches, err := s.Rules.ScanProc(pid, 0, 10)
	result := &models.ProcessScanResult{Pid: pid, Matches: matches}
	return err, result
}

func (s *Scanner) ScanProcesses() {
	pss := collector.GetProcess()
	//var wg sync.WaitGroup
	// wg.Add(len(pss))
	// go-yara不是协程安全的，并发模式不可用，改为普通的循环
	for _, ps := range pss.Process {
		//wg.Add(1)
		pid := os.Getpid()
		if pid == ps.PPID {
			//wg.Done()
			continue
		}
		t := strings.Split(ps.Path, "/")
		tt := t[len(t)-1]
		whiteList := []string{"python", "python2.7", "ruby", "sagent", "crond", "mysqld", "rsyslogd"}
		if !slice.ContainsString(whiteList, tt) {
			models.SaveProcessResult(s.ScanProcess(ps.PID))
		}

		//go func(pid int) {
		//	defer wg.Done()
		//	models.SaveProcessResult(s.ScanProcess(pid))
		//}(ps.PID)
		//waitTimeout(&wg, 60)
	}

}

// waitTimeout waits for the waitgroup for the specified max timeout.
// Returns true if waiting timed out.
func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}
