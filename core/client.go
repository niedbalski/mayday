package core

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"sync"
)

type Client struct {
	Hostname    string
	ReportsPath string
	APIClient   *APIClient
}

func NewClient(server string, uuid string, authToken string) (*Client, error) {
	reportsPath, err := GetDefaultReportsDirectory()
	if err != nil {
		return nil, err
	}

	return &Client{
		ReportsPath: reportsPath,
		APIClient:   NewAPIClient(server, uuid, authToken),
	}, nil
}

func (client *Client) Create() (interface{}, error) {
	new_case, err := client.APIClient.CreateCase()
	if err != nil {
		return "", fmt.Errorf("Error creating new case on server: %s", err)
	}

	return new_case, nil
}

func (client *Client) Show() (string, string, error) {
	apiConfig, err := client.APIClient.GetConfig()
	if err != nil {
		return "", "", fmt.Errorf("Error getting configuration from server: %s", err)
	}

	return client.APIClient.UUID, apiConfig.GetRawDecoded(), nil
}

func (client *Client) Run(pgp bool, upload bool, timeout int, dryRun bool) error {
	reportPath, err := GetTempReportDirectory()

	if err != nil {
		return err
	}

	apiConfig, err := client.APIClient.GetConfig()
	if err != nil {
		return fmt.Errorf("Error getting configuration from server: %s", err)
	}

	config, err := NewConfig(apiConfig.GetRawDecoded(), apiConfig.GetSignedDecoded())
	if err != nil {
		return err
	}

	if pgp {
		err := config.CheckPGPSignature()
		if err != nil {
			return err
		}
		answer := PromptPGPConfirmation(config)
		if answer != true {
			return fmt.Errorf("PGP key has not been accepted")
		}
	}

	wg := new(sync.WaitGroup)
	log.Printf("Starting a new report on: %s", reportPath)

	for _, command := range config.Commands {
		wg.Add(1)
		go client.RunCommand(reportPath, command, wg)
	}

	for _, file := range config.Files {
		finfo, err := os.Stat(file.Path)
		if err != nil {
			log.Printf("Cannot stat file:%s", file.Path)
		} else {
			log.Printf("Archiving file:%s", file.Path)
			if finfo.IsDir() {
				CopyDir(file.Path, path.Join(reportPath, file.Path))
			} else {
				CopyFile(file.Path, reportPath)
			}
		}
	}

	wg.Wait()
	return nil
}

func (m *Client) RunCommand(reportPath string, command Command, wg *sync.WaitGroup) {
	// cmd := exec.Command("/bin/bash", "-c", command.Executable)

	// if timeout == 0 {
	// 	if err := cmd.Start(); err != nil {

	// 	} else {
	// 		log.Printf("Running command: %s", command.Executable)
	// 	}
	// } else {
	// 	done := make(chan error)
	// 	go func() {
	// 		done <- cmd.Run()
	// 	}()

	// 	select {
	// 	case <-time.After(time.Duration(timeout) * time.Second):
	// 		if err := cmd.Process.kill(); err != nil {
	// 			log.Fatal("Cannot kill process: ", err)
	// 		}
	// 		<- done
	// 		log.Printf("Command:%s killed by timeout", command.Executable)
	// 	case err := <- done:

	// }

	// defer close(done)

	log.Printf("Running %s", command.Executable)
	ran, _ := exec.Command("/bin/bash", "-c", command.Executable).Output()
	outfile, _ := os.Create(command.GetFileName(reportPath))
	outfile.WriteString(string(ran))
	defer outfile.Close()
	defer wg.Done()
}

func (c *Command) GetFileName(Base string) string {
	return path.Join(Base, MangleCommand(c.Executable))
}
