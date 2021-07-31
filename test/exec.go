package main

import (
	"artemis_hids/utils/gonlconnector"
	"log"
	"os"
)

func main() {

	watcher, err := gonlconnector.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			select {
			case ev := <-watcher.Exec:
				log.Println("exec event:", ev)
			}
		}
	}()

	err = watcher.Watch(os.Getpid(), gonlconnector.PROC_EVENT_ALL)
	if err != nil {
		log.Fatal(err)
	}
	watcher.Close()

}
