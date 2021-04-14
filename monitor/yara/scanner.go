package yara

import (
	"github.com/Velocidex/go-yara"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/toolkits/slice"

	"os"
	"path/filepath"
	"peppa_hids/utils/log"
	"sec_check/collector"
	"sec_check/models"
	"strings"
	"sync"
	"time"

)

var (
	Debug    bool
	Verbose  bool
	RulePath = "rules"
	RulesDb  = "rules.db"

	Addr = "127.0.0.1"
	Port = 8000

	ProcessResultMap = cmap.New()
	FileResultMap    = cmap.New()

	CurrentDir = ""
)

type Scanner struct {
	Rules *yara.Rules
}

func GetFiles(filePath string) (Files []string, err error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return Files, err
	}
	rulesStat, _ := os.Stat(filePath)
	switch mode := rulesStat.Mode(); {
	case mode.IsDir():
		err = filepath.Walk(filePath, func(filePath string, fileObj os.FileInfo, err error) error {
			rulesObj, err := os.Open(filePath)
			defer rulesObj.Close()
			if err == nil {
				Files = append(Files, filePath)
			}
			return nil
		})
	case mode.IsRegular():
		rulesObj, err := os.Open(filePath)
		defer rulesObj.Close()
		if err == nil {
			Files = append(Files, filePath)
		}
	}
	return Files, err
}

func NewScanner(rulesData string) (*Scanner, error) {
	rules, err := LoadRules(rulesData)
	return &Scanner{Rules: rules}, err
}

func LoadRules(rulesData string) (*yara.Rules, error) {
	rules, err := yara.LoadRules(rulesData)
	return rules, err
}

func (s *Scanner) ScanFile(filename string) (error, *FileScanResult) {
	if Verbose {
		log.Log.Debugf("checking file: %v", filename)
	}
	matches, err := s.Rules.ScanFile(filename, 0, 10)
	result := &FileScanResult{FileName: filename, Matches: matches}
	return err, result
}

func (s *Scanner) ScanFiles(filename string) {
	files, err := GetFiles(filename)
	if err == nil {
		for _, f := range files {
			models.SaveFileResult(s.ScanFile(f))
		}
	}
}

func (s *Scanner) ScanProcess(pid int) (error, *models.ProcessScanResult) {
	if Verbose {
		log.Log.Debugf("checking pid: %v", pid)
	}
	matches, err := s.Rules.ScanProc(pid, 0, 10)
	result := &models.ProcessScanResult{Pid: pid, Matches: matches}
	return err, result
}

func (s *Scanner) ScanProcesses() {
	pss := collector.GetProcess()
	for _, ps := range pss.Process {
		pid := os.Getpid()
		if pid == ps.PPID {
			continue
		}
		t := strings.Split(ps.Path, "/")
		tt := t[len(t)-1]
		whiteList := []string{"python", "python2.7", "ruby", "sagent", "crond", "mysqld", "rsyslogd"}
		if !slice.ContainsString(whiteList, tt) {
			models.SaveProcessResult(s.ScanProcess(ps.PID))
		}
	}

}

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
