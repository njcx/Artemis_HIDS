package yara

import (
	"artemis_hids/utils/log"
	"github.com/Velocidex/go-yara"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func TestRule(rulesPath string, debug bool) (ruleFiles []string) {
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		panic(err)
	}
	rulesStat, _ := os.Stat(rulesPath)
	switch mode := rulesStat.Mode(); {
	case mode.IsDir():
		filepath.Walk(rulesPath, func(filePath string, fileObj os.FileInfo, err error) error {
			fileName := fileObj.Name()
			if (path.Ext(fileName) == ".yar") || (path.Ext(fileName) == ".yara") {
				rulesObj, err := os.Open(filePath)
				defer rulesObj.Close()
				if err == nil {
					compiler, err := yara.NewCompiler()
					if err != nil {
						log.Log.Panic(err)
					}
					errRet := compiler.AddFile(rulesObj, "")
					if errRet == nil {
						ruleFiles = append(ruleFiles, filePath)
					} else {
						if debug {
							log.Log.Debugf("invalid rule file: %v, detail: %v", filePath, errRet)
						}
					}
				}
			}
			return nil
		})
	case mode.IsRegular():
		rulesObj, err := os.Open(rulesPath)
		defer rulesObj.Close()
		if err == nil {
			compiler, err := yara.NewCompiler()
			if err != nil {
				log.Log.Panic(err)
			}
			errRet := compiler.AddFile(rulesObj, "")
			if errRet == nil {
				ruleFiles = append(ruleFiles, rulesPath)
			} else {
				if debug {
					log.Log.Debugf("invalid rule file: %v, detail: %v", rulesPath, errRet)
				}
			}
		}
	}
	return ruleFiles
}

func InitRule(rulePath string, debug bool) error {
	files := TestRule(rulePath, debug)
	return initRule(files, debug)
}

func initRule(ruleFiles []string, debug bool) error {
	compiler, err := yara.NewCompiler()
	if err != nil {
		log.Log.Panic(err)
	}
	for _, rulePath := range ruleFiles {
		if strings.Contains(rulePath, "index") || strings.Contains(rulePath, "util") {
			continue
		}
		rulesObj, err := os.Open(rulePath)
		defer rulesObj.Close()
		if err == nil {
			paths := strings.Split(rulePath, "/")
			namespace := paths[len(paths)-2]
			err := compiler.AddFile(rulesObj, namespace)
			if debug {
				log.Log.Printf("Compiling Yara rule: %v, result: %v", rulePath, err)
			}
		}
	}
	rules, err := compiler.GetRules()
	if err == nil {
		rules.Save("rules.db")
	}
	total := len(rules.GetRules())
	log.Log.Printf("Init rules Done, total: %v rules, err: %v", total, err)
	return err
}
