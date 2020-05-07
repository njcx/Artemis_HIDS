package log


//import (
//	"io"
//	"log"
//	"os"
//)
//
//var (
//	Info *log.Logger
//	Warning *log.Logger
//	Error * log.Logger
//)
//
//func init(){
//	errFile,err:=os.OpenFile("errors.log",os.O_CREATE|os.O_WRONLY|os.O_APPEND,0666)
//	if err!=nil{
//		log.Fatalln("打开日志文件失败：",err)
//	}
//
//	Info = log.New(io.MultiWriter(os.Stdout,errFile),"Info:",log.Ldate | log.Ltime | log.Lshortfile)
//	Warning = log.New(io.MultiWriter(os.Stdout,errFile),"Warning:",log.Ldate | log.Ltime | log.Lshortfile)
//	Error = log.New(io.MultiWriter(os.Stderr,errFile),"Error:",log.Ldate | log.Ltime | log.Lshortfile)
//
//}



import (
	"github.com/sirupsen/logrus"
	"github.com/x-cray/logrus-prefixed-formatter"
)

var (
	Log *logrus.Entry
)

func init() {
	logger := logrus.New()
	logger.Formatter = new(prefixed.TextFormatter)
	logger.Level = logrus.DebugLevel
	Log = logger.WithFields(logrus.Fields{"prefix": "xsec checker"})
}
