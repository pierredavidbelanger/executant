package main

import (
	consulapi "github.com/hashicorp/consul/api"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"
)

func main() {

	if logLevelEnv := os.Getenv("LOG_LEVEL"); logLevelEnv != "" {
		logLevel, err := log.ParseLevel(logLevelEnv)
		if err != nil {
			log.Fatal("Invalid log level: ", err)
		}
		log.SetLevel(logLevel)
	}

	keys := os.Args[1:]
	if len(keys) == 0 {
		log.Fatal("At least one key is required")
	}

	workDir := "/var/lib/executant"
	if workDirEnv := os.Getenv("EXECUTANT_WORK_DIR"); workDirEnv != "" {
		workDir = workDirEnv
	}
	log.Debug("Work dir ", workDir)

	config := consulapi.DefaultConfig()
	client, err := consulapi.NewClient(config)
	if err != nil {
		log.Fatal("Can not create a Consul API client: ", err)
	}

	for {
		leader, err := client.Status().Leader()
		if err != nil {
			log.Warn("Can not contact a Consul loader: ", err)
			time.Sleep(time.Second)
		} else {
			log.Debug("Consul loader: ", leader)
			break
		}
	}

	for i := 0; i < len(keys); i++ {
		go work(client, workDir, keys[i])
	}

	// trap ctrl-c
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	chanSignal := <-signalChan
	log.Debug("Got signal: ", chanSignal)

	for i := 0; i < len(keys); i++ {
		update(filepath.Join(workDir, keys[i]), "")
	}
}

func work(client *consulapi.Client, workDir string, key string) {
	projectDir := filepath.Join(workDir, key)
	log.Debug("Key ", key, " project dir ", projectDir)
	if err := os.MkdirAll(projectDir, os.ModePerm); err != nil {
		log.Error("Error while mkdir ", projectDir)
		return
	}
	queryOptions := consulapi.QueryOptions{
		WaitIndex: 0,
		WaitTime:  5 * time.Minute,
	}
	for {
		keyPair, _, err := client.KV().Get(key, &queryOptions)
		if err != nil {
			log.Warn("Error while getting key ", key, " from Consul: ", err)
			time.Sleep(5 * time.Second)
		} else if keyPair == nil {
			log.Debug("Empty response for key ", key)
			update(projectDir, "")
			time.Sleep(5 * time.Second)
		} else if keyPair.ModifyIndex == queryOptions.WaitIndex {
			log.Debug("No change for key ", key)
		} else if keyPair.Value == nil {
			log.Debug("Empty value for key ", key)
			update(projectDir, "")
			queryOptions.WaitIndex = keyPair.ModifyIndex
		} else if keyPair != nil && keyPair.Value != nil {
			yml := string(keyPair.Value)
			log.Debug("Value for key ", key, " is ", yml)
			update(projectDir, yml)
			queryOptions.WaitIndex = keyPair.ModifyIndex
		}
	}
}

func update(projectDir string, yml string) {
	projectFile := filepath.Join(projectDir, "docker-compose.yml")
	if yml == "" {
		if fileInfo, err := os.Stat(projectFile); fileInfo != nil && err == nil {
			cmd := exec.Command("docker-compose", "down", "--remove-orphans")
			log.Info("Execute docker-compose down in ", projectDir)
			cmd.Dir = projectDir
			if log.GetLevel() >= log.InfoLevel {
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
			}
			err = cmd.Run()
			if err != nil {
				log.Error("Unable to docker-compose down ", err)
				return
			}
			err = os.Remove(projectFile)
			if err != nil {
				log.Error("Unable to remove ", projectFile, err)
				return
			}
		}
		return
	}
	if err := ioutil.WriteFile(projectFile, []byte(yml), os.ModePerm); err != nil {
		log.Error("Unable to write to file ", projectFile)
		return
	}
	cmd := exec.Command("docker-compose", "up", "-d", "--remove-orphans")
	log.Info("Execute docker-compose up in ", projectDir)
	cmd.Dir = projectDir
	if log.GetLevel() >= log.InfoLevel {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	err := cmd.Run()
	if err != nil {
		log.Error("Unable to docker-compose up", err)
	}
}
