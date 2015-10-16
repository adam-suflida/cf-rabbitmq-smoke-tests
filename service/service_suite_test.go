package service_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/cloudfoundry-incubator/cf-test-helpers/services"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type rabbitmqTestConfig struct {
	services.Config

	ServiceName string   `json:"service_name"`
	PlanNames   []string `json:"plan_names"`
}

func loadConfig() (testConfig rabbitmqTestConfig) {
	path := os.Getenv("CONFIG_PATH")
	configFile, err := os.Open(path)
	if err != nil {
		panic(err)
	}

	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&testConfig)
	if err != nil {
		panic(err)
	}

	return testConfig
}

var config = loadConfig()

func TestService(t *testing.T) {
	services.NewContext(config.Config, "rabbitmq-smoke-tests").Setup()
	config.TimeoutScale = 3
	RegisterFailHandler(Fail)
	RunSpecs(t, "RabbitMQ Smoke Tests")
}
