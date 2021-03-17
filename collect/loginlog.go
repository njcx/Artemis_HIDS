package collect

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

type utmp struct {
	UtType uint32
	UtPid  uint32    // PID of login process
	UtLine [32]byte  // device name of tty - "/dev/"
	UtID   [4]byte   // init id or abbrev. ttyname
	UtUser [32]byte  // user name
	UtHost [256]byte // hostname for remote login
	UtExit struct {
		ETermination uint16 // process termination status
		EExit        uint16 // process exit status
	}
	UtSession uint32 // Session ID, used for windowing
	UtTv      struct {
		TvSec  uint32 /* Seconds */
		TvUsec uint32 /* Microseconds */
	}
	UtAddrV6 [4]uint32 // IP address of remote host
	Unused   [20]byte  // Reserved for future use
}

func getLast(t string) (result []map[string]string) {
	var timestamp int64
	if t == "all" {
		timestamp = 615147123
	} else {
		ti, _ := time.Parse("2006-01-02T15:04:05Z07:00", t)
		timestamp = ti.Unix()
	}
	wtmpFile, err := os.Open("/var/log/wtmp")
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer wtmpFile.Close()
	for {
		wtmp := new(utmp)
		err = binary.Read(wtmpFile, binary.LittleEndian, wtmp)
		if err != nil {
			break
		}
		if wtmp.UtType == 7 && int64(wtmp.UtTv.TvSec) > timestamp {
			m := make(map[string]string)
			m["status"] = "true"
			m["remote"] = string(bytes.TrimRight(wtmp.UtHost[:], "\x00"))
			if m["remote"] == "" {
				continue
			}
			m["username"] = string(bytes.TrimRight(wtmp.UtUser[:], "\x00"))
			m["time"] = time.Unix(int64(wtmp.UtTv.TvSec), 0).Format("2006-01-02T15:04:05Z07:00")
			result = append(result, m)
		}
	}
	return result
}
func getLastb(t string) (result []map[string]string) {

	if len(t) ==0 {
		t="all"
	}
	var cmd string
	ti, _ := time.Parse("2006-01-02T15:04:05Z07:00", t)
	if t == "all" {
		cmd = "/usr/local/peppac/p-lastb --time-format iso -f /var/log/btmp"
	} else {
		cmd = fmt.Sprintf("/usr/local/peppac/p-lastb -s %s --time-format iso -f /var/log/btmp",
			ti.Format("20060102150405"))
	}
	out := Cmdexec(cmd)
	logList := strings.Split(out, "\n")
	if len(logList) >=3 {
		s :=len(logList)
		if s ==3{
			s=s+1
		}
		for _, v := range logList[0 : s-3] {
			m := make(map[string]string)
			reg := regexp.MustCompile("\\s+")
			v = reg.ReplaceAllString(strings.TrimSpace(v), " ")
			s := strings.Split(v, " ")
			if len(s) < 4 {
				continue
			}
			m["status"] = "false"
			m["username"] = s[0]
			m["remote"] = s[2]
			t, _ := time.Parse("2006-01-02T15:04:05Z0700", s[3])
			m["time"] = t.Format("2006-01-02T15:04:05Z07:00")
			Config.Lasttime = t.Format("20060102150405")
			result = append(result, m)
		}
	}
	return
}
func GetLoginLog() (resultData []map[string]string) {
	resultData = getLast(Config.Lasttime)
	resultData = append(resultData, getLastb(Config.Lasttime)...)
	return
}
