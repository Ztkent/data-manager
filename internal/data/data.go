package data

import (
	"log"
	"os/exec"
)

func StartProcessor() {
	go func() {
		cmd := exec.Command("python3", "pkg/data-processor/data_processor.py", "--database", "pkg/data-crawler/results.db")
		err := cmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}()
}
