package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	sshtools "github.com/scylladb/go-sshtools"
	"golang.org/x/crypto/ssh"
)

const (
	FailConfig    = "config_fail"
	FailConnect   = "connect_fail"
	FailStart     = "start_fail"
	FailWait      = "wait_fail"
	Success       = "success"
	Fail          = "fail"
	VMCmd         = "VM_CMD"
	ConnectorType = "CONNECTOR_TYPE"
)

var metrics = map[string]string{
	"gitlab": "connector_gitlab",
	"vm":     "connector_vm",
}

type Args struct {
	host                    string
	user                    string
	port                    int
	serverKeepAliveInterval time.Duration
	serverKeepAliveMaxCount int
	sshFilePath             string
}

func readArgs() *Args {
	userPtr := flag.String("user", "", "SSH user")
	hostPtr := flag.String("host", "", "Host ip to connect to")
	portPtr := flag.Int("port", 22, "Port for ssh server")
	serverAlivePtr := flag.Int("server-alive-interval", 5, "Interval at which to send keep alive messages")
	serverAliveCountPtr := flag.Int("server-alive-count", 560, "Maximum keep alive messages to send before disconnecting upon no reply")
	sshFilePathPtr := flag.String("ssh-file", "", "Path to private ssh key")

	flag.Parse()

	return &Args{
		host:                    *hostPtr,
		user:                    *userPtr,
		port:                    *portPtr,
		serverKeepAliveInterval: time.Duration(*serverAlivePtr) * time.Second,
		serverKeepAliveMaxCount: *serverAliveCountPtr,
		sshFilePath:             *sshFilePathPtr,
	}
}

type ConnectorInfo struct {
	// For gitlab runnner this will be the job id
	// For metal instance this will be empty
	connectorHost string
	connectorType string
}

func getConnectorInfo() (ConnectorInfo, error) {
	connectorType, ok := os.LookupEnv(ConnectorType)
	if !ok {
		return ConnectorInfo{}, fmt.Errorf("no connector type provided")
	}

	ok = false
	for ct, _ := range metrics {
		if connectorType == ct {
			ok = true
		}
	}
	if !ok {
		return ConnectorInfo{}, fmt.Errorf("unknown connector type: %s", connectorType)
	}

	return ConnectorInfo{
		connectorHost: os.Getenv("$CI_JOB_ID"),
		connectorType: connectorType,
	}, nil
}

func sshCommunicator(args *Args, sshKey []byte) (*sshtools.Communicator, error) {
	config := sshtools.Config{
		Port:                args.port,
		ServerAliveInterval: args.serverKeepAliveInterval,
		ServerAliveCountMax: args.serverKeepAliveMaxCount,
	}
	config, err := config.WithIdentityFileAuth(args.user, sshKey)
	if err != nil {
		return nil, fmt.Errorf("unable to build sshtools config: %w", err)
	}
	config.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	return sshtools.NewCommunicator(args.host, config, sshtools.ContextDialer(&net.Dialer{}), log.New(os.Stdout, "", log.LstdFlags)), nil
}

func main() {
	var failType string
	var cmd sshtools.Cmd

	args := readArgs()
	status := Fail

	cinfo, err := getConnectorInfo()
	if err != nil {
		log.Fatal(err)
	}
	key, err := os.ReadFile(args.sshFilePath)
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	communicator, err := sshCommunicator(args, key)
	ctx := context.Background()
	if err != nil {
		failType = FailConfig
		goto fail
	}

	if err := communicator.Connect(ctx); err != nil {
		failType = FailConnect
		goto fail
	}

	cmd.Command = os.Getenv(VMCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := communicator.Start(ctx, &cmd); err != nil {
		failType = FailConnect
		goto fail
	}

	if err := cmd.Wait(); err != nil {
		failType = FailWait
		goto fail
	}

	status = Success
fail:
	if err := SubmitExecutionMetric(cinfo, failType, status); err != nil {
		log.Fatal(err)
	}

	log.Fatal(err)
}

func buildMetric(cinfo ConnectorInfo, failType, status string) datadogV2.MetricPayload {
	tags := []string{
		fmt.Sprintf("error:%s", failType),
		fmt.Sprintf("status:%s", status),
	}
	return datadogV2.MetricPayload{
		Series: []datadogV2.MetricSeries{
			{
				Metric: metrics[cinfo.connectorType],
				Type:   datadogV2.METRICINTAKETYPE_COUNT.Ptr(),
				Points: []datadogV2.MetricPoint{
					{
						Timestamp: datadog.PtrInt64(time.Now().Unix()),
						Value:     datadog.PtrFloat64(1),
					},
				},
				Resources: []datadogV2.MetricResource{
					{
						Name: datadog.PtrString(cinfo.connectorHost),
						Type: datadog.PtrString("host"),
					},
				},
				Tags: tags,
			},
		},
	}
}

func SubmitExecutionMetric(cinfo ConnectorInfo, failType, status string) error {
	metricBody := buildMetric(cinfo, failType, status)

	ctx := datadog.NewDefaultContext(context.Background())
	configuration := datadog.NewConfiguration()
	apiClient := datadog.NewAPIClient(configuration)
	api := datadogV2.NewMetricsApi(apiClient)
	resp, r, err := api.SubmitMetrics(ctx, metricBody, *datadogV2.NewSubmitMetricsOptionalParameters())

	if err != nil {
		fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
		return fmt.Errorf("error when calling `MetricsApi.SubmitMetrics`: %v\n", err)
	}

	responseContent, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Fprintf(os.Stdout, "Response from `MetricsApi.SubmitMetrics`:\n%s\n", responseContent)

	return nil
}
