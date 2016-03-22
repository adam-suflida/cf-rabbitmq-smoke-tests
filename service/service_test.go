package service_test

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/pborman/uuid"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry-incubator/cf-test-helpers/runner"
	"github.com/cloudfoundry-incubator/cf-test-helpers/services"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

type rabbitmqTestConfig struct {
	services.Config

	ServiceName     string   `json:"service_name"`
	PlanNames       []string `json:"plan_names"`
	RabbitMQSkipSSL bool     `json:"rabbitmq_skip_ssl"`
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
var context services.Context

var _ = Describe("RabbitMQ Service", func() {
	var timeout = time.Second * 60
	var retryInterval = time.Second * 1
	var appPath = "../assets/cf-rabbitmq-example-app"

	var appName string

	randomName := func() string {
		return uuid.NewRandom().String()
	}

	appUri := func(appName string) string {
		return "https://" + appName + "." + config.AppsDomain
	}

	assertAppIsRunning := func(appName string) {
		pingUri := appUri(appName) + "/ping"
		fmt.Println("Checking that the app is responding at url: ", pingUri)
		Eventually(runner.Curl(pingUri, "-k"), config.ScaledTimeout(timeout), retryInterval).Should(Say("OK"))
		fmt.Println("\n")
	}

	BeforeSuite(func() {
		config.TimeoutScale = 3
		context = services.NewContext(config.Config, "rabbitmq-smoke-test")
		context.Setup()
	})

	AfterSuite(func() {
		context.Teardown()
	})

	AssertLifeCycleBehavior := func(planName string) {
		var serviceInstanceName string
		appPushed := false
		serviceCreated := false
		serviceBound := false
		appIsRunning := false

		It("Should be able to push the application", func() {
			appName = randomName()
			Eventually(cf.Cf("push", appName, "-m", "256M", "-p", appPath, "-s", "cflinuxfs2", "-no-start"), config.ScaledTimeout(timeout)).Should(Exit(0))
			appPushed = true
		})

		It("Can create the service instance", func() {
			Ω(appPushed).Should(BeTrue())
			serviceInstanceName = randomName()
			Eventually(cf.Cf("create-service", config.ServiceName, planName, serviceInstanceName), config.ScaledTimeout(timeout)).Should(Exit(0))
			serviceCreated = true
		})

		It("Can bind the service and start the application", func() {
			Ω(appPushed && serviceCreated).Should(BeTrue())
			Eventually(cf.Cf("bind-service", appName, serviceInstanceName), config.ScaledTimeout(timeout)).Should(Exit(0))
			serviceBound = true
			var RMQSkipSSLValue string = "0"
			if config.RabbitMQSkipSSL {
				RMQSkipSSLValue = "1"
			}
			Eventually(cf.Cf("set-env", appName, "RABBITMQ_SKIP_SSL", RMQSkipSSLValue), config.ScaledTimeout(5*time.Minute)).Should(Exit(0))
			Eventually(cf.Cf("start", appName), config.ScaledTimeout(5*time.Minute)).Should(Exit(0))
			assertAppIsRunning(appName)
			appIsRunning = true
		})

		It("can write to and read from a service instance using the "+planName+" plan", func() {
			Ω(appPushed && serviceCreated && serviceBound && appIsRunning).Should(BeTrue())
			/*
			   create a queue     (should 201)
			   list the queues    (should 200)
			   subscribe          (should 204)
			   publish            (should 201)
			   subscribe          (should 200)
			*/
			uri := appUri(appName) + "/queues"
			fmt.Println("Creating a new queue: ", uri)
			Eventually(runner.Curl(uri, "-k", "-X", "POST", "-d", "name=test-q"), config.ScaledTimeout(timeout), retryInterval).Should(Say("SUCCESS"))
			fmt.Println("\n")

			fmt.Println("Listing the queues: ", uri)
			Eventually(runner.Curl(uri, "-k"), config.ScaledTimeout(timeout), retryInterval).Should(Say("test-q\n"))
			fmt.Println("\n")

			uri = appUri(appName) + "/queue/test-q"
			fmt.Println("Reading from the (empty) queue: ", uri)
			Eventually(runner.Curl(uri, "-k"), config.ScaledTimeout(timeout), retryInterval).Should(Say(""))
			fmt.Println("\n")

			fmt.Println("Publishing to the queue: ", uri)
			Eventually(runner.Curl("-d", "data=test-message", "-X", "PUT", uri, "-k"), config.ScaledTimeout(timeout), retryInterval).Should(Say("SUCCESS"))
			fmt.Println("\n")

			fmt.Println("Reading from the (non-empty) queue: ", uri)
			Eventually(runner.Curl(uri, "-k"), config.ScaledTimeout(timeout), retryInterval).Should(Say("test-message"))
			fmt.Println("\n")
		})

		It("Should be able to clean up after itself", func() {
			if serviceBound {
				Eventually(cf.Cf("unbind-service", appName, serviceInstanceName), config.ScaledTimeout(timeout)).Should(Exit(0))
			}
			if serviceCreated {
				Eventually(cf.Cf("delete-service", "-f", serviceInstanceName), config.ScaledTimeout(timeout)).Should(Exit(0))
			}
			if appPushed {
				Eventually(cf.Cf("delete", appName, "-f"), config.ScaledTimeout(timeout)).Should(Exit(0))
			}
		})
	}

	Context("for each plan", func() {
		for _, planName := range config.PlanNames {
			AssertLifeCycleBehavior(planName)
		}
	})
})
