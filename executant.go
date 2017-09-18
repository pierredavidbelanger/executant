package main

import (
	consulapi "github.com/hashicorp/consul/api"
	"os"
	"log"
	"time"
	"gopkg.in/yaml.v2"
	"strings"
	"path/filepath"
	"io/ioutil"
	"os/exec"
	"os/signal"
)

type loggerWriter struct {
	l *log.Logger
}

func (l *loggerWriter) Write(p []byte) (n int, err error) {
	l.l.Print(string(p))
	return len(p), nil
}

func main() {

	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime|log.LUTC)
	loggerWriter := loggerWriter{logger}

	key := "executant.yml"
	if keyEnv := os.Getenv("EXECUTANT_KEY"); keyEnv != "" {
		key = keyEnv
	}
	logger.Printf("%20s: %s", "EXECUTANT_KEY", key)

	filters := []string{"executant.enabled=true"}
	if filtersEnv := os.Getenv("EXECUTANT_FILTERS"); filtersEnv != "" {
		filters = strings.Split(filtersEnv, ",")
	}
	logger.Printf("%20s: %v", "EXECUTANT_FILTERS", filters)

	workDir := "/var/lib/executant"
	if workDirEnv := os.Getenv("EXECUTANT_WORK_DIR"); workDirEnv != "" {
		workDir = workDirEnv
	}
	logger.Printf("%20s: %v", "EXECUTANT_WORK_DIR", workDir)

	if err := os.MkdirAll(workDir, os.ModePerm); err != nil {
		logger.Fatalf("Unable to create working folder: %v", err)
		return
	}

	composeFilePath := filepath.Join(workDir, "docker-compose.yml")
	logger.Print("Compose file ", composeFilePath)

	consul, err := consulapi.NewClient(consulapi.DefaultConfig())
	if err != nil {
		logger.Fatalf("Unable to create Consul API client: %v", err)
	}

	go func() {

		qo := consulapi.QueryOptions{}
		qo.WaitTime = 1 * time.Minute

		for {

			kv, _, err := consul.KV().Get(key, &qo)
			if err != nil {
				logger.Printf("Unable to get value for key '%s' (will wait 30s): %v", key, err)
				time.Sleep(30 * time.Second)
				continue
			}

			if kv == nil {
				logger.Printf("Key '%s' does not exists (will wait 15s)", key)
				time.Sleep(15 * time.Second)
				continue
			}

			if qo.WaitIndex == kv.ModifyIndex {
				logger.Printf("Key '%s' did not changed", key)
				continue
			}

			logger.Printf("Key '%s' did changed", key)

			qo.WaitIndex = kv.ModifyIndex

			var yml map[interface{}]interface{}
			err = yaml.Unmarshal(kv.Value, &yml)
			if err != nil {
				logger.Printf("Unable to unmarshal YAML: %v\n%s", err, string(kv.Value))
				continue
			}

			services, ok := yml["services"].(map[interface{}]interface{})
			if !ok {
				logger.Printf("Unable to get services from data: %#v", services)
			}

		withNextService:
			for serviceName, service := range services {
				if service, ok := service.(map[interface{}]interface{}); ok {
					if labels, ok := service["labels"].([]interface{}); ok {
						for _, label := range labels {
							if label, ok := label.(string); ok {
								for _, filter := range filters {
									if label == filter {
										logger.Printf("Keeps service '%s' because of label '%s'", serviceName, label)
										continue withNextService
									}
								}
							}
						}

					}
				}
				logger.Printf("Ignore service '%s'", serviceName)
				delete(services, serviceName)
			}

			data, err := yaml.Marshal(yml)
			if err != nil {
				logger.Printf("Unable to marshal YAML: %v\n%#v", err, yml)
				continue
			}

			logger.Printf("Write to YAML to '%s'\n%s", composeFilePath, string(data))

			err = ioutil.WriteFile(composeFilePath, data, os.ModePerm)
			if err != nil {
				logger.Printf("Unable to write compose content into '%s': %v", composeFilePath, err)
				continue
			}

			var cmd *exec.Cmd
			if len(services) == 0 {
				logger.Printf("Compose down")
				cmd = exec.Command("docker-compose", "down", "--remove-orphans")
				cmd.Dir = workDir
				cmd.Stdout = &loggerWriter
				cmd.Stderr = &loggerWriter
				err = cmd.Run()
				if err != nil {
					logger.Printf("Unable to compose down for '%s': %v", composeFilePath, err)
					continue
				}
				logger.Printf("Composed down!")
			} else {
				logger.Printf("Compose pull")
				cmd = exec.Command("docker-compose", "pull")
				cmd.Dir = workDir
				cmd.Stdout = &loggerWriter
				cmd.Stderr = &loggerWriter
				err = cmd.Run()
				if err != nil {
					logger.Printf("Unable to compose pull for '%s': %v", composeFilePath, err)
					continue
				}
				logger.Printf("Compose up")
				cmd = exec.Command("docker-compose", "up", "--remove-orphans", "-d")
				cmd.Dir = workDir
				cmd.Stdout = &loggerWriter
				cmd.Stderr = &loggerWriter
				err = cmd.Run()
				if err != nil {
					logger.Printf("Unable to compose up for '%s': %v", composeFilePath, err)
					continue
				}
				logger.Printf("Composed up!")
			}
		}

	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan

	cmd := exec.Command("docker-compose", "down", "--remove-orphans")
	cmd.Dir = workDir
	cmd.Stdout = &loggerWriter
	cmd.Stderr = &loggerWriter
	cmd.Run()
}
