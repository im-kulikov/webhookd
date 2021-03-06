package worker

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/ncarlier/webhookd/pkg/tools"
)

// ChanWriter is a simple writer to a channel of byte.
type ChanWriter struct {
	ByteChan chan []byte
}

func (c *ChanWriter) Write(p []byte) (int, error) {
	c.ByteChan <- p
	return len(p), nil
}

var (
	workingdir = os.Getenv("APP_WORKING_DIR")
)

func runScript(work *WorkRequest) (string, error) {
	if workingdir == "" {
		workingdir = os.TempDir()
	}

	log.Println("Starting script:", work.Script, "...")
	binary, err := exec.LookPath(work.Script)
	if err != nil {
		return "", err
	}

	// Exec script with args...
	cmd := exec.Command(binary, work.Payload)
	// with env variables...
	cmd.Env = append(os.Environ(), work.Args...)

	// Open the out file for writing
	logFilename := path.Join(workingdir, fmt.Sprintf("%s_%s.txt", tools.ToSnakeCase(work.Name), time.Now().Format("20060102_1504")))
	logFile, err := os.Create(logFilename)
	if err != nil {
		return "", err
	}
	defer logFile.Close()
	log.Println("Writing output to file: ", logFilename, "...")

	wLogFile := bufio.NewWriter(logFile)

	r, w := io.Pipe()
	cmd.Stdout = w
	cmd.Stderr = w

	// Start the script...
	err = cmd.Start()
	if err != nil {
		return logFilename, err
	}

	// Write script output to log file and the work message cahnnel.
	go func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			// writing to the work channel
			line := scanner.Text()
			work.MessageChan <- []byte(line)
			// writing to outfile
			if _, err := wLogFile.WriteString(line + "\n"); err != nil {
				log.Println("Error while writing into the log file:", logFilename, err)
			}
			if err = wLogFile.Flush(); err != nil {
				log.Println("Error while flushing the log file:", logFilename, err)
			}
		}
		if err := scanner.Err(); err != nil {
			log.Println("Error scanning the script stdout: ", logFilename, err)
		}
	}(r)

	err = cmd.Wait()
	if err != nil {
		log.Println("Starting script:", work.Script, "-> ERROR")
		return logFilename, err
	}
	log.Println("Starting script:", work.Script, "-> OK")
	return logFilename, nil
}
